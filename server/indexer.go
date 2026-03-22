package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	exiftool "github.com/barasher/go-exiftool"
)

var supportedExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true,
	".heic": true, ".mp4": true, ".mov": true,
}

// indexFile extracts metadata for a single file and writes it to the DB.
// Returns true if a new row was inserted (false if skipped as duplicate).
// Shared by the bulk indexer (indexFolder) and the file watcher.
func indexFile(path string, et *exiftool.Exiftool, db *sql.DB, thumb *Thumbnailer) bool {
	if !supportedExts[strings.ToLower(filepath.Ext(path))] {
		return false
	}

	var getString func(string) string
	var getFloat func(string) float64

	results := et.ExtractMetadata(path)
	if len(results) == 0 || results[0].Err != nil {
		log.Printf("exiftool error for %s: %v — inserting with empty metadata", filepath.Base(path), results[0].Err)
		getString = func(string) string { return "" }
		getFloat = func(string) float64 { return 0 }
	} else {
		m := results[0]
		getString = func(key string) string {
			if v, ok := m.Fields[key]; ok {
				return fmt.Sprintf("%v", v)
			}
			return ""
		}
		getFloat = func(key string) float64 {
			if v, ok := m.Fields[key]; ok {
				if n, ok := v.(float64); ok {
					return n
				}
			}
			return 0
		}
	}

	// Videos use CreateDate; photos use DateTimeOriginal. Check both.
	dateTaken := getString("DateTimeOriginal")
	if dateTaken == "" {
		dateTaken = getString("CreateDate")
	}

	result, err := db.Exec(`INSERT OR IGNORE INTO media
		(path, filename, date_taken, latitude, longitude, make, model)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		path,
		filepath.Base(path),
		dateTaken,
		getFloat("GPSLatitude"),
		getFloat("GPSLongitude"),
		getString("Make"),
		getString("Model"),
	)
	if err != nil {
		log.Printf("db insert error for %s: %v", filepath.Base(path), err)
		return false
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		log.Printf("skipped (already in db): %s", filepath.Base(path))
		return false
	}

	log.Printf("indexed: %s", filepath.Base(path))
	var id int64
	db.QueryRow(`SELECT id FROM media WHERE path = ?`, path).Scan(&id)
	thumb.generateAsync(id, path)
	return true
}

func indexFolder(folderPath string, db *sql.DB, thumb *Thumbnailer, onFile func()) error {
	// TODO: replace hardcoded path with runtime-relative path once installer is built
	// final form: filepath.Join(filepath.Dir(os.Executable()), "tools", "exiftool.exe")
	et, err := exiftool.NewExiftool(exiftool.SetExiftoolBinaryPath(`C:\Users\James\HarborTools\exiftool.exe`))
	if err != nil {
		return fmt.Errorf("exiftool init failed: %w", err)
	}
	defer et.Close()

	return filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if indexFile(path, et, db, thumb) {
			onFile()
		}
		return nil
	})
}
