package main

import (
	"fmt"
	"net/http"
	"io"
	"encoding/json"
	"bytes"
	// "errors"
	"bufio"
	"os"
	"strings"
	"time"
	// "github.com/joho/godotenv"
)




type APIResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
	}

// type ClientConfig struct {
// 	Config interface
// }

type Client struct {
	Conn string
	Token string
	Name string
	Client http.Client
}


func (a APIResponse) String() string {
	r, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return "error json conversion"
	}
	return string(r)

}

func (c *Client) Init(){
	// fmt.Println(os.Getenv("API_TOKEN"), "!!!!!!!!!!!!!!!!!")
	c.Token = os.Getenv("API_TOKEN")
	c.Conn = "http://localhost:7842/"
	c.Client = http.Client{
		Timeout: 5*time.Second,
	} //make all of this load from env and config files
}

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

	req, err := http.NewRequest(method, c.Conn+path, reader)
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



func main() {
	
	var c Client
	c.Init()

	scan := bufio.NewReader(os.Stdin)

	for {


		if c.Token == ""{
			fmt.Printf("Auth for :")
			in, _ := scan.ReadString('\n')
			in = strings.TrimSpace(in)
			if in == "y"{

				r,err := c.Send("POST", "auth",map[string]string{
					"Name":"Name4",
					"Password": "123",
				})
				if err != nil {
					fmt.Printf("%w", err)
					return 
				}

				fmt.Println(r)
				token, ok := r.Data.(string)

				if !ok {
					fmt.Println("errrrror u bs type shi")
				} else {
					c.Token = token
					fmt.Println("yayyyy ", c.Token, " stored in client")
				}
			} else {
				fmt.Println("well ok then")
			}
		} else {
			in, _ := scan.ReadString('\n')
			in = strings.TrimSpace(in)
			if in == "1"{
				r, err := c.Send("GET", "devices", nil)
				if err != nil {
					fmt.Printf("%w", err)
					return 
				}
				fmt.Println(r)
			} else if in == "2"{
				r, err := c.Send("GET", "files", nil)
				if err != nil {
					fmt.Printf("%w", err)
					return 
				}
				fmt.Println(r)

			}else if in == "0"{
				fmt.Println("exit")
				return
			}

		}

		
	}
	

	

	
}