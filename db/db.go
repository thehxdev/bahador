package db

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"runtime"

	_ "github.com/glebarez/go-sqlite"
)

type DB struct {
	Write *sql.DB
	Read  *sql.DB
	Log   *log.Logger
}

// const dbSchema = `CREATE TABLE users (
//     user_id BIGINT PRIMARY KEY,
//     first_name TEXT,
//     username TEXT
// );
//
// CREATE TABLE messages (
//     message_id BIGINT PRIMARY KEY,
//     date UNSIGNED BIGINT NOT NULL,
//     user_id BIGINT NOT NULL,
//     chat_id BIGINT NOT NULL,
//     FOREIGN KEY(user_id) REFERENCES users(user_id)
// );
//
// CREATE TABLE files (
//     id INTEGER PRIMARY KEY,
//     file_id TEXT NOT NULL,
//     file_unique_id TEXT UNIQUE NOT NULL,
//     file_name TEXT NOT NULL,
//     file_size BIGINT NOT NULL,
//     message_id BIGINT NOT NULL,
//     user_id BIGINT NOT NULL,
//     FOREIGN KEY(message_id) REFERENCES messages(message_id),
//     FOREIGN KEY(user_id) REFERENCES users(user_id)
// );`

const driverName string = "sqlite"

func New(path string) (*DB, error) {
	connUrlParams := &url.Values{}
	connUrlParams.Add("_txlock", "immediate")
	connUrlParams.Add("_journal_mode", "WAL")
	connUrlParams.Add("_busy_timeout", "5000")
	connUrlParams.Add("_synchronous", "NORMAL")
	// connUrlParams.Add("_cache_size", "1000000000")
	connUrlParams.Add("_foreign_keys", "true")
	connUrl := fmt.Sprintf("file:%s?%s", path, connUrlParams.Encode())

	writeDB, err := sql.Open(driverName, connUrl)
	if err != nil {
		return nil, err
	}
	writeDB.SetMaxOpenConns(1)

	readDB, err := sql.Open(driverName, connUrl)
	if err != nil {
		return nil, err
	}
	readDB.SetMaxOpenConns(max(1, runtime.NumCPU()))

	db := &DB{
		Write: writeDB,
		Read:  readDB,
		Log:   log.New(os.Stderr, "[db] ", log.Ldate|log.Lshortfile),
	}
	return db, nil
}

func (db *DB) Setup(schemaPath string) error {
	file, err := os.Open(schemaPath)
	if err != nil {
		return err
	}
	defer file.Close()
	dbSchema, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	_, err = db.Write.Exec(string(dbSchema))
	return err
}

func (db *DB) Close() error {
	err := db.Read.Close()
	if err != nil {
		return err
	}
	return db.Write.Close()
}
