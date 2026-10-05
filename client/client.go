package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type (
	ClientConfig struct {
		Server          string `json:"server"`
		TimeoutSeconds  int    `json:"timeout_seconds"`
		SaveDir         string `json:"save_dir"`
		AutoDownload    bool   `json:"autodownload"`
		DaemonRetrySecs int    `json:"daemon_retry_secs"`
	}
	UploadType string

	APIResponse struct {
		Success bool        `json:"success"`
		Data    interface{} `json:"data,omitempty"`
		Error   string      `json:"error,omitempty"`
	}
	Client struct {
		Token  string
		Name   string
		Client http.Client
		Config ClientConfig
	}
	Dir struct {
		Files   []string `json:"files"`
		DirName string   `json:"name"`
		Dirs    []Dir    `json:"dirs,omitempty"`
	}
	Device struct {
		ID   int64
		Name string
	}
	DownloadItem struct {
		UUID   string
		Type   string
		Header string
	}
)

const (
	UploadLink        UploadType = "link"
	UploadFile        UploadType = "file"
	UploadDirChild    UploadType = "dir-child"
	UploadDirManifest UploadType = "dir-struct"
)

func (a APIResponse) String() string {
	r, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return "error json conversion"
	}
	return string(r)

}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (c *Client) Send(method, path string, data any) (*APIResponse, error) {
	var reader io.Reader

	if method == "POST" && data == nil { //post eq always has a body
		data = map[string]interface{}{}
	}

	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("(Send method) error when formatting data: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.Config.Server+path, reader)
	if err != nil {
		return nil, fmt.Errorf("(Send method) error when creating request: %w", err)
	}

	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("(Send method) request error: %w", err)
	}
	defer resp.Body.Close()

	var out APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("(Send method) response decode error (HTTP %d): %w", resp.StatusCode, err)
	}
	return &out, nil
}

func (c *Client) SendRaw(method, path, contentType string, body io.Reader, dirFlag UploadType) (*APIResponse, error) {
	req, err := http.NewRequest(method, c.Config.Server+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("InputType", string(dirFlag))

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DownloadItem(uuid, path string) (string, error) {

	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.Config.Server+"download?uuid="+uuid, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("server %d: %s", resp.StatusCode, body)
	}

	cd := resp.Header.Get("Content-Disposition")
	if cd == "" {
		return "", fmt.Errorf("server sent no Content-Disposition")
	}
	_, params, err := mime.ParseMediaType(cd)
	if err != nil {
		return "", fmt.Errorf("bad Content-Disposition %q: %w", cd, err)
	}
	name := params["filename"]
	if name == "" {
		return "", fmt.Errorf("no filename in %q", cd)
	}
	name = filepath.Base(name)

	base := strings.TrimSuffix(name, filepath.Ext(name))
	ext := filepath.Ext(name)

	dest := filepath.Join(path, name)
	for i := 1; ; i++ {
		_, err := os.Stat(dest)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("stat %s: %w", dest, err)
		}
		dest = filepath.Join(path,
			fmt.Sprintf("%s (%d)%s", base, i, ext))
	}

	out, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer out.Close()

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return "", fmt.Errorf("short read: %d/%d", n, resp.ContentLength)
	}
	return dest, out.Close()
}

