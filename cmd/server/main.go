package main

import (
	"fmt"
	"net/http"
	// "html/template"
	// "strings"
	"os"
	"io"
	"path/filepath"
	"time"
	"encoding/json"
	// "embed"
  // "html/template"
)



type Config struct {
    TokensFile string
    SaveDir    string
}

const MAINPASSWORD = "123"

var appConfig Config




func handleAuth(w http.ResponseWriter, r *http.Request) {
	fmt.Println("handling auth")

	var req struct{ Token string }
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	
	data, err := os.ReadFile(appConfig.TokensFile)
	tokensMap := make(map[string]string)
	json.Unmarshal(data, &tokensMap)

	_, ok := tokensMap[req.Token]
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

	
	
	tokensMap := make(map[string]string)

	
	data, err := os.ReadFile(appConfig.TokensFile)
	if err == nil {
		// If file exists, unmarshal it
		json.Unmarshal(data, &tokensMap)
	}
	
	tokensMap[token] = req.Name
	newData, _ := json.MarshalIndent(tokensMap, "", "  ")
	os.WriteFile(appConfig.TokensFile, newData, 0644)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func handleUpload(w http.ResponseWriter, r *http.Request) {


    redirectError := func(msg string) {
        fmt.Printf("ERROR: %s\n", msg)
        http.Redirect(w, r, "/?success=false", http.StatusSeeOther)
    }
		
    err := r.ParseMultipartForm(50 << 20)
    if err != nil {
        redirectError("Form too large or invalid")
        return
    }
		link := r.FormValue("text")
    receiver := r.FormValue("receiver")
		if receiver == "" {
			receiver = "default"
		}

    receiverPath := filepath.Join(appConfig.SaveDir, receiver) 

    if err := os.MkdirAll(receiverPath, os.ModePerm); err != nil {
        redirectError("Could not create uploads folder")
        return
    }

    files := r.MultipartForm.File["file"] 

    if len(files) == 0 && len(link) == 0{
        redirectError("No files provided")
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
				redirectError("failed write")
				return
			}
			
			fmt.Printf("LINK SAVED: %s\n", link)
		}

    for _, fileHeader := range files {
        file, err := fileHeader.Open()
        if err != nil {
            redirectError("Failed to open uploaded file")
            return
        }
        defer file.Close()

        filename := filepath.Base(fileHeader.Filename)

        dst, err := os.Create(filepath.Join(receiverPath, filename))
        if err != nil {
            redirectError("Failed to create file: " + filename)
            return
        }
        defer dst.Close()

        _, err = io.Copy(dst, file)
        if err != nil {
            redirectError("Failed to save file: " + filename)
            return
        }
    }

    http.Redirect(w, r, "/?success=true", http.StatusSeeOther)
}

func getDataDir() string {
	exe, err := os.Executable()
	if err != nil {
			return "./data" 
	}
	dir := filepath.Dir(exe) 
	return filepath.Join(dir, "data")
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

func main() {

	dataDir := getDataDir()
	appConfig.TokensFile = filepath.Join(dataDir, "devices.json")
	appConfig.SaveDir = filepath.Join(dataDir, "uploads")
	os.MkdirAll(appConfig.SaveDir, os.ModePerm)
	
	devInit()

	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/auth", handleLogin)
	http.HandleFunc("/whoami", handleAuth)
	fmt.Println("started server")
	http.ListenAndServe(":7842", nil)	
	
}