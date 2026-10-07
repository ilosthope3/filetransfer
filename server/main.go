package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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

	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type (
	Config struct {
		TokensFile            string
		SaveDir               string
		MaxFormSize           int64
		Password              string
		AllowCrossUserDelete  bool
		CleanIntervalMins     int
		DeleteGracePeriodMins int
		ServerPort            string
		QueryIntervalSecs     int
		KeepAliveSecs         int
		DBPath                string
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
	Dir struct {
		Files   []string `json:"files"`
		DirName string   `json:"name"`
		Dirs    []Dir    `json:"dirs,omitempty"`
	}
	WsMessage struct {
		MsgType int
		Data    []byte
	}
	DownloadItem struct {
		UUID   string `json:"uuid"`
		Type   string `json:"type"`
		Header string `json:"header,omitempty"`
	}
)

var (
	appConfig    Config
	connUpgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	tlsCertPath string
	tlsKeyPath  string
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
	err := DB.QueryRow("SELECT id, username FROM users WHERE TOKEN = ?", token).Scan(&row.ID, &row.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			return row, errors.New("invalid token")
		}
		return row, errors.New("db error during auth service")

	}
	return row, nil

}

func mainInit() {
	stateDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("could not find working directory: %v\n", err)
		os.Exit(1)
	}

	configPath := filepath.Join(stateDir, "config.json")

	defaultConfig := Config{
		MaxFormSize:           500 << 20,
		SaveDir:               "data",
		DBPath:                "items.db",
		Password:              "123",
		AllowCrossUserDelete:  true,
		CleanIntervalMins:     60,
		DeleteGracePeriodMins: 1,
		ServerPort:            ":7842",
		QueryIntervalSecs:     5,
		KeepAliveSecs:         30,
	}

	data, err := os.ReadFile(configPath)

	if errors.Is(err, os.ErrNotExist) {
		appConfig = defaultConfig

		data, err := json.MarshalIndent(appConfig, "", "    ")
		if err != nil {
			fmt.Printf("could not create default config: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(configPath, data, 0o644); err != nil {
			fmt.Printf("could not write default config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("created default config: %s\n", configPath)

	} else if err != nil {
		fmt.Printf("could not read config %q: %v\n", configPath, err)
		os.Exit(1)

	} else if err := json.Unmarshal(data, &appConfig); err != nil {
		fmt.Printf(
			"invalid config %q: %v\nPlease fix the config file and restart the server.\n",
			configPath,
			err,
		)
		os.Exit(1)
	}

	if !filepath.IsAbs(appConfig.SaveDir) {
		appConfig.SaveDir = filepath.Join(stateDir, appConfig.SaveDir)
	}

	if !filepath.IsAbs(appConfig.DBPath) {
		appConfig.DBPath = filepath.Join(stateDir, appConfig.DBPath)
	}

	if err := os.MkdirAll(appConfig.SaveDir, 0o755); err != nil {
		fmt.Printf("create save dir: %v\n", err)
		os.Exit(1)
	}

	tlsKeyPath = filepath.Join(stateDir, "tls.key")
	tlsCertPath = filepath.Join(stateDir, "tls.crt")

	if _, err := os.Stat(tlsCertPath); errors.Is(err, os.ErrNotExist) {
		key, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			fmt.Printf("could not generate TLS key: %v\n", err)
			os.Exit(1)
		}

		serial, err := rand.Int(
			rand.Reader,
			new(big.Int).Lsh(big.NewInt(1), 128),
		)
		if err != nil {
			fmt.Printf("could not generate TLS certificate serial: %v\n", err)
			os.Exit(1)
		}

		template := x509.Certificate{
			SerialNumber: serial,

			Subject: pkix.Name{
				CommonName: "Silkwrap Server",
			},

			NotBefore: time.Now(),
			NotAfter:  time.Now().AddDate(10, 0, 0),

			KeyUsage: x509.KeyUsageDigitalSignature |
				x509.KeyUsageKeyEncipherment,

			ExtKeyUsage: []x509.ExtKeyUsage{
				x509.ExtKeyUsageServerAuth,
			},

			BasicConstraintsValid: true,
		}

		certDER, err := x509.CreateCertificate(
			rand.Reader,
			&template,
			&template,
			&key.PublicKey,
			key,
		)
		if err != nil {
			fmt.Printf("could not create TLS certificate: %v\n", err)
			os.Exit(1)
		}

		keyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})

		certPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certDER,
		})

		if err := os.WriteFile(tlsKeyPath, keyPEM, 0o600); err != nil {
			fmt.Printf("could not write TLS key: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(tlsCertPath, certPEM, 0o644); err != nil {
			fmt.Printf("could not write TLS certificate: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("created TLS certificate: %s\n", tlsKeyPath)
		fmt.Printf("created TLS key: %s\n", tlsKeyPath)
	}

	certPEM, err := os.ReadFile(tlsCertPath)
	if err != nil {
		fmt.Printf("could not read TLS certificate: %v\n", err)
		os.Exit(1)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		fmt.Println("could not decode TLS certificate")
		os.Exit(1)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		fmt.Printf("could not parse TLS certificate: %v\n", err)
		os.Exit(1)
	}

	fingerprint := sha256.Sum256(cert.Raw)

	fmt.Printf("TLS certificate fingerprint:\nSHA256:%X\n", fingerprint)
}

func cleanUpScheduler(ctx context.Context) {
	err := cleanUp()
	if err != nil {
		log.Printf("Cleanup error (startup): %v", err)
	}
	t := time.NewTicker(time.Duration(appConfig.CleanIntervalMins) * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("Cleanup scheduler goroutine done")
			return
		case <-t.C:
			err := cleanUp()
			if err != nil {
				log.Printf("Cleanup error %v", err)
			}
			log.Println("Cleanup complete")
		}
	}
}