func (c *Client) DownloadItems(fileItems []DownloadItem) {
	for _, item := range fileItems {
		if item.Type == "link" {
			fmt.Printf("\n  TEXT:\n    %s\n\n", item.Header)
			resp, err := c.Send("DELETE", "delete?uuid="+item.UUID, nil)
			if err != nil {
				fmt.Printf("GET deleting text error (confirmation request): %v\n", err)
			}
			if !resp.Success {
				fmt.Printf("GET deleting text error (server): %v\n", resp.Error)
			}
			continue
		}
		savePath, err := c.DownloadItem(item.UUID, c.Config.SaveDir)
		if err != nil {
			fmt.Printf("GET [%s] error: %v\n", item.UUID, err)
			continue
		}
		if resp, err := c.Send("DELETE", "delete?uuid="+item.UUID, nil); err != nil {
			fmt.Printf("GET [%s] succeeded: confirmation error: %v\n", item.UUID, err)
			continue
		} else if !resp.Success {
			fmt.Printf("GET [%s] succeeded: confirmation error (server): %v\n", item.UUID, resp.Error)
			continue
		}
		fmt.Printf("Downloaded Item [%s]\n", item.UUID)

		if item.Type == string(UploadDirManifest) {
			file, err := os.Open(savePath)
			if err != nil {
				fmt.Printf("GET opening dir manifest error: %v\n", err)
				return
			}

			var dir Dir
			if err := json.NewDecoder(file).Decode(&dir); err != nil {
				fmt.Printf("GET parsing dir manifest error: %v\n", err)
				return
			}
			file.Close()

			var parseDirStruct func(Dir, string) error

			parseDirStruct = func(curr Dir, directory string) error {
				currSave := filepath.Join(directory, curr.DirName)
				if err := os.MkdirAll(currSave, 0o755); err != nil {
					return err
				}
				for _, uuid := range curr.Files {
					_, err := c.DownloadItem(uuid, currSave)
					if err != nil {
						return err
					}
				}
				for _, newDir := range curr.Dirs {
					err := parseDirStruct(newDir, currSave)
					if err != nil {
						return err
					}
				}
				return nil
			}

			err = parseDirStruct(dir, c.Config.SaveDir)
			if err != nil {
				fmt.Printf("GET saving Dir struct error: %v\n", err)
				return
			}

			if err := os.Remove(savePath); err != nil {
				fmt.Printf("GET removing dir manifest error: %v\n", err)
				return
			}
		}
	}
}

func (c *Client) Init() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("failed to find home dir:", err)
		os.Exit(1)
	}
	dir := filepath.Join(home, ".filetransfer")

	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Println("failed to create app dir:", err)
		os.Exit(1)
	}

	cfgPath := filepath.Join(dir, "client_config.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		defaultCfg := ClientConfig{
			Server:          "http://localhost:7842/",
			TimeoutSeconds:  5,
			SaveDir:         "~/Downloads/filetransfer",
			AutoDownload:    false,
			DaemonRetrySecs: 15,
		}
		err := saveConfig(defaultCfg)
		if err != nil {
			fmt.Println("error writing default cfg")
			os.Exit(1)
		}
	}
	if err := c.loadConfig(); err != nil {
		fmt.Printf("Fatal: %v", err)
		os.Exit(1)
	}

	tokPath := filepath.Join(dir, "token")
	if _, err := os.Stat(tokPath); os.IsNotExist(err) {
		if err := os.WriteFile(tokPath, []byte{}, 0o600); err != nil {
			fmt.Println("failed to create token file:", err)
			os.Exit(1)
		}
	}

	if tb, err := os.ReadFile(tokPath); err == nil {
		c.Token = strings.TrimSpace(string(tb))
	}

	timeout := time.Duration(c.Config.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	c.Client = http.Client{Timeout: timeout}

	if c.Token != "" {
		_, name, err := getWhoAmI(c)

		if err != nil {
			fmt.Printf("error getting who am i: %v\n", err)
			return
		}

		c.Name = name
	}
}

