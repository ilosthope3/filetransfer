package main

import (
	"fmt"
	"net/http"
	"os"
	"io"
	"path/filepath"
	"time"
	"encoding/json"
	"errors"
	"strings"
)

const MAINPASSWORD = "123"

type (
	Config struct {
    TokensFile string
    SaveDir    string
		MaxFormSize int64
	}
	APIResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
	}
)

var (
	appConfig Config
	tokenMap map[string]string
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////

func sendJSON(w http.ResponseWriter,success bool, code int, msg interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	resp := APIResponse{Success: success}
	if success {
		resp.Data = msg
	} else {
			
		if errMsg, ok := msg.(string); ok {
			resp.Error = errMsg
		} else {
			resp.Error = "unknown error"
		}
	}
	json.NewEncoder(w).Encode(resp)
}

func authenticate(r *http.Request) (string, error) {
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return "", errors.New("missing Authorization header")
    }

    parts := strings.Split(authHeader, " ")
    if len(parts) != 2 || parts[0] != "Bearer" {
        return "", errors.New("invalid Authorization format")
    }
    token := parts[1]

    deviceName, ok := tokenMap[token]
    if !ok {
        return "", errors.New("invalid token")
    }
    return deviceName, nil
}

func devInit() {
		if err := os.RemoveAll(appConfig.SaveDir); err != nil {
			fmt.Println("deverr1: %v",err)
    }
    if err := os.MkdirAll(appConfig.SaveDir, os.ModePerm); err != nil {
			fmt.Println("deverr2: %v",err)
    }
    if err := os.WriteFile(appConfig.TokensFile, []byte("{}\n"), 0644); err != nil {
			fmt.Println("deverr3: %v",err)
    }
		fmt.Println("files reset")
}

func mainInit() {
	data, err := os.ReadFile(appConfig.TokensFile)
	tokenMap = make(map[string]string)
	if err == nil {
		json.Unmarshal(data, &tokenMap)
	}
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////

func handleAuth(w http.ResponseWriter, r *http.Request) {
	fmt.Println("handling auth")

	var req struct{ Token string }
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	_, ok := tokenMap[req.Token]
	if (ok) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"success": "true"})
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid token"})
		return
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {

	fmt.Println("handling lgoin")
	var req struct{ Password, Name string }
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if (req.Password != MAINPASSWORD) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid password"})
		return
	} 

	token := fmt.Sprintf("token-%d", time.Now().UnixNano())
	
	tokenMap[token] = req.Name
	newData, _ := json.MarshalIndent(tokenMap, "", "  ")
	os.WriteFile(appConfig.TokensFile, newData, 0644)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	_, err := authenticate(r)
	if err != nil {
    sendJSON(w, false, http.StatusUnauthorized, "Unauthorized")
    return
	}

	err = r.ParseMultipartForm(appConfig.MaxFormSize)
	if err != nil {
		sendJSON(w, false, http.StatusBadRequest, "Files too large")
		return
	}

	link := r.FormValue("text")
	receiver := filepath.Base(r.FormValue("receiver"))

	if receiver == "" || receiver == "." || receiver == ".." {
    sendJSON(w, false, http.StatusBadRequest, "Invalid receiver")
    return
	}

	receiverPath := filepath.Join(appConfig.SaveDir, receiver) 

	if err := os.MkdirAll(receiverPath, os.ModePerm); err != nil {
		sendJSON(w, false, http.StatusInternalServerError, "Failed to make upload path")		
		return
	}

	files := r.MultipartForm.File["file"] 

	if len(files) == 0 && len(link) == 0{
		sendJSON(w, false, http.StatusBadRequest, "No files selected")
		return
	}

	if link!= "" {
		linkFileP := filepath.Join(receiverPath, "links.md")

		existing, _ := os.ReadFile(linkFileP)
		
		timestamp := time.Now().Format("2006-01-02 15:04")
		
		new := fmt.Sprintf("[%s] <%s>\n", timestamp, link) 
		content := []byte(new + string(existing))
		
		err:= os.WriteFile(linkFileP, content, 0644)
		if err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "Failed to write to link file")
			return
		}
		
		fmt.Printf("LINK SAVED: %s\n", link)
	}

	for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				sendJSON(w, false, http.StatusInternalServerError, "Failed to open file")
				return
			}
			defer file.Close()

			filename := filepath.Base(fileHeader.Filename)

			dst, err := os.Create(filepath.Join(receiverPath, filename))
			if err != nil {
					sendJSON(w, false, http.StatusInternalServerError, "Failed to create filepath")
					return
			}
			defer dst.Close()

			_, err = io.Copy(dst, file)
			if err != nil {
					sendJSON(w, false, http.StatusInternalServerError, "Failed to copy file")
					return
			}
	}

	sendJSON(w, true, http.StatusOK, "Upload success")
}

func getDeviceNames(w http.ResponseWriter, r *http.Request) {
	deviceName, err := authenticate(r)
	if err != nil {
    sendJSON(w, false, http.StatusUnauthorized, "Unauthorized")
    return
	}

	n := make([]string, 0 ,len(tokenMap))
	for _, name := range tokenMap {
		if name != deviceName{
			n = append(n, name)
		}
		
	}

	sendJSON(w, true, http.StatusOK, n)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {

	appConfig.MaxFormSize = 50 << 20 
	appConfig.TokensFile = "../../data/devices.json"
	appConfig.SaveDir = "../../data/uploads"
	os.MkdirAll(appConfig.SaveDir, os.ModePerm)
	
	devInit()
	mainInit()

	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/auth", handleLogin)
	http.HandleFunc("/whoami", handleAuth)
	http.HandleFunc("/devices", getDeviceNames)

	fmt.Println("started server")
	http.ListenAndServe(":7842", nil)	
	
}