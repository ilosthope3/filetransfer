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
	"log"
	"crypto/rand"
	"encoding/hex"
	"database/sql"
)



type (
	Config struct {
    TokensFile string
    SaveDir    string
		MaxFormSize int64
		Password 		string
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
  FullReturn struct {
    Files []FileMeta  `json:"files"`
    Links []LinkMeta  `json:"links"`
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

func authenticate(r *http.Request) (UserRecord, error) {
		var row UserRecord

    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return row, errors.New("missing Authorization header")
    }

    parts := strings.Split(authHeader, " ")
    if len(parts) != 2 || parts[0] != "Bearer" {
        return row, errors.New("invalid Authorization format")
    }
    token := parts[1]
		row.Token = token 
		err := DB.QueryRow("SELECT id, username from USERS WHERE TOKEN = ?", token).Scan(&row.ID, &row.Name)
		if err != nil {
			if err == sql.ErrNoRows {
				return row, errors.New("invalid token")
			}
			return row, errors.New("db error during euth service")

		}
		return row, nil
		
    
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
	
	appConfig.MaxFormSize = 50 << 20 
	appConfig.SaveDir = "data"
	appConfig.Password = "123"

	if err := os.MkdirAll(appConfig.SaveDir, 0o755); err != nil {
    log.Fatalf("create save dir: %v", err)
	}

	
}

func readFileMeta(path string) ([]FileMeta, error) {
  data, err := os.ReadFile(path)
  if err != nil {
    return nil, err
  }
  var entries []FileMeta
  if err := json.Unmarshal(data, &entries); err != nil {
    return nil, err
  }
  return entries, nil
}

func writeFileMeta(path string, entries []FileMeta) error {
  data, err := json.MarshalIndent(entries, "", "  ")
  if err != nil {
    return err
  }
  return os.WriteFile(path, data, 0644)
}

func readLinkMeta(path string) ([]LinkMeta, error) {
  data, err := os.ReadFile(path)
  if err != nil {
    return nil, err
  }
  var entries []LinkMeta
  if err := json.Unmarshal(data, &entries); err != nil {
    return nil, err
  }
  return entries, nil
}

func writeLinkMeta(path string, entries []LinkMeta) error {
  data, err := json.MarshalIndent(entries, "", "  ")
  if err != nil {
    return err
  }
  return os.WriteFile(path, data, 0644)
}

func filterFileMeta(entries []FileMeta, id string) ([]FileMeta, bool) {
  found := false
  result := []FileMeta{}
  for _, e := range entries {
    if e.ID == id {
      found = true
      continue
    }
    result = append(result, e)
  }
  return result, found
}

func filterLinkMeta(entries []LinkMeta, id string) ([]LinkMeta, bool) {
  found := false
  result := []LinkMeta{}
  for _, e := range entries {
    if e.ID == id {
      found = true
      continue
    }
    result = append(result, e)
  }
  return result, found
}

func readOrCreateJSON(filePath string, v interface{}) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			if writeErr := os.WriteFile(filePath, []byte("[]\n"), 0644); writeErr != nil {
				return fmt.Errorf("failed to create file: %w", writeErr)
			}
			return nil
		}
		return fmt.Errorf("failed to read file: %w", err)
	}
	fmt.Println(data)
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	return nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////

