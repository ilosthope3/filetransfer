package main

import (
	"fmt"
	"net/http"
	// "io"
	"encoding/json"
)


type APIResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
	}


func (a APIResponse) String() string {
	// r := ""
	// if a.Success {
	// 	r += fmt.Sprintf("SUCCESS: TRUE;\nDATA: %v", a.Data)
		
	// } else {
	// 	r += "SUCCESS: FALSE;\nERROR: " + a.Error

	// }
	// return r


	r, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return "error json conversion"
	}
	return string(r)

} 


func main() {
	resp, err := http.Get("http://localhost:7842/files")
	if err != nil {
		fmt.Println("error: ", err)
		return
	}
	
	defer resp.Body.Close()

	// Accessing the struct fields
	fmt.Printf("Type: %T\n", resp)         // Output: *http.Response
	fmt.Println("Status:", resp.StatusCode) // Output: 200


	// bytes, err := io.ReadAll(resp.Body) 
	// if err != nil {
	// 	fmt.Printf("error while read")
	// 	return
	// }

	// fmt.Printf(string(bytes))	

	// fmt.Println("body:", resp.Body) 

	var apiR APIResponse
	err = json.NewDecoder(resp.Body).Decode(&apiR)
	if err != nil {
		fmt.Printf("err decoding: ", err)
		return
	}

	fmt.Println(apiR)
}