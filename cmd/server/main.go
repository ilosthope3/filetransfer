package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
)

type (
	Config struct {
		TokensFile           string
		SaveDir              string
		MaxFormSize          int64
		Password             string
		AllowCrossUserDelete bool
		CleanIntervalMins    int
	}
	APIResponse struct {
		Success bool        `json:"success"`
		Data    interface{} `json:"data,omitempty"`
		Error   string      `json:"error,omitempty"`
	}
	FileMeta struct {
		ID     string `json:"id"`
		Size   int64  `json:"size,omitempty"`
		Time   string `json:"timestamp,omitempty"`
		Name   string `json:"filename,omitempty"`
		Sender string `json:"sender,omitempty"`
	}
	LinkMeta struct {
		ID     string `json:"id"`
		Time   string `json:"timestamp,omitempty"`
		Url    string `json:"url,omitempty"`
		Sender string `json:"sender,omitempty"`
	}
	FullReturn struct {
		Files []FileMeta `json:"files"`
		Links []LinkMeta `json:"links"`
	}
)

var (
	appConfig Config
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////

func sendJSON(w http.ResponseWriter, success bool, code int, msg interface{}) {
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

func mainInit() {

	appConfig.MaxFormSize = 50 << 40
	appConfig.SaveDir = "data"
	appConfig.Password = "123"
	appConfig.AllowCrossUserDelete = true
	appConfig.CleanIntervalMins = 30

	if err := os.MkdirAll(appConfig.SaveDir, 0o755); err != nil {
		log.Fatalf("create save dir: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go cleanUpScheduler(ctx)

}

func cleanUpScheduler(ctx context.Context) {
	cleanUp()
	t := time.NewTicker(time.Duration(appConfig.CleanIntervalMins) * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			err := cleanUp()
			if err != nil {
				fmt.Printf("idk ill cahnge to logs later and figure it out, %v", err)
			}
		}
	}
}

func cleanUp() error {
	removeOrphanBlobs := func() (int, error) {
		entries, err := os.ReadDir(appConfig.SaveDir)
		if err != nil {
			return 0, err
		}

		rows, err := DB.Query(`
			SELECT uuid FROM items
			WHERE consumed = FALSE
				OR (consumed = TRUE AND consumed_at >= ?)
		`, time.Now().Add(-24*time.Hour))
		if err != nil && err != sql.ErrNoRows {
			return 0, err
		}
		defer rows.Close()

		keep := map[string]struct{}{}
		for rows.Next() {
			var u string
			if err := rows.Scan(&u); err != nil {
				continue
			}
			keep[u] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			return 0, err
		}

		removed := 0
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if _, ok := keep[e.Name()]; ok {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if time.Since(info.ModTime()) < 10*time.Minute {
				continue
			}
			p := filepath.Join(appConfig.SaveDir, e.Name())
			if err := os.Remove(p); err != nil {
				fmt.Printf("orphan remove %s: %v\n", p, err)
				continue
			}
			removed++
		}
		return removed, nil
	}

	r, err := DB.Exec(`
		DELETE FROM items
		WHERE receiver_id NOT IN (SELECT id FROM users)
	`)
	if err != nil {
		return err
	}

	orphans, err := removeOrphanBlobs()
	if err != nil {
		fmt.Println("orphan sweep:", err)
		return err
	}

	n, _ := r.RowsAffected()
	fmt.Printf("cleanup: rows=%d orphans=%d\n", n, orphans)
	return nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////

func handleWhoAmI(w http.ResponseWriter, r *http.Request) {
	fmt.Println("whoami")

	sender, err := authenticate(r)
	if err != nil {
		sendJSON(w, false, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sendJSON(w, true, http.StatusOK, sender)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct{ Password, Name string }
	var row struct{ Username, Token string }
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

	if req.Password != appConfig.Password {
		sendJSON(w, false, http.StatusUnauthorized, "Wrong password")
		return
	}

	err = DB.QueryRow("SELECT username, token FROM users WHERE username = ?", req.Name).Scan(&row.Username, &row.Token)

	if err == sql.ErrNoRows {

		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Fatal("rand failed:", err)
		}
		token := fmt.Sprintf("token-%s", hex.EncodeToString(b))

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
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	sender, err := authenticate(r)
	if err != nil {
		sendJSON(w, false, http.StatusUnauthorized, "Unauthorized")
		return
	}

	err = r.ParseMultipartForm(appConfig.MaxFormSize)
	if err != nil {
		fmt.Printf("form parse error %v\n", err)
		sendJSON(w, false, http.StatusBadRequest, "Files too large")
		return
	}

	link := r.FormValue("text")
	// fmt.Println(link)
	receiver := r.FormValue("receiver")

	if receiver == "" || receiver == "." || receiver == ".." {
		sendJSON(w, false, http.StatusBadRequest, "Invalid receiver")
		return
	}

	var receiver_id int64
	if err = DB.QueryRow("SELECT id FROM users WHERE username = ? ", receiver).Scan(&receiver_id); err != nil {
		if err == sql.ErrNoRows {
			sendJSON(w, false, http.StatusBadRequest, "No such receiver")
			return
		}
		sendJSON(w, false, http.StatusInternalServerError, "db error")
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 && len(link) == 0 {
		sendJSON(w, false, http.StatusBadRequest, "No files selected")
		return
	}

	if len(files) > 1 {
		sendJSON(w, false, http.StatusBadRequest, "1 file at a time, or use send dir")
		return
	}
	typeDir := r.Header.Get("Directory")

	if !slices.Contains([]string{"file", "dir-child", "dir-manifest"}, typeDir) {
		sendJSON(w, false, http.StatusInternalServerError, "error executing db insert")
		return
	}

	fileUUID := uuid.New().String()
	if link != "" {
		_, err = DB.Exec("INSERT INTO items (uuid, sender_id, receiver_id, type, url, uploaded_at, consumed, is_dir) VALUES (?, ?, ?, ?, ?, ?,?,?);",
			fileUUID,
			sender.ID,
			receiver_id,
			"link",
			link,
			time.Now().Format("2006-01-02 15:04"),
			false,
			typeDir,
		)
		if err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "error executing db insert")
			return
		}

		fmt.Printf("LINK SAVED: %s\n", link)

	} else if len(files) != 0 {

		fileHeader := files[0]

		file, err := fileHeader.Open()
		if err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "Failed to open file")
			return
		}
		defer file.Close()

		dst, err := os.Create(filepath.Join(appConfig.SaveDir, fileUUID))
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

		_, err = DB.Exec("INSERT INTO items (uuid, sender_id, receiver_id, type, filename,size, uploaded_at, consumed, is_dir) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);",
			fileUUID,
			sender.ID,
			receiver_id,
			"file",
			filepath.Base(fileHeader.Filename),
			fileHeader.Size,
			time.Now().Format("2006-01-02 15:04"),
			false,
			typeDir,
		)
		if err != nil {
			sendJSON(w, false, http.StatusInternalServerError, "error executing db insert")
			return
		}

		fmt.Printf("file saved")
	}

	sendJSON(w, true, http.StatusOK, fileUUID)
}

func handleDeviceNames(w http.ResponseWriter, r *http.Request) {
	device, err := authenticate(r)
	if err != nil {
		sendJSON(w, false, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var users []UserRecord

	users = append(users, UserRecord{
		ID:    device.ID,
		Token: "",
		Name:  fmt.Sprintf("%s  <-- current user", device.Name),
	})
	rows, err := DB.Query(
		`SELECT id, username
		FROM users
		WHERE id != ?`,
		device.ID,
	)
	if err != nil {
		sendJSON(w, false, http.StatusInternalServerError, "error when searching db")
		return
	}
	defer rows.Close()

	for rows.Next() {

		var u UserRecord
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
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
		`SELECT id, uuid, sender_id, receiver_id, type, filename, size, url, uploaded_at, consumed, consumed_at, is_dir
		FROM items
		WHERE receiver_id = ? AND consumed = FALSE AND NOT(is_dir = "dir-child")`,
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
			&f.DirType,
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

	var res sql.Result
	if id == "all" {
		res, err = DB.Exec(
			`UPDATE items SET consumed = TRUE, consumed_at = ? WHERE receiver_id = ? AND consumed = FALSE`,
			time.Now(), device.ID,
		)
	} else {
		res, err = DB.Exec(
			`UPDATE items SET consumed = TRUE, consumed_at = ? WHERE receiver_id = ? AND id = ? AND consumed = FALSE`,
			time.Now(), device.ID, id,
		)
	}
	if err != nil {
		fmt.Println("delete update failed:", err)
		sendJSON(w, false, http.StatusInternalServerError, "db update failed")
		return
	}

	n, _ := res.RowsAffected()
	sendJSON(w, true, http.StatusOK, fmt.Sprintf("marked %d item(s) consumed", n))
}

func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	device, err := authenticate(r)
	if err != nil {
		sendJSON(w, false, http.StatusUnauthorized, "unauthorized")
		return
	}

	s := r.URL.Query().Get("id")
	id, err := strconv.Atoi(s)
	if err != nil {
		sendJSON(w, false, http.StatusBadRequest, "missing/invalid id parameter")
		return
	}

	if (!appConfig.AllowCrossUserDelete) && (int64(id) != device.ID) {
		sendJSON(w, false, http.StatusBadRequest, "cross user delete disabled, input nd self ids do not match")
		return
	}

	res, err := DB.Exec(`DELETE FROM users WHERE id = ?`, id)

	if err != nil {
		fmt.Println("delete update failed:", err)
		sendJSON(w, false, http.StatusInternalServerError, "deleting user failed")
		return
	}

	n, _ := res.RowsAffected()
	if n == 0 {
		sendJSON(w, false, http.StatusBadRequest, fmt.Sprintf("no user with id %v", id))
		return
	}
	sendJSON(w, true, http.StatusOK, fmt.Sprintf("deleted user %v", id))

}

func handleDownload(w http.ResponseWriter, r *http.Request) {
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

	var f FileRecord
	err = DB.QueryRow(
		`SELECT id, uuid, sender_id, receiver_id, type, filename, size, url, uploaded_at, consumed, consumed_at, is_dir
     FROM items
     WHERE receiver_id = ? AND consumed = FALSE AND id = ? AND is_dir`,
		device.ID, id,
	).Scan(&f.ID, &f.UUID, &f.SenderID, &f.ReceiverID, &f.Type,
		&f.Filename, &f.Size, &f.URL, &f.UploadedAt, &f.Consumed, &f.ConsumedAt, &f.DirType)
	if err != nil {
		if err == sql.ErrNoRows {
			sendJSON(w, false, http.StatusBadRequest, "no such id found")
			return
		}
		sendJSON(w, false, http.StatusInternalServerError, "db error during euth service")
		return
	}

	if f.Type == "link" {
		sendJSON(w, true, http.StatusOK, f.URL)
		return
	}

	filePath := filepath.Join(appConfig.SaveDir, f.UUID)
	file, err := os.Open(filePath)
	if err != nil {
		sendJSON(w, false, http.StatusNotFound, "file not found on disk")
		return
	}
	defer file.Close()

	filenameDeref := f.UUID
	if f.Filename != nil {
		filenameDeref = *f.Filename
	}

	safe := strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(filenameDeref)
	w.Header().Set("Content-Disposition", `attachment; filename="`+safe+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, filenameDeref, time.Now(), file)
	fmt.Printf("File %s downloaded for %s\n", filenameDeref, device.Name)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {
	InitDB("items.db")
	defer DB.Close()

	mainInit()

	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/auth", handleLogin)
	http.HandleFunc("/whoami", handleWhoAmI)
	http.HandleFunc("/devices", handleDeviceNames)
	http.HandleFunc("/files", handleFileList)
	http.HandleFunc("/delete", handleDelete)
	http.HandleFunc("/download", handleDownload)
	http.HandleFunc("/delete-user", handleDeleteUser)
	http.ListenAndServe(":7842", nil)
}
