package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

type FileRecord struct {
	ID         int64  `json:"id"`
	UUID       string `json:"uuid"`
	SenderID   int64  `json:"sender_id"`
	ReceiverID int64  `json:"receiver_id"`
	Filename   *string `json:"filename,omitempty"`
	Size       *int64  `json:"size,omitempty"`
	UploadedAt string `json:"uploaded_at"`
	Consumed   bool   `json:"consumed"`
	ConsumedAt *string `json:"consumed_at"`
	Type       string `json:"type"`
	URL        *string `json:"url,omitempty"`
}

type UserRecord struct {
	ID    int64  `json:"id"`
	Token string `json:"token"`
	Name  string `json:"name"`
}

var DB *sql.DB

func InitDB(path string) {
	var err error
	DB, err = sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal("Failed to open DB:", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id       INTEGER PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		token    TEXT UNIQUE NOT NULL
	);

	CREATE TABLE IF NOT EXISTS items (
    id             INTEGER PRIMARY KEY,
    uuid           TEXT UNIQUE NOT NULL,
    sender_id      INTEGER NOT NULL REFERENCES users(id),
    receiver_id    INTEGER NOT NULL REFERENCES users(id),
    type           TEXT NOT NULL CHECK (type IN ('file', 'link')),
    filename       TEXT,
    size           INTEGER, 
    url            TEXT, 
    uploaded_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    consumed       BOOLEAN DEFAULT FALSE,
    consumed_at    DATETIME
	);`

	if _, err := DB.Exec(schema); err != nil {
		log.Fatal("Failed to create tables:", err)
	}

	log.Println("Database initialized")
}