func cleanUp() error {
	removeOrphanBlobs := func() (int, error) {
		entries, err := os.ReadDir(appConfig.SaveDir)
		if err != nil {
			return 0, err
		}
		cutoff := time.Now().Add(-time.Duration(appConfig.DeleteGracePeriodMins) * time.Minute)
		rows, err := DB.Query(`
			SELECT uuid
			FROM items
			WHERE (type = 'dir-child'
							AND ( (consumed = TRUE AND consumed_at >= ?)
								OR  (consumed = FALSE AND is_child = TRUE)
								OR  (consumed = FALSE AND is_child = FALSE AND uploaded_at >= ?)
							))
				OR  (type != 'dir-child'
							AND (  consumed = FALSE
								OR   consumed_at >= ?
							))
		`, cutoff, cutoff, cutoff)
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
		return err
	}

	n, _ := r.RowsAffected()
	log.Printf("Cleanup: rows=%d orphans=%d\n", n, orphans)
	return nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////

func handleWhoAmI(w http.ResponseWriter, r *http.Request) {
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
		sendJSON(w, false, http.StatusBadRequest, "Files too large")
		return
	}

	link := r.FormValue("text")
	var receivers []string
	if err := json.Unmarshal([]byte(r.FormValue("receivers")), &receivers); err != nil {
		sendJSON(w, false, http.StatusInternalServerError, "error decoding receivers from json")
		return
	}
	if len(receivers) == 0 {
		sendJSON(w, false, http.StatusBadRequest, "Invalid receiver")
		return
	}

	for i := range receivers {
		if receivers[i] == sender.Name {
			receivers = append(receivers[:i], receivers[i+1:]...)
			break
		}
	}

	type Rec struct {
		Name    string `json:"name"`
		Error   string `json:"error"`
		ID      int64  `json:"id"`
		Errored bool   `json:"errored"`
	}

	recs := []Rec{}
	for _, receiver := range receivers {

		if receiver == "" || receiver == "." || receiver == ".." {
			recs = append(recs, Rec{Name: receiver, Error: "invalid receiver format", Errored: true, ID: 0})
			continue
		}

		var receiver_id int64
		if err = DB.QueryRow("SELECT id FROM users WHERE username = ? ", receiver).Scan(&receiver_id); err != nil {
			if err == sql.ErrNoRows {
				recs = append(recs, Rec{Name: receiver, Error: "no such receiver", Errored: true, ID: 0})
				continue
			}
			recs = append(recs, Rec{Name: receiver, Error: "db error", Errored: true, ID: 0})
			continue
		}
		recs = append(recs, Rec{Name: receiver, Error: "", Errored: false, ID: receiver_id})
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
	inputType := r.Header.Get("InputType")

	if !slices.Contains([]string{"file", "dir-child", "dir-struct", "link"}, inputType) {
		sendJSON(w, false, http.StatusInternalServerError, "inputType header wrong")
		return
	}

	fileUUID := uuid.New().String()
	if inputType == "link" {

		for i := range recs {
			if recs[i].Errored {
				continue
			}
			_, err = DB.Exec("INSERT INTO items (uuid, sender_id, receiver_id, type, url, uploaded_at, consumed, is_child) VALUES (?, ?, ?, ?, ?, ?,?,?);",
				fileUUID,
				sender.ID,
				recs[i].ID,
				inputType,
				link,
				time.Now().Format("2006-01-02 15:04:57"),
				false,
				false,
			)
			if err != nil {
				recs[i].Errored = true
				recs[i].Error = "error executing db insert (link)"
				continue
			}

			log.Printf("LINK SAVED: %s\n", link)
		}

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

		var gatherUUIDs func(Dir) []string
		gatherUUIDs = func(curr Dir) []string {
			var r = []string{}
			for _, d := range curr.Dirs {
				r = append(r, gatherUUIDs(d)...)
			}
			return append(r, curr.Files...)
		}

		for i := range recs {
			if recs[i].Errored {
				continue
			}

			tx, err := DB.Begin()
			if err != nil {
				recs[i].Errored = true
				recs[i].Error = fmt.Sprintf("error starting db transaction: %v", err)
				continue
			}

			_, err = tx.Exec(`
						INSERT INTO items (
								uuid,
								sender_id,
								receiver_id,
								type,
								filename,
								size,
								uploaded_at,
								consumed,
								is_child
						) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
				`,
				fileUUID,
				sender.ID,
				recs[i].ID,
				inputType,
				filepath.Base(fileHeader.Filename),
				fileHeader.Size,
				time.Now().Format("2006-01-02 15:04:57"),
				false,
				false,
			)
			if err != nil {
				tx.Rollback()
				recs[i].Errored = true
				recs[i].Error = "error executing db insert (file)"
				continue
			}

			if inputType == "dir-struct" {
				file.Close()

				file, err = os.Open(filepath.Join(appConfig.SaveDir, fileUUID))
				if err != nil {
					tx.Rollback()
					recs[i].Errored = true
					recs[i].Error = "failed to reopen manifest"
					continue
				}

				var root Dir
				if err := json.NewDecoder(file).Decode(&root); err != nil {
					file.Close()
					tx.Rollback()
					recs[i].Errored = true
					recs[i].Error = "invalid directory manifest"
					continue
				}
				file.Close()

				uuids := gatherUUIDs(root)

				args := make([]any, len(uuids)+1)
				placeholders := make([]string, len(uuids))

				for j, uuid := range uuids {
					placeholders[j] = "?"
					args[j] = uuid
				}

				args[len(uuids)] = recs[i].ID

				query := fmt.Sprintf(`
								UPDATE items
								SET is_child = TRUE
								WHERE uuid IN (%s)
								AND receiver_id = ?
						`, strings.Join(placeholders, ","))

				if _, err := tx.Exec(query, args...); err != nil {
					tx.Rollback()
					recs[i].Errored = true
					recs[i].Error = fmt.Sprintf("error accounting directory children: %v", err)
					continue
				}
			}

			if err := tx.Commit(); err != nil {
				recs[i].Errored = true
				recs[i].Error = fmt.Sprintf("error committing db transaction: %v", err)
				continue
			}
		}
	} else {
		sendJSON(w, false, http.StatusInternalServerError, "unknown error when saving couldnt define input type")
		return
	}

	type UploadResp struct {
		UUID      string `json:"uuid"`
		Receivers []Rec  `json:"receivers"`
	}

	sendJSON(w, true, http.StatusOK, UploadResp{UUID: fileUUID, Receivers: recs})
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
		Name:  device.Name,
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
		`SELECT id, uuid, sender_id, receiver_id, type, filename, size, url, uploaded_at, consumed, consumed_at, is_child
		FROM items
		WHERE receiver_id = ? AND consumed = FALSE AND NOT(type = 'dir-child')`,
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
			&f.IsChild,
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

	uuid := r.URL.Query().Get("uuid")
	if uuid == "" {
		sendJSON(w, false, http.StatusBadRequest, "missing id parameter")
		return
	}

	var res sql.Result
	if uuid == "all" {
		res, err = DB.Exec(
			`UPDATE items SET consumed = TRUE, consumed_at = ? WHERE receiver_id = ? AND consumed = FALSE`,
			time.Now(), device.ID,
		)
	} else {
		res, err = DB.Exec(
			`UPDATE items SET consumed = TRUE, consumed_at = ? WHERE receiver_id = ? AND uuid = ? AND consumed = FALSE`,
			time.Now(), device.ID, uuid,
		)
	}
	if err != nil {
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

	uuid := r.URL.Query().Get("uuid")
	if uuid == "" {
		sendJSON(w, false, http.StatusBadRequest, "missing id")
		return
	}

	var f FileRecord
	err = DB.QueryRow(
		`SELECT id, uuid, sender_id, receiver_id, type, filename, size, url,
			uploaded_at, consumed, consumed_at, is_child
    FROM items
    WHERE receiver_id = ?
      AND uuid = ?
      AND consumed = FALSE
      AND (NOT(type = 'dir-child') OR is_child)`,
		device.ID, uuid,
	).Scan(&f.ID, &f.UUID, &f.SenderID, &f.ReceiverID, &f.Type,
		&f.Filename, &f.Size, &f.URL, &f.UploadedAt, &f.Consumed, &f.ConsumedAt, &f.IsChild)
	if err != nil {
		if err == sql.ErrNoRows {
			sendJSON(w, false, http.StatusBadRequest, "no such id found")
			return
		}
		sendJSON(w, false, http.StatusInternalServerError, "db error idgaf")
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
}

func handleWebsocket(w http.ResponseWriter, r *http.Request) {

	device, err := authenticate(r)
	if err != nil {
		sendJSON(w, false, http.StatusUnauthorized, "Unauthorised")
		return
	}

	conn, err := connUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	defer conn.Close()

	log.Println("client connected")

	conn.SetReadDeadline(time.Now().Add(2 * time.Duration(appConfig.KeepAliveSecs) * time.Second))

	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(2 * time.Duration(appConfig.KeepAliveSecs) * time.Second))
		return nil
	})

	tickerKeepAlive := time.NewTicker(time.Duration(appConfig.KeepAliveSecs) * time.Second)
	defer tickerKeepAlive.Stop()

	tickerInbox := time.NewTicker(time.Duration(appConfig.QueryIntervalSecs) * time.Second)
	defer tickerInbox.Stop()

	messageQueue := make(chan WsMessage)

	done := make(chan struct{})
	defer close(done)

	// goroutine for ping pong pings
	go func() {
		for {
			select {
			case <-tickerKeepAlive.C:
				select {
				case messageQueue <- WsMessage{
					MsgType: websocket.PingMessage,
					Data:    nil,
				}:
				case <-done:
					return
				}

			case <-done:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-tickerInbox.C:
				var items []DownloadItem

				rows, err := DB.Query(`
					SELECT uuid, type
					FROM items
					WHERE receiver_id = ?
					AND consumed = FALSE
					AND NOT(type = 'dir-child' OR type = 'link')
				`, device.ID)

				if err != nil {
					log.Printf("goroutine db query error: %v\n", err)
					continue
				}

				for rows.Next() {
					var item DownloadItem

					if err := rows.Scan(&item.UUID, &item.Type); err != nil {
						log.Printf("goroutine db scan error: %v\n", err)
						continue
					}

					items = append(items, item)
				}

				if err := rows.Err(); err != nil {
					log.Printf("goroutine db rows error: %v\n", err)
					rows.Close()
					continue
				}

				rows.Close()

				if len(items) != 0 {
					data, err := json.Marshal(items)
					if err != nil {
						log.Printf("goroutine json marshal error: %v\n", err)
						continue
					}

					select {
					case messageQueue <- WsMessage{
						MsgType: websocket.TextMessage,
						Data:    data,
					}:
						fmt.Println("alerting client of new files")
					case <-done:
						return
					}
				}

			case <-done:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case msg := <-messageQueue:
				if err := conn.WriteMessage(msg.MsgType, msg.Data); err != nil {
					log.Printf("sending goroutine errored: %v\n", err)
					return
				}

			case <-done:
				return
			}
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("client disconnected:", err)
			return
		}

		log.Println("received:", string(msg))
	}
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {

	mainInit()
	InitDB(appConfig.DBPath)
	defer DB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go cleanUpScheduler(ctx)

	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/auth", handleLogin)
	http.HandleFunc("/whoami", handleWhoAmI)
	http.HandleFunc("/devices", handleDeviceNames)
	http.HandleFunc("/files", handleFileList)
	http.HandleFunc("/delete", handleDelete)
	http.HandleFunc("/download", handleDownload)
	http.HandleFunc("/delete-user", handleDeleteUser)
	http.HandleFunc("/ws", handleWebsocket)

	server := &http.Server{
		Addr: appConfig.ServerPort,
	}

	go func() {
		<-ctx.Done()

		log.Println("SHUTTING DOWN")
		server.Shutdown(context.Background())
	}()

	err := server.ListenAndServeTLS(tlsCertPath, tlsKeyPath)
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
