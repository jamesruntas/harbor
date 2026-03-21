package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	exiftool "github.com/barasher/go-exiftool"
)

func indexFolder(folderPath string, db *sql.DB, onFile func()) error {
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

		switch filepath.Ext(path) {
		case ".jpg", ".jpeg", ".png", ".heic", ".mp4", ".mov":
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
				if n, ok := v.(float64); ok {
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
			onFile()
		}
		return nil
	})
}
