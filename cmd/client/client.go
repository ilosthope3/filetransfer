package main

import (
	"bytes"
	"context"
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
)

type ClientConfig struct {
	Server              string `json:"server"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
	SaveDir             string `json:"save_dir"`
	AutoDownload        bool   `json:"autodownload"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
}

type UploadType string

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type Client struct {
	Token  string
	Name   string
	Client http.Client
	Config ClientConfig
}

type Dir struct {
	Files   []string `json:"files"`
	DirName string   `json:"name"`
	Dirs    []Dir    `json:"dirs,omitempty"`
}

const (
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
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.Config.Server+path, reader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	var out APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response (HTTP %d): %w", resp.StatusCode, err)
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

func (c *Client) Download(uuid, path string) error {
	req, err := http.NewRequest("POST", c.Config.Server+"download?uuid="+uuid, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server %d: %s", resp.StatusCode, body)
	}

	cd := resp.Header.Get("Content-Disposition")
	if cd == "" {
		return fmt.Errorf("server sent no Content-Disposition")
	}
	_, params, err := mime.ParseMediaType(cd)
	if err != nil {
		return fmt.Errorf("bad Content-Disposition %q: %w", cd, err)
	}
	name := params["filename"]
	if name == "" {
		return fmt.Errorf("no filename in %q", cd)
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
			return fmt.Errorf("stat %s: %w", dest, err)
		}
		dest = filepath.Join(path,
			fmt.Sprintf("%s (%d)%s", base, i, ext))
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return fmt.Errorf("short read: %d/%d", n, resp.ContentLength)
	}
	return out.Close()
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
			Server:              "http://localhost:7842/",
			TimeoutSeconds:      5,
			SaveDir:             "~/Downloads/filetransfer",
			AutoDownload:        false,
			PollIntervalSeconds: 30,
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

func buildUploadBody(receiver, filePath, link string) (*bytes.Buffer, string, error) {
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
	w.WriteField("receiver", receiver)
	w.Close()

	return &body, w.FormDataContentType(), nil
}

func buildDirManifestBody(receiver string, root Dir) (*bytes.Buffer, string, error) {
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

	if err := w.WriteField("receiver", receiver); err != nil {
		return nil, "", err
	}

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
	case "download":
		cmdDownload(&c, args)
	case "cfg":
		cmdEditCFG(&c, args)
	case "delete":
		cmdDelete(&c, args)
	case "deleteuser":
		cmdDeleteUser(&c, args)
	case "dir":
		cmdDir(&c, args)
	default:
		fmt.Printf("Unknown command: %s\n\n", cmd)
		cmdHelp()
		os.Exit(1)
	}
}

func cmdHelp() {
	fmt.Println(`  ./filetransfer <command> [args]
  COMMANDS:
    auth <name> <password>     authenticate and get a token
    whoami                     currently logged in as which user
    devices                    list registered devices
    inbox                      list pending files/text
    download all|<id> [<id>]   download files/text
    delete all|<id> [<id>]     delete files/text
    file <receiver>:<path>     send a file
    text <receiver>:<url>      send a link
    dir  <reciever>:<path>     send a folder
    cfg                        current config and config path
    help                       this menu`)
	fmt.Println()
}

func cmdAuth(c *Client, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: filetransfer auth <name> <password>")
	}

	r, err := c.Send("POST", "auth", map[string]string{
		"Name":     args[0],
		"Password": args[1],
	})
	if err != nil {
		return fmt.Errorf("auth request: %w", err)
	}

	token, ok := r.Data.(string)
	if !ok || token == "" {
		return fmt.Errorf("server returned unexpected data")
	}

	if err := saveToken(token); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	return nil
}