func (c *Client) loadConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("failed to find home dir:", err)
		os.Exit(1)
	}
	cfgPath := filepath.Join(home, ".filetransfer", "client_config.json")

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &c.Config); err != nil {
		return err
	}

	if strings.HasPrefix(c.Config.SaveDir, "~/") {
		c.Config.SaveDir = filepath.Join(home, c.Config.SaveDir[2:])
	}

	if err := os.MkdirAll(c.Config.SaveDir, 0o755); err != nil {
		return err
	}

	if c.Config.Server == "" {
		return err
	}
	return nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func saveConfig(cfg ClientConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".filetransfer", "client_config.json")
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func saveToken(token string) error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".filetransfer", "token")
	return os.WriteFile(path, []byte(token), 0o600)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func getFiles(c *Client) ([]map[string]any, error) {
	r, err := c.Send("GET", "files", nil)
	if err != nil {
		return nil, err
	}
	// fmt.Println(r)
	listAny, ok := r.Data.([]any)
	if !ok {
		return nil, errors.New("unexpected file type")
	}

	var list []map[string]any
	for _, item := range listAny {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		list = append(list, m)
	}
	return list, nil
}

func getDevices(c *Client) ([]Device, error) {
	devices := []Device{}

	r, err := c.Send("GET", "devices", nil)
	if err != nil {
		return devices, err
	}
	list, ok := r.Data.([]any)
	if !ok {
		return devices, fmt.Errorf("unexpected response shape")
	}

	for _, entry := range list {
		dev, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		id, ok := dev["id"].(float64)
		if !ok {
			return devices, fmt.Errorf("invalid user id")
		}

		name, ok := dev["name"].(string)
		if !ok {
			return devices, fmt.Errorf("invalid user name")
		}
		devices = append(devices, Device{ID: int64(id), Name: name})
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].ID < devices[j].ID
	})
	return devices, nil
}

func getWhoAmI(c *Client) (int64, string, error) {
	r, err := c.Send("GET", "whoami", nil)
	if err != nil {
		return 0, "", fmt.Errorf("Error when sending request: %v", err)
	}

	dev, ok := r.Data.(map[string]any)
	if !ok {
		return 0, "", fmt.Errorf("Error when parsing response: %v", err)
	}
	id, ok := dev["id"].(float64)
	if !ok {
		return 0, "", fmt.Errorf("Error when parsing response(id): %v", err)
	}

	name, ok := dev["name"].(string)
	if !ok {
		return 0, "", fmt.Errorf("Error when parsing response(name): %v", err)
	}

	return int64(id), name, nil
}

func buildUploadBody(receivers []string, filePath, link string) (*bytes.Buffer, string, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	if filePath != "" {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, "", err
		}
		defer f.Close()

		part, err := w.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			return nil, "", err
		}
		if _, err := io.Copy(part, f); err != nil {
			return nil, "", err
		}
	}

	if link != "" {
		w.WriteField("text", link)
	}

	recData, err := json.Marshal(receivers)
	if err != nil {
		fmt.Printf("error encoding receivers into json: %v\n", err)
	}

	w.WriteField("receivers", string(recData))
	w.Close()

	return &body, w.FormDataContentType(), nil
}

