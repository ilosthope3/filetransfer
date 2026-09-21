package main

import (
	"fmt"
	"net/http"
	"io"
	"encoding/json"
	"bytes"
	// "errors"
	"sort"
	"os"
	"strings"
	"time"
	"path/filepath"
	"mime/multipart"
	// "log"
	// "github.com/joho/godotenv"
)

type ClientConfig struct {
	Server              string `json:"server"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
	SaveDir             string `json:"save_dir"`
	AutoDownload        bool   `json:"autodownload"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type Client struct {
	Token string
	Name string
	Client http.Client
	Config ClientConfig
}

func (a APIResponse) String() string {
	r, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return "error json conversion"
	}
	return string(r)

}

func loadConfig() (ClientConfig, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".filetransfer", "config.json")
	

	b, err := os.ReadFile(path)
	if err != nil {
		return ClientConfig{}, fmt.Errorf("read config: %w", err)
	}

	var cfg ClientConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return ClientConfig{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// func (c *Client) Init(){
// 	// fmt.Println(os.Getenv("API_TOKEN"), "!!!!!!!!!!!!!!!!!")
// 	conf, err := loadConfig()
// 	if err!= nil {
// 		fmt.Printf("Error loading config %v \n", err)
// 		os.Exit(1)
// 	}
// 	c.Config = conf
// 	c.Token = os.Getenv("API_TOKEN")
// 	c.Client = http.Client{
// 		Timeout: 5*time.Second,
// 	}

// 	// resp

	
// }


func (c *Client) Send(method, path string, data any) (*APIResponse, error) {
	var reader io.Reader

	if method == "POST" && data==nil { //post eq always has a body
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
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

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

func (c *Client) SendRaw(method, path, contentType string, body io.Reader) (*APIResponse, error) {
	req, err := http.NewRequest(method, c.Config.Server+path, body)
	if err != nil {
			return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
	}

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

func (c *Client) Init() {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".filetransfer")

	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Println("failed to create app dir:", err)
		os.Exit(1)
	}

	cfgPath := filepath.Join(dir, "config.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		defaultCfg := ClientConfig{
			Server:              "http://localhost:7842/",
			TimeoutSeconds:      5,
			SaveDir:             filepath.Join(dir, "downloads"),
			AutoDownload:        false,
			PollIntervalSeconds: 30,
		}
		b, _ := json.MarshalIndent(defaultCfg, "", "  ")
		if err := os.WriteFile(cfgPath, b, 0o600); err != nil {
			fmt.Println("failed to write default config:", err)
			os.Exit(1)
		}
	}

	tokPath := filepath.Join(dir, "token")
	if _, err := os.Stat(tokPath); os.IsNotExist(err) {
		if err := os.WriteFile(tokPath, []byte{}, 0o600); err != nil {
			fmt.Println("failed to create token file:", err)
			os.Exit(1)
		}
	}

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Println("failed to read config:", err)
		os.Exit(1)
	}
	if err := json.Unmarshal(b, &c.Config); err != nil {
		fmt.Println("failed to parse config:", err)
		os.Exit(1)
	}

	c.Token = os.Getenv("API_TOKEN")
	if c.Token == "" {
		if tb, err := os.ReadFile(tokPath); err == nil {
			c.Token = strings.TrimSpace(string(tb))
		}
	}

	timeout := time.Duration(c.Config.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	c.Client = http.Client{Timeout: timeout}
}

func saveToken(token string) error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".filetransfer", "token")
	return os.WriteFile(path, []byte(token), 0o600)
}

func main() {
	

	var c Client
	c.Init()

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "auth":
		cmdAuth(&c, args)
		usage()
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
	default:
		fmt.Printf("unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
    fmt.Println(`usage: filetransfer <command> [args]

	commands:
  auth <name> <password>     authenticate and get a token
  devices                    list registered devices
  inbox                      list pending files/text
  file <receiver>:<path>     send a file
  text <receiver>:<url>      send a link
  whoami                     currently logged in as which user`)
}

func cmdAuth(c *Client, args []string) (string, string, error) {
	if len(args) != 2 {
		return "", "", fmt.Errorf("usage: filetransfer auth <name> <password>")
	}

	r, err := c.Send("POST", "auth", map[string]string{
		"Name":     args[0],
		"Password": args[1],
	})
	if err != nil {
		return "", "", fmt.Errorf("auth request: %w", err)
	}

	token, ok := r.Data.(string)
	if !ok || token == "" {
		return "", "", fmt.Errorf("server returned unexpected data")
	}

	if err := saveToken(token); err != nil {
		return "", "", fmt.Errorf("save token: %w", err)
	}

	return args[0], token, nil
}

func cmdDevices(c *Client, args []string) {
	r, err := c.Send("GET", "devices", nil)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
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

	for _, d := range devices {
		fmt.Printf("\t%v\t%v\n", d.ID, d.Name)
	}
}

func cmdFiles(c *Client, args []string) {
	r, err := c.Send("GET", "files", nil)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	fmt.Println(r)
}

func cmdWho(c *Client, args []string) {
	r, err := c.Send("GET", "whoami", nil)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	fmt.Println(r)
}

func cmdSend(c *Client, args []string) {
	if len(args) != 1 {
		fmt.Println("usage: filetransfer send <receiver>:<path>")
		os.Exit(1)
	}

	receiver, path, found := strings.Cut(args[0], ":")
	if !found {
		fmt.Println("format must be <receiver>:<path>")
		os.Exit(1)
	}

	body, contentType, err := buildUploadBody(strings.TrimSpace(receiver), strings.TrimSpace(path), "")
	if err != nil {
		fmt.Println("error building body:", err)
		os.Exit(1)
	}

	resp, err := c.SendRaw("POST", "upload", contentType, body)
	if err != nil {
		fmt.Println("error uploading:", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdLink(c *Client, args []string) {
	if len(args) != 1 {
		fmt.Println("usage: filetransfer link <receiver>:<url>")
		os.Exit(1)
	}

	receiver, url, found := strings.Cut(args[0], ":")
	if !found {
		fmt.Println("format must be <receiver>:<url>")
		os.Exit(1)
	}

	body, contentType, err := buildUploadBody(strings.TrimSpace(receiver), "", strings.TrimSpace(url))
	if err != nil {
		fmt.Println("error building body:", err)
		os.Exit(1)
	}

	resp, err := c.SendRaw("POST", "upload", contentType, body)
	if err != nil {
		fmt.Println("error uploading:", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}