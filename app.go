package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	exiftool "github.com/barasher/go-exiftool"
	_ "github.com/mattn/go-sqlite3"
)
func initDB(path string) *sql.DB {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        log.Fatal(err)
    }
    db.Exec(`CREATE TABLE IF NOT EXISTS media (
        id        INTEGER PRIMARY KEY AUTOINCREMENT,
        path      TEXT UNIQUE,
        filename  TEXT,
        date_taken TEXT,
        latitude  REAL,
        longitude REAL,
        make      TEXT,
        model     TEXT
    )`)
    return db
}

func indexFolder(folderPath string, db *sql.DB) error {
    et, err := exiftool.NewExiftool()
    if err != nil {
        return fmt.Errorf("exiftool init failed: %w", err)
    }
    defer et.Close()

    return filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return err
        }

        ext := filepath.Ext(path)
        switch ext {
        case ".jpg", ".jpeg", ".png", ".heic", ".mp4", ".mov":
            // continue
        default:
            return nil
        }

        results := et.ExtractMetadata(path)
        if len(results) == 0 || results[0].Err != nil {
            return nil
        }
        m := results[0]

        getString := func(key string) string {
            if v, ok := m.Fields[key]; ok {
                return fmt.Sprintf("%v", v)
            }
            return ""
        }
        getFloat := func(key string) float64 {
            if v, ok := m.Fields[key]; ok {
                switch n := v.(type) {
                case float64:
                    return n
                }
            }
            return 0
        }

        _, err = db.Exec(`INSERT OR IGNORE INTO media
            (path, filename, date_taken, latitude, longitude, make, model)
            VALUES (?, ?, ?, ?, ?, ?, ?)`,
            path,
            filepath.Base(path),
            getString("DateTimeOriginal"),
            getFloat("GPSLatitude"),
            getFloat("GPSLongitude"),
            getString("Make"),
            getString("Model"),
        )
        if err != nil {
            log.Printf("db insert error for %s: %v", path, err)
        } else {
            log.Printf("indexed: %s", filepath.Base(path))
        }
        return nil
    })
}

type App struct {
    ctx context.Context
    db  *sql.DB
}

func (a *App) startup(ctx context.Context) {
    a.ctx = ctx
}

func (a *App) IndexFolder(path string) string {
    err := indexFolder(path, a.db)
    if err != nil {
        return fmt.Sprintf("Error: %v", err)
    }
    return "Done"
}