func handleAuth(w http.ResponseWriter, r *http.Request) {
	fmt.Println("handling auth")

	var req struct{ Token string }
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		sendJSON(w, false, http.StatusBadRequest, "Invalid JSON")
		return
	}

	_, ok := tokenMap[req.Token]
	if (ok) {
		sendJSON(w, true, http.StatusOK, "Authorized")
	} else {
		sendJSON(w, false, http.StatusUnauthorized, "Invalid Token")
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct{ Password, Name string }
	var row struct{ Username, Token string}
	fmt.Println("handling lgoin")
	
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		sendJSON(w, false, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if strings.ContainsAny(req.Name, `/\..`) {
		sendJSON(w, false, http.StatusBadRequest, "Bad name")
		return
	}

	if (req.Password != appConfig.Password) {
    sendJSON(w, false, http.StatusUnauthorized, "Wrong password")
		return
	} 
	
	err = DB.QueryRow("SELECT username, token FROM users WHERE username = ?", req.Name).Scan(&row.Username, &row.Token)

	if err == sql.ErrNoRows {

		b := make([]byte, 32) 
    if _, err := rand.Read(b); err != nil {
        log.Fatal("rand failed:", err)
    }
		token := fmt.Sprintf("token-%s",  hex.EncodeToString(b))

		if _, err = DB.Exec("INSERT INTO users (username, token) VALUES(? , ?);", req.Name, token); err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "failed adding new user to db")
			return
		}
		// refreshUsers()
		sendJSON(w, true, http.StatusOK, token)
		return
	} 

	if err != nil {
		sendJSON(w, false, http.StatusInternalServerError, "DB query to check if users contains login failed")
		return
	}

	
	
	sendJSON(w, true, http.StatusOK, row.Token)
	return 
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	sender, err := authenticate(r)
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
	receiver := r.FormValue("receiver")

	if receiver == "" || receiver == "." || receiver == ".." {
    sendJSON(w, false, http.StatusBadRequest, "Invalid receiver")
    return
	}

	var exists bool
	if err = DB.QueryRow(
    "SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)",
    receiver
	).Scan(&exists); err!= nil {
		sendJSON(w, false, http.StatusBadRequest, "No files selected")
		return
	}
	if !exists {
		sendJSON(w, false, http.StatusBadRequest, "No such receiver")
		return
	}
	
	files := r.MultipartForm.File["file"] 
	if len(files) == 0 && len(link) == 0{
		sendJSON(w, false, http.StatusBadRequest, "No files selected")
		return
	}

	if link!= "" {
		DB.exec("INSERT INTO items ()")
		var linkData LinkMeta
		linkData.Time = time.Now().Format("2006-01-02 15:04")
		linkData.Sender = sender.Name
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
		metadata.Sender = sender.Name
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
			
	}
	sendJSON(w, true, http.StatusOK, "Upload success")
}

func handleDeviceNames(w http.ResponseWriter, r *http.Request) {
	device, err := authenticate(r)
	if err != nil {
    sendJSON(w, false, http.StatusUnauthorized, "Unauthorized")
    return
	}

	var users []UserRecord
	rows, err := DB.Query(
		`SELECT id, username
		FROM users
		WHERE id != ?`,
		device.ID,
	)
	if err!= nil {
		sendJSON(w, false, http.StatusInternalServerError, "error when searching db")
		return
	}
	defer rows.Close()

	for rows.Next() {
		
		var u UserRecord
		if err:= rows.Scan(&u.ID, &u.Name); err!= nil {
			sendJSON(w, false, http.StatusInternalServerError, "error when iterating over db")
			return
		}
		users = append(users, u)

	}

	if rows.Err() != nil {
		sendJSON(w, false, http.StatusInternalServerError, "error when iterating over db")
		return
	}

	sendJSON(w, true, http.StatusOK, users)
}