func cmdDevices(c *Client, args []string) {
	r, err := c.Send("GET", "devices", nil)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	list, ok := r.Data.([]any)
	if !ok {
		fmt.Println("unexpected response shape")
		return
	}

	type Device struct {
		ID   float64
		Name string
	}

	var devices []Device
	for _, entry := range list {
		dev, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		id, _ := dev["id"].(float64)
		name, _ := dev["name"].(string)
		devices = append(devices, Device{ID: id, Name: name})
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].ID < devices[j].ID
	})

	fmt.Println("\n  DEVICES:")
	for _, d := range devices {
		fmt.Printf("    [%v] %v\n", d.ID, d.Name)
	}
	fmt.Println()
}

func cmdFiles(c *Client, args []string) {

	fmt.Println("\n  INBOX:")
	list, err := getFiles(c)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	if len(list) == 0 {
		fmt.Println("Empty")
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
			fmt.Printf("    [%s] T  %-60s  from [%s]  %s\n", id, truncate(text, 60), sender, date)
		} else {
			name := fmt.Sprintf("%v", row["filename"])
			fmt.Printf("    [%s] F  %-60s  from [%s]  %s\n", id, truncate(name, 60), sender, date)
		}
	}
	fmt.Println()
}

func cmdWho(c *Client, args []string) {
	r, err := c.Send("GET", "whoami", nil)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	type Device struct {
		ID   float64
		Name string
	}

	dev, ok := r.Data.(map[string]any)
	if !ok {
		fmt.Println("Error when parsing response")
		return
	}
	id, _ := dev["id"].(float64)
	name, _ := dev["name"].(string)

	fmt.Printf("\n  Logged in as [%v] - %v\n\n", id, name)

}

func cmdDir(c *Client, args []string) {
	if len(args) != 1 {
		fmt.Println("filetransfer dir <receiver>:<path>")
		os.Exit(1)
	}

	receiver, path, found := strings.Cut(args[0], ":")
	if !found {
		fmt.Println("Format must be <receiver>:<path>")
		os.Exit(1)
	}

	f, err := os.Stat(path)
	if os.IsNotExist(err) {
		fmt.Println("no such path found")
		os.Exit(1)
	}

	if !f.IsDir() {
		fmt.Println("path is a file: use file command")
		os.Exit(1)
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
				body, contentType, err := buildUploadBody(strings.TrimSpace(receiver), strings.TrimSpace(cpath), "")
				if err != nil {
					fmt.Printf("error building body: %s\n%v\n", cpath, err)
					continue
				}

				resp, err := c.SendRaw("POST", "upload", contentType, body, UploadDirChild)
				if err != nil {
					fmt.Printf("error uploading: %s\n%v\n", cpath, err)
					continue
				}
				if !resp.Success {
					fmt.Printf("error: %s\n%v\n", cpath, resp.Error)
					continue
				}

				r.Files = append(r.Files, resp.Data.(string))

			}
		}

		if len(r.Dirs) == 0 {
			r.Dirs = nil
		}
		if len(r.Files) == 0 {
			r.Files = nil
		}
		return r
	}

	root := traverseDir(path)

	body, contentType, err := buildDirManifestBody(receiver, root)
	if err != nil {
		fmt.Printf("error building manifest: %v\n", err)
		return
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
	fmt.Println("dir saved ok")

}

func cmdSend(c *Client, args []string) {
	if len(args) != 1 {
		fmt.Println("filetransfer send <receiver>:<path>")
		os.Exit(1)
	}

	receiver, path, found := strings.Cut(args[0], ":")
	if !found {
		fmt.Println("Format must be <receiver>:<path>")
		os.Exit(1)
	}

	body, contentType, err := buildUploadBody(strings.TrimSpace(receiver), strings.TrimSpace(path), "")
	if err != nil {
		fmt.Println("error building body:", err)
		os.Exit(1)
	}

	resp, err := c.SendRaw("POST", "upload", contentType, body, UploadFile)
	if err != nil {
		fmt.Println("error uploading:", err)
		os.Exit(1)
	}
	if resp.Success {
		fmt.Println("OK")
	} else {
		fmt.Printf("Error when sending file: %v\n", resp.Error)
	}
}