func buildDirManifestBody(receivers []string, root Dir) (*bytes.Buffer, string, error) {
	var jsonBody bytes.Buffer

	err := json.NewEncoder(&jsonBody).Encode(root)
	if err != nil {
		return nil, "", err
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	part, err := w.CreateFormFile("file", fmt.Sprintf("%s.json", root.DirName))
	if err != nil {
		return nil, "", err
	}

	if _, err := io.Copy(part, &jsonBody); err != nil {
		return nil, "", err
	}

	recData, err := json.Marshal(receivers)
	if err != nil {
		fmt.Printf("error encoding receivers into json: %v\n", err)
	}

	w.WriteField("receivers", string(recData))

	if err := w.Close(); err != nil {
		return nil, "", err
	}

	return &body, w.FormDataContentType(), nil
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {
	var c Client
	c.Init()

	if len(os.Args) < 2 {
		cmdHelp()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch strings.ToLower(cmd) {
	case "auth":
		cmdAuth(&c, args)
	case "devices":
		cmdDevices(&c, args)
	case "inbox":
		cmdFiles(&c, args)
	case "file":
		cmdSend(&c, args)
	case "text":
		cmdLink(&c, args)
	case "whoami":
		cmdWho(&c, args)
	case "get":
		cmdDownload(&c, args)
	case "cfg":
		cmdEditCFG(&c, args)
	case "del":
		cmdDelete(&c, args)
	case "deluser":
		cmdDeleteUser(&c, args)
	case "dir":
		cmdDir(&c, args)
	case "daemon":
		runDaemon(&c, args)
	default:
		fmt.Printf("Unknown command: %s\n\n", cmd)
		cmdHelp()
		os.Exit(1)
	}
}

func cmdHelp() {
	fmt.Println(`  ./filetransfer <command> [args]
  COMMANDS:
	  BASIC:
      auth <name> <password>     authenticate and get a token +
      whoami                     currently logged in as which user +
      users                      list registered users +
      cfg                        current config and config path +
      help                       this menu
      daemon                     launches autodownload (later)
			deluser <id>               deletes user with that id +

    INCOMING:
      inbox                      list pending Files/Text/Dirs +
      get all|<id> [<id>]        download Files/Text/Dirs +
      del all|<id> [<id>]        delete Files/Text/Dirs +

    OUTGOING:
      file <path> all|<user_id> [<user_id>]    send a file
      text "text" all|<user_id> [<user_id>]    send a link
      dir  <path> all|<user_id> [<user_id>]    send a folder
    
    `)
	fmt.Println()
}

func cmdAuth(c *Client, args []string) {
	if len(args) != 2 {
		fmt.Println("USAGE: filetransfer auth <name> <password>")
		return
	}

	r, err := c.Send("POST", "auth", map[string]string{
		"Name":     args[0],
		"Password": args[1],
	})
	if err != nil {
		fmt.Printf("Auth request error: %v\n", err)
		return
	}

	token, ok := r.Data.(string)
	if !ok || token == "" {
		fmt.Println("Auth request error: server returned unexpected value")
		return
	}

	if err := saveToken(token); err != nil {
		fmt.Printf("Auth saving token error: %v\n", err)
		return
	}

	fmt.Printf("OK - Auth as user: %s\n", args[0])
}

func cmdDevices(c *Client, _ []string) {
	devices, err := getDevices(c)
	if err != nil {
		fmt.Printf("Users fetching users error: %v\n", err)
		return
	}

	fmt.Println("\n  DEVICES:")

	for _, d := range devices {
		if d.Name != c.Name {
			fmt.Printf("    [%v] %v\n", d.ID, d.Name)
		} else {
			fmt.Printf("    [%v] %v <-- current user\n", d.ID, d.Name)
		}
	}
	fmt.Println()
}

func cmdFiles(c *Client, _ []string) {

	list, err := getFiles(c)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	fmt.Println("\n  INBOX:")

	if len(list) == 0 {
		fmt.Println("    Empty")
		return
	}

	truncate := func(s string, n int) string {
		r := []rune(s)
		if len(r) <= n {
			return s
		}
		return string(r[:n-1]) + "…"
	}

	for _, row := range list {
		id := fmt.Sprintf("%v", row["id"])
		sender := fmt.Sprintf("%v", row["sender_id"])
		date := fmt.Sprintf("%v", row["uploaded_at"])

		if t, _ := row["type"].(string); t == "link" {
			text := fmt.Sprintf("%v", row["url"])
			fmt.Printf("    [%s] Te  %-30s  from [%s]  %s\n", id, truncate(text, 60), sender, date)
		} else if t == "file" {
			name := fmt.Sprintf("%v", row["filename"])
			fmt.Printf("    [%s] Fi  %-30s  from [%s]  %s\n", id, truncate(name, 60), sender, date)
		} else {
			name := strings.TrimSuffix(fmt.Sprintf("%v", row["filename"]), ".json") + "/"
			fmt.Printf("    [%s] Di  %-30s  from [%s]  %s\n", id, truncate(name, 60), sender, date)
		}
	}
	fmt.Println()
}

func cmdWho(c *Client, _ []string) {

	id, name, err := getWhoAmI(c)

	if err != nil {
		fmt.Printf("Whoami error: %v\n", err)
		return
	}

	fmt.Printf("\n  Logged in as [%v] - %v\n\n", id, name)

}

func cmdDir(c *Client, args []string) {

	if len(args) < 2 {
		fmt.Println("filetransfer dir <path> <receiver> [<receiver]")
		os.Exit(1)
	}

	path := args[0]
	receivers := args[1:]
	if receivers[0] == "all" {
		receivers = []string{}
		devices, err := getDevices(c)
		if err != nil {
			fmt.Printf("error when fetching devices: %v\n", err)
		}
		for _, r := range devices {
			receivers = append(receivers, r.Name)
		}
	}

	f, err := os.Stat(path)
	if os.IsNotExist(err) {
		fmt.Println("no such path found")
		os.Exit(1)
	} else if err != nil {
		fmt.Printf("unkown error when os.Stat: %v\n", err)
		os.Exit(1)
	}

	if !f.IsDir() {
		fmt.Println("path is a file: use file command")
		os.Exit(1)
	}

	type Rec struct {
		Name    string `json:"name"`
		Error   string `json:"error"`
		ID      int64  `json:"id"`
		Errored bool   `json:"errored"`
	}

	type UploadResult struct {
		UUID      string `json:"uuid"`
		Receivers []Rec  `json:"receivers"`
	}

	receiverMap := map[string]bool{} // false 0,..,n-1/n, true n/n success case
	for _, r := range receivers {
		receiverMap[r] = true
	}
	var traverseDir func(string) Dir
	traverseDir = func(path string) Dir {

		var r Dir
		r.DirName = filepath.Base(path)
		files, err := os.ReadDir(path)
		if err != nil {
			fmt.Printf("error reading dir: %s\n%v\n", path, err)
			return r
		}
		r.Dirs = []Dir{}
		r.Files = []string{}

		for _, item := range files {
			if item.IsDir() {
				r.Dirs = append(r.Dirs, traverseDir(filepath.Join(path, item.Name())))
			} else {
				cpath := filepath.Join(path, item.Name())
				body, contentType, err := buildUploadBody(receivers, strings.TrimSpace(cpath), "")
				if err != nil {
					fmt.Printf("error building body: %s\n%v\n", cpath, err)
					os.Exit(1)
				}

				resp, err := c.SendRaw("POST", "upload", contentType, body, UploadDirChild)
				if err != nil {
					fmt.Printf("error uploading: %s\n%v\n", cpath, err)
					os.Exit(1)
				}
				if !resp.Success {
					fmt.Printf("error: %s\n%v\n", cpath, resp.Error)
					os.Exit(1)
				}

				var result UploadResult

				data, err := json.Marshal(resp.Data)
				if err != nil {
					fmt.Printf("error encoding response data: %v\n", err)
					os.Exit(1)
				}

				if err := json.Unmarshal(data, &result); err != nil {
					fmt.Printf("error unmarshalling response data: %v\n", err)
					os.Exit(1)
				}

				r.Files = append(r.Files, result.UUID)

				for _, rec := range result.Receivers {
					if rec.Errored {
						receiverMap[rec.Name] = false
					}
				}
			}
		}

		return r
	}

	root := traverseDir(path)

	validReceivers := []string{}
	for key, val := range receiverMap {
		if val {
			validReceivers = append(validReceivers, key)
		}
	}

	if len(validReceivers) == 0 {
		fmt.Println("no valid receivers when sending dir")
		return
	}

	body, contentType, err := buildDirManifestBody(validReceivers, root)
	if err != nil {
		fmt.Printf("error building manifest: %v\n", err)
		return
	}

	for key, val := range receiverMap {
		if !val {
			fmt.Printf("file sending error for %v\n", key)
		} else {
			fmt.Printf("%v files sent ok, now manifest\n", key)
		}
	}

	resp, err := c.SendRaw(
		"POST",
		"upload",
		contentType,
		body,
		UploadDirManifest,
	)

	if err != nil {
		fmt.Printf("error when sending %v\n", err)
		os.Exit(1)
	}
	if !resp.Success {
		fmt.Printf("error server: %v\n", resp.Error)
		os.Exit(1)
	}

	var result UploadResult

	data, err := json.Marshal(resp.Data)
	if err != nil {
		fmt.Printf("error encoding response data: %v\n", err)
		os.Exit(1)
	}

	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Printf("error decoding upload response: %v\n", err)
		os.Exit(1)
	}

	for _, r := range result.Receivers {
		if r.Errored {
			fmt.Printf("manifest sending error for %v: %v\n", r.Name, r.Error)
		} else {
			fmt.Printf("%v manifest sent ok\n", r.Name)
		}
	}

}

func cmdSend(c *Client, args []string) {
	if len(args) < 2 {
		fmt.Println("USAGE: filetransfer send <path> all|<receiver> [<receiver]")
		os.Exit(1)
	}

	path := args[0]
	receivers := args[1:]
	if receivers[0] == "all" {
		receivers = []string{}
		devices, err := getDevices(c)
		if err != nil {
			fmt.Printf("SENDFILE fetching devices error: %v\n", err)
		}
		for _, r := range devices {
			receivers = append(receivers, r.Name)
		}
	}

	body, contentType, err := buildUploadBody(receivers, strings.TrimSpace(path), "")
	if err != nil {
		fmt.Println("SENDFILE building body error:", err)
		os.Exit(1)
	}

	resp, err := c.SendRaw("POST", "upload", contentType, body, UploadFile)
	if err != nil {
		fmt.Println("SENDFILE upload error:", err)
		os.Exit(1)
	}
	if resp.Success {
		fmt.Println("OK")
	} else {
		fmt.Printf("SENDFILE upload error (server): %v\n", resp.Error)
	}
}

func cmdLink(c *Client, args []string) {

	if len(args) < 2 {
		fmt.Println(`filetransfer link "<url>" <receiver> [<receiver]`)
		os.Exit(1)
	}

	url := args[0]
	receivers := args[1:]
	if receivers[0] == "all" {
		receivers = []string{}
		devices, err := getDevices(c)
		if err != nil {
			fmt.Printf("error when fetching devices: %v\n", err)
		}
		for _, r := range devices {
			receivers = append(receivers, r.Name)
		}
	}

	body, contentType, err := buildUploadBody(receivers, "", url)
	if err != nil {
		fmt.Println("error building body:", err)
		os.Exit(1)
	}

	resp, err := c.SendRaw("POST", "upload", contentType, body, UploadLink)
	if err != nil {
		fmt.Println("error uploading:", err)
		os.Exit(1)
	}
	if resp.Success {
		fmt.Println("OK")
	} else {
		fmt.Printf("Error when sending text: %v\n", resp.Error)
	}
}

func cmdDownload(c *Client, args []string) {
	if len(args) == 0 {
		fmt.Println("USAGE: get all|<fileID> [<fileID>]")
		return
	}

	files, err := getFiles(c)
	if err != nil {
		fmt.Println("GET error when fetching files:", err)
		return
	}

	fileItems := []DownloadItem{}
	if args[0] == "all" {
		for _, f := range files {
			var d DownloadItem
			if uuid, ok := f["uuid"].(string); ok {
				d.UUID = uuid
			} else {
				continue
			}
			if t, ok := f["type"].(string); ok {
				d.Type = t
			} else {
				continue
			}

			if url, ok := f["url"].(string); ok {
				d.Header = url
			} else if fName, ok := f["filename"].(string); ok {
				d.Header = fName
			}
			fileItems = append(fileItems, d)
		}
	} else {
		for _, f := range files {
			id := fmt.Sprint(f["id"])
			if slices.Contains(args, id) {
				var d DownloadItem
				if uuid, ok := f["uuid"].(string); ok {
					d.UUID = uuid
				} else {
					continue
				}
				if t, ok := f["type"].(string); ok {
					d.Type = t
				} else {
					continue
				}

				if url, ok := f["url"].(string); ok {
					d.Header = url
				} else if fName, ok := f["filename"].(string); ok {
					d.Header = fName
				}
				fileItems = append(fileItems, d)
			}
		}
	}

	c.DownloadItems(fileItems)
	fmt.Println("OK")
}

func cmdDelete(c *Client, args []string) {
	if len(args) == 0 {
		fmt.Println("UASGE: del all|<fileID> [<fileID> ...]")
		return
	}

	var fileIDs []string
	if args[0] == "all" {
		files, err := getFiles(c)
		if err != nil {
			fmt.Println("DEL fetching files error:", err)
			return
		}
		for _, f := range files {
			if id, ok := f["id"].(string); ok {
				fileIDs = append(fileIDs, id)
			}
		}
	} else {
		fileIDs = args
	}
	for _, id := range fileIDs {
		if resp, err := c.Send("DELETE", "delete?id="+id, nil); err != nil {
			fmt.Printf("DEL item [%s] error: %v\n", id, err)
			continue
		} else if !resp.Success {
			fmt.Printf("DEL item [%s] error (server): %v\n", id, resp.Error)
			continue
		}

		fmt.Printf("Deleted Item [%s]\n", id)
	}

}

func cmdEditCFG(c *Client, _ []string) {
	b, err := json.MarshalIndent(c.Config, "", "  ")
	if err != nil {
		fmt.Printf("Config json -> []byte error: %v\n", err)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Config homedir not found error: %v\n", err)
		return
	}
	path := filepath.Join(home, ".filetransfer", "client_config.json")
	fmt.Println("\n  CONFIG PATH: ", path)
	fmt.Println(string(b))
	fmt.Println()
}

func cmdDeleteUser(c *Client, args []string) {
	if len(args) == 0 {
		fmt.Println("USAGE: deluser <id>")
	}
	id := args[0]
	if resp, err := c.Send("DELETE", "delete-user?id="+id, nil); err != nil {
		fmt.Printf("Delete user id [%s] error: %v\n", id, err)
		return
	} else if !resp.Success {
		fmt.Printf("Delete user id [%s] server error: %v\n", id, resp.Error)
		return
	}

	fmt.Println("OK")

}

func runDaemon(c *Client, _ []string) {
	wsEndpoint := strings.TrimRight(c.Config.Server, "/") + "/ws"
	wsEndpoint = strings.Replace(wsEndpoint, "http://", "ws://", 1)
	wsEndpoint = strings.Replace(wsEndpoint, "https://", "wss://", 1)

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+c.Token)

	conn, _, err := websocket.DefaultDialer.Dial(
		wsEndpoint,
		headers,
	)

	for err != nil {
		fmt.Printf("connection failed: %v; retrying in %ds\n",
			err, c.Config.DaemonRetrySecs)

		time.Sleep(time.Second * time.Duration(c.Config.DaemonRetrySecs))

		conn, _, err = websocket.DefaultDialer.Dial(wsEndpoint, headers)
	}

	defer conn.Close()

	fmt.Println("Connected to server")

	for {
		msgType, msg, err := conn.ReadMessage()

		for err != nil {
			conn.Close()
			fmt.Printf("connection failed: %v; retrying in %ds\n",
				err, c.Config.DaemonRetrySecs)

			time.Sleep(time.Second * time.Duration(c.Config.DaemonRetrySecs))

			conn, _, err = websocket.DefaultDialer.Dial(wsEndpoint, headers)
		}

		fmt.Println("server:", string(msg))

		if msgType == websocket.TextMessage {
			var items []DownloadItem

			if err := json.Unmarshal(msg, &items); err != nil {
				fmt.Printf("error parsing server message: %v\n", err)
				continue
			}

			c.DownloadItems(items)
		}
	}
}
