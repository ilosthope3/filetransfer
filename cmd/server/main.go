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
	"github.com/google/uuid"
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
	FileMeta struct {
		ID   string			`json:"id"`
		Size int64			`json:"size,omitempty"`
		Time string			`json:"timestamp,omitempty"`
		Name string			`json:"filename,omitempty"`
		Sender string		`json:"sender,omitempty"`

	}
	LinkMeta struct {
		ID   string			`json:"id"`
		Time string			`json:"timestamp,omitempty"`
		Url string			`json:"url,omitempty"`
		Sender string		`json:"sender,omitempty"`
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
		sendJSON(w, true, http.StatusOK, "Authorized")
	} else {
		sendJSON(w, false, http.StatusUnauthorized, "Invalid Token")
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

  for _, i := range tokenMap {
    if strings.ToLower(i) == strings.ToLower(req.Name) {
      sendJSON(w, false, http.StatusBadRequest, "Name in use")
      return
    }
  }
	if (req.Password != MAINPASSWORD) {
    sendJSON(w, false, http.StatusUnauthorized, "Wrong password")
		return
	} 

	token := fmt.Sprintf("token-%d", time.Now().UnixNano())
	
	tokenMap[token] = req.Name
	newData, _ := json.MarshalIndent(tokenMap, "", "  ")
	os.WriteFile(appConfig.TokensFile, newData, 0644)

  sendJSON(w, true, http.StatusOK, token)
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	senderName, err := authenticate(r)
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

		var linkData LinkMeta
		linkData.Time = time.Now().Format("2006-01-02 15:04")
		linkData.Sender = senderName
		linkData.Url = link
		linkData.ID = uuid.New().String()


		var existing []LinkMeta

		linkFileP := filepath.Join(receiverPath, "links.json")
		data, err := os.ReadFile(linkFileP)
		if err == nil {
			if err := json.Unmarshal(data, &existing); err != nil {
				existing = []LinkMeta{}
			}
		} else {
				
			existing = []LinkMeta{}
		}
		existing = append([]LinkMeta{linkData}, existing...)
		
		newData, err := json.MarshalIndent(existing, "", "  ")
		if err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "Failed to encode links")
			return
		}

		if err := os.WriteFile(linkFileP, newData, 0644); err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "Failed to write link file")
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

		var metadata FileMeta
		metadata.Name = filename
		metadata.Time = time.Now().Format("2006-01-02 15:04")
		metadata.Sender = senderName
		metadata.Size = fileHeader.Size
		metadata.ID = uuid.New().String()

		dst, err := os.Create(filepath.Join(receiverPath, metadata.ID))
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

		
		var existing []FileMeta

		fileFileP := filepath.Join(receiverPath, "files.json")
		data, err := os.ReadFile(fileFileP)
		if err == nil {
			if err := json.Unmarshal(data, &existing); err != nil {
				existing = []FileMeta{}
			}
		} else {
			existing = []FileMeta{}
		}
		existing = append([]FileMeta{metadata}, existing...)
		
		newData, err := json.MarshalIndent(existing, "", "  ")
		if err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "Failed to encode file meta")
			return
		}

		if err := os.WriteFile(fileFileP, newData, 0644); err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "Failed to write files file")
			return
		}
			
		// metaFile, err := os.Create(filepath.Join(receiverPath, metadata.ID + ".meta.json"))
		// if err != nil {
		// 	sendJSON(w, false, http.StatusInternalServerError, "Failed to create meta file")
		// 	return
		// }
		// defer metaFile.Close()

		// encoder := json.NewEncoder(metaFile)
		// encoder.SetIndent("", "  ") // Makes the JSON human-readable
		// if err := encoder.Encode(metadata); err != nil {
		// 	sendJSON(w, false, http.StatusInternalServerError, "Failed to write metadata")
		// 	return
		// }
	}
	sendJSON(w, true, http.StatusOK, "Upload success")
}

func handleDeviceNames(w http.ResponseWriter, r *http.Request) {
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

func handleFileList(w http.ResponseWriter, r *http.Request) {
	deviceName, err := authenticate(r)
	if err != nil {
		sendJSON(w, false, http.StatusUnauthorized, "Unauthorized")
		return
	}

	filePath := filepath.Join(appConfig.SaveDir, deviceName, "files.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		sendJSON(w, false, http.StatusInternalServerError, "Failed to read existing data")
		return
	}

	var entries []FileMeta
	if err := json.Unmarshal(data, &entries); err != nil {
		sendJSON(w, false, http.StatusInternalServerError, "Failed to parse metadata")
		return
	}

	

	sendJSON(w, true, http.StatusOK, entries)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {

	sender, err := authenticate(r)
	if err != nil {
		sendJSON(w, false, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := r.URL.Query().Get("id")
	if len(id) == 0 {
		sendJSON(w, false, http.StatusBadRequest, "invalid id delete argument")
		return
	}

	if id == "all" {
		if err := os.RemoveAll(filepath.Join(appConfig.SaveDir, sender)); err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "failed to delete dir")
			return
    }
	} else {

		
		err = os.Remove(filepath.Join(appConfig.SaveDir, sender, id))
		if err != nil {

			if os.IsNotExist(err) {
				fmt.Printf("file missing: non fatal: cleaning json\n")
			} else {	
				sendJSON(w, false, http.StatusInternalServerError, "Failed to delete physical file (perms err etc)")
				return
			}
		}
		filePath := filepath.Join(appConfig.SaveDir, sender, "files.json")
		data, err := os.ReadFile(filePath)
		if err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "Failed to read existing metadata")
			return
		}

		var entries, newEntries []FileMeta
		if err := json.Unmarshal(data, &entries); err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "Failed to parse metadata")
			return
		}

		flag:= true
		for _, i := range entries {
			if id != i.ID {
				newEntries = append(newEntries, i)
			} else {
				flag = false
			}
		}
		if flag {
			sendJSON(w, false, http.StatusBadRequest, "No id found")
			return
		}

		newData, err := json.MarshalIndent(newEntries, "", "  ")
		if err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "Failed to encode editted metadata")
			return
		}

		if err := os.WriteFile(filePath, newData, 0644); err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "Failed to write files file")
			return
		}

	}

	sendJSON(w, true, http.StatusOK, "delete successful")

}
///////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {

	appConfig.MaxFormSize = 50 << 20 
	appConfig.TokensFile = "data/devices.json"
	appConfig.SaveDir = "data/uploads"
	os.MkdirAll(appConfig.SaveDir, os.ModePerm)
	
	// devInit()
	mainInit()

	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/auth", handleLogin)
	http.HandleFunc("/whoami", handleAuth)
	http.HandleFunc("/devices", handleDeviceNames)
	http.HandleFunc("/files", handleFileList)
	http.HandleFunc("/delete", handleDelete)

	fmt.Println("started server")
	http.ListenAndServe(":7842", nil)	
	
}