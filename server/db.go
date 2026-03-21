package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func initDB(path string) *sql.DB {
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