func cmdLink(c *Client, args []string) {
	if len(args) < 1 {
		fmt.Println("filetransfer link <receiver>:<url>")
		os.Exit(1)
	}

	inp := strings.Join(args, " ")
	receiver, url, found := strings.Cut(inp, ":")
	if !found {
		fmt.Println("Format must be <receiver>:<url>")
		os.Exit(1)
	}

	body, contentType, err := buildUploadBody(receiver, "", url)
	if err != nil {
		fmt.Println("error building body:", err)
		os.Exit(1)
	}

	resp, err := c.SendRaw("POST", "upload", contentType, body, UploadFile)
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
		fmt.Println("download all|<fileID> [<fileID> ...]")
		return
	}

	files, err := getFiles(c)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	for _, i := range args {
		fmt.Println("   hehe: ", i)
	}
	for _, i := range files {
		fmt.Println("   hehe: ", i)
	}

	var fileIDs []string
	if args[0] == "all" {
		for _, f := range files {
			if uuid, ok := f["uuid"].(string); ok {
				fileIDs = append(fileIDs, uuid)
			}
		}
	} else {
		for _, f := range files {
			id := fmt.Sprint(f["id"])
			if slices.Contains(args, id) {
				if uuid, ok := f["uuid"].(string); ok {
					fileIDs = append(fileIDs, uuid)
					fmt.Printf("%s %s debug booger man\n", id, uuid)
				}
			}
		}
	}

	fmt.Println("what da helly", len(files), len(fileIDs))

	//!!!! fileIDS now has uuids =, could be bothered refactoring
	for _, id := range fileIDs {
		if err := c.Download(id, c.Config.SaveDir); err != nil {
			fmt.Printf("ID %s\terror: %v\n", id, err)
			continue
		}
		if resp, err := c.Send("DELETE", "delete?uuid="+id, nil); err != nil {
			fmt.Printf("ID %s\tdownloaded, confirm failed: %v\n", id, err)
			continue
		} else if !resp.Success {
			fmt.Printf("ID %s\tdownloaded, confirm failed: %v\n", id, resp.Error)
			continue
		}

		fmt.Printf("ID %s\tOK\n", id)
	}
}

func cmdDelete(c *Client, args []string) {
	if len(args) == 0 {
		fmt.Println("delete all|<fileID> [<fileID> ...]")
		return
	}

	var fileIDs []string
	if args[0] == "all" {
		files, err := getFiles(c)
		if err != nil {
			fmt.Println("error:", err)
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
			fmt.Printf("ID %s\tdelete failed: %v\n", id, err)
			continue
		} else if !resp.Success {
			fmt.Printf("ID %s\tdelete failed: %v\n", id, resp.Error)
			continue
		}

		fmt.Printf("ID %s\tdeleted\n", id)
	}

}

func cmdEditCFG(c *Client, args []string) {
	b, err := json.MarshalIndent(c.Config, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("failed to find home dir:", err)
		os.Exit(1)
	}
	path := filepath.Join(home, ".filetransfer", "client_config.json")
	fmt.Println("\n\nCONFIG PATH:", path)
	fmt.Println(string(b))
}

func cmdDeleteUser(c *Client, args []string) {
	if len(args) == 0 {
		fmt.Println("deleteUser <id>")
	}
	id := args[0]
	if resp, err := c.Send("DELETE", "delete-user?id="+id, nil); err != nil {
		fmt.Printf("ID %s\tdelete failed: %v\n", id, err)
		return
	} else if !resp.Success {
		fmt.Printf("ID %s\tdelete failed: %v\n", id, resp.Error)
		return
	}

	fmt.Printf("ID %s\tdeleted\n", id)

}

func runDaemon(ctx context.Context, c *Client) {
	t := time.NewTicker(time.Duration(c.Config.PollIntervalSeconds))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cmdDownload(c, []string{"all"})
		}
	}
}
