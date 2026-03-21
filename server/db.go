package main

import (
	"database/sql"
	"log"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func initDB(path string) *sql.DB {
	abs, _ := filepath.Abs(path)
	log.Printf("database: %s", abs)

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Fatal(err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS media (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		path       TEXT UNIQUE,
		filename   TEXT,
		date_taken TEXT,
		latitude   REAL,
		longitude  REAL,
		make       TEXT,
		model      TEXT
	)`)
	return db
}
