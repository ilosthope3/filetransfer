package main

import (
	"fmt"
	"net/http"
	// "html/template"
	"strings"
	"os"
	"io"
	"path/filepath"
	"time"
	"encoding/json"
	"embed"
  "html/template"
)

const MAINPASSWORD = "123"

//go:embed web/index.html
var indexHTML string



func mainPage(w http.ResponseWriter, r *http.Request) {
	
	successMsg := func(s string) string {
		if strings.ToLower(s) == "true" {
			return `<p style="color:green; font-weight:bold;">Upload successful</p>`
		}
		if strings.ToLower(s) == "false" { return `<p style="color:red; font-weight:bold;">Upload failed</p>`}
		return ""
	}

	getNames := func() string {
		r:= ""

		tokensFile := "devices.json"
		data, _ := os.ReadFile(tokensFile)
		tokensMap := make(map[string]string)
		json.Unmarshal(data, &tokensMap)

		for i := range tokensMap {
			r += fmt.Sprintf(`<option value="%s">%s</option>`, tokensMap[i], tokensMap[i])
		}

		if (r!= "") {
			r = `<select name="receiver">` + r + `</select>`
		}
		return r
	}
	
	var htmlData struct {
		Success := successMsg(r.URL.Query().Get("success"))
		DevicesNames := getNames()
	}

	temp, err := template.New("index").Parse(indexHTML)
	if err != nil {
		http.Error("error parsing mainpage", http.StatusInternalServerError)
		return
	}

	data := struct {
		SuccessMsg   template.HTML
		DeviceNames template.HTML
	}{
		SuccessMsg:   template.HTML(successMsg(r.URL.Query().Get("success"))),   // Mark as safe HTML
		DeviceOptions: template.HTML(getNames()), // Mark as safe HTML
	}
}

func handleAuth(w http.ResponseWriter, r *http.Request) {
	fmt.Println("handling auth")

	var req struct{ Token string }
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	tokensFile := "devices.json"
	data, err := os.ReadFile(tokensFile)
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

	
	tokensFile := "devices.json"
	tokensMap := make(map[string]string)

	
	data, err := os.ReadFile(tokensFile)
	if err == nil {
		// If file exists, unmarshal it
		json.Unmarshal(data, &tokensMap)
	}
	
	tokensMap[token] = req.Name
	newData, _ := json.MarshalIndent(tokensMap, "", "  ")
	os.WriteFile(tokensFile, newData, 0644)

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

    saveDir := filepath.Join("uploads", receiver) 

    if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
        redirectError("Could not create uploads folder")
        return
    }

		

    files := r.MultipartForm.File["file"] // This is a slice now

    if len(files) == 0 && len(link) == 0{
        redirectError("No files provided")
        return
    }

		if link!= "" {
			linkFileP := filepath.Join(saveDir, "links.md")

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

        // Sanitize filename
        filename := filepath.Base(fileHeader.Filename)

        // Create the destination file on disk
        dst, err := os.Create(filepath.Join(saveDir, filename))
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

func main() {
	os.MkdirAll("./uploads", os.ModePerm)
	http.HandleFunc("/", mainPage)
	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/auth", handleLogin)
	http.HandleFunc("/whoami", handleAuth)
	fmt.Println("started server")
	http.ListenAndServe(":7842", nil)	
}