func handleFileList(w http.ResponseWriter, r *http.Request) {
	device, err := authenticate(r)
	if err != nil {
		sendJSON(w, false, http.StatusUnauthorized, "unauthorized")
		return
	}

	files := []FileRecord{}

	rows, err := DB.Query(
		`SELECT id, uuid, sender_id, receiver_id, type, filename, size, url, uploaded_at, consumed, consumed_at
		FROM items
		WHERE receiver_id = ? AND consumed = FALSE`,
		device.ID,
	)
	if err != nil {
		sendJSON(w, false, http.StatusInternalServerError, "db query failed")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var f FileRecord
		if err := rows.Scan(
			&f.ID,
			&f.UUID,
			&f.SenderID,
			&f.ReceiverID,
			&f.Type,
			&f.Filename,
			&f.Size,
			&f.URL,
			&f.UploadedAt,
			&f.Consumed,
			&f.ConsumedAt,
		); err != nil {
			log.Printf("scan item: %v", err)
			continue
		}
		files = append(files, f)
	}	

	if err := rows.Err(); err != nil {
		sendJSON(w, false, http.StatusInternalServerError, "db iterate failed")
		return
	}

	sendJSON(w, true, http.StatusOK, files)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
  device, err := authenticate(r)
  if err != nil {
    sendJSON(w, false, http.StatusUnauthorized, "unauthorized")
    return
  }

  id := r.URL.Query().Get("id")
  if id == "" {
    sendJSON(w, false, http.StatusBadRequest, "missing id parameter")
    return
  }

  isFile := r.URL.Query().Get("isFile") == "true"
  baseDir := filepath.Join(appConfig.SaveDir, device.Name)

  if id == "all" {
    if isFile {
      entries, err := os.ReadDir(baseDir)
      if err != nil {
        sendJSON(w, false, http.StatusInternalServerError, "failed to read directory")
        return
      }
      for _, entry := range entries {
        if entry.IsDir() {
          continue
        }
        if entry.Name() == "links.json" {
          continue
        }
        if err := os.Remove(filepath.Join(baseDir, entry.Name())); err != nil {
          fmt.Printf("could not delete %s: %v\n", entry.Name(), err)
        }
      }
      os.Remove(filepath.Join(baseDir, "files.json"))
      sendJSON(w, true, http.StatusOK, "all files deleted")
      return
    } else {
      if err := os.Remove(filepath.Join(baseDir, "links.json")); err != nil && !os.IsNotExist(err) {
        sendJSON(w, false, http.StatusInternalServerError, "failed to delete links.json")
        return
      }
      sendJSON(w, true, http.StatusOK, "links deleted")
      return
    }
  }

  if isFile {
    filePath := filepath.Join(baseDir, id)
    if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
      sendJSON(w, false, http.StatusInternalServerError, "failed to delete physical file")
      return
    }

    metaPath := filepath.Join(baseDir, "files.json")
    entries, err := readFileMeta(metaPath)
    if err != nil {
      if os.IsNotExist(err) {
        sendJSON(w, false, http.StatusNotFound, "no metadata file")
      } else {
        sendJSON(w, false, http.StatusInternalServerError, "failed to read metadata")
      }
      return
    }
    newEntries, found := filterFileMeta(entries, id)
    if !found {
      sendJSON(w, false, http.StatusBadRequest, "ID not found in metadata")
      return
    }
    if err := writeFileMeta(metaPath, newEntries); err != nil {
      sendJSON(w, false, http.StatusInternalServerError, "failed to write metadata")
      return
    }
  } else {
    metaPath := filepath.Join(baseDir, "links.json")
    entries, err := readLinkMeta(metaPath)
    if err != nil {
      if os.IsNotExist(err) {
        sendJSON(w, false, http.StatusNotFound, "no links file")
      } else {
        sendJSON(w, false, http.StatusInternalServerError, "failed to read links")
      }
      return
    }
    newEntries, found := filterLinkMeta(entries, id)
    if !found {
      sendJSON(w, false, http.StatusBadRequest, "ID not found in links")
      return
    }
    if err := writeLinkMeta(metaPath, newEntries); err != nil {
      sendJSON(w, false, http.StatusInternalServerError, "failed to write links")
      return
    }
  }

  sendJSON(w, true, http.StatusOK, "deleted successfully")
}

func handleDownloadAndDelete(w http.ResponseWriter, r *http.Request) {
  device, err := authenticate(r)
  if err != nil {
    sendJSON(w, false, http.StatusUnauthorized, "unauthorized")
    return
  }

  id := r.URL.Query().Get("id")
  if id == "" {
    sendJSON(w, false, http.StatusBadRequest, "missing id")
    return
  }

  baseDir := filepath.Join(appConfig.SaveDir, device.Name)

  metaPath := filepath.Join(baseDir, "files.json")
  entries, err := readFileMeta(metaPath)
  if err != nil {
    sendJSON(w, false, http.StatusInternalServerError, "failed to read metadata")
    return
  }

  var originalName string
  found := false
  for _, entry := range entries {
    if entry.ID == id {
      originalName = entry.Name
      found = true
      break
    }
  }
  if !found {
    sendJSON(w, false, http.StatusNotFound, "file not found")
    return
  }

  filePath := filepath.Join(baseDir, id)
  file, err := os.Open(filePath)
  if err != nil {
    sendJSON(w, false, http.StatusNotFound, "file not found on disk")
    return
  }
  defer file.Close()

  w.Header().Set("Content-Disposition", "attachment; filename=\""+originalName+"\"")
  w.Header().Set("Content-Type", "application/octet-stream")
  http.ServeContent(w, r, originalName, time.Now(), file)

  os.Remove(filePath) 

  newEntries, _ := filterFileMeta(entries, id)
  writeFileMeta(metaPath, newEntries)

  fmt.Printf("File %s downloaded and deleted for %s\n", originalName, device.Name)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {
	InitDB("items.db")
	defer DB.Close()
	
	mainInit()
	

	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/auth", handleLogin)
	http.HandleFunc("/whoami", handleAuth)
	http.HandleFunc("/devices", handleDeviceNames)
	http.HandleFunc("/files", handleFileList)
	http.HandleFunc("/delete", handleDelete)
	http.HandleFunc("/download-and-delete", handleDownloadAndDelete) 
	http.ListenAndServe(":7842", nil)	
	
}