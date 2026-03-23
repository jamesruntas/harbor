package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var movieExts = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
	".m4v": true, ".wmv": true, ".flv": true, ".webm": true,
}

// indexMovies walks folderPath, inserts new movie files into the DB, and
// queues thumbnail generation for each new entry. Duplicates are skipped.
func indexMovies(folderPath string, db *sql.DB, thumb *Thumbnailer, onFile func()) error {
	return filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !movieExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}

		result, err := db.Exec(
			`INSERT OR IGNORE INTO movies (path, filename, size, modified_at) VALUES (?, ?, ?, ?)`,
			path,
			filepath.Base(path),
			info.Size(),
			info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
		)
		if err != nil {
			log.Printf("movies: db error for %s: %v", filepath.Base(path), err)
			return nil
		}

		n, _ := result.RowsAffected()
		if n == 0 {
			return nil
		}

		log.Printf("movies: indexed: %s", filepath.Base(path))
		var id int64
		db.QueryRow(`SELECT id FROM movies WHERE path = ?`, path).Scan(&id)
		thumb.generateAsync(id, path)
		if onFile != nil {
			onFile()
		}
		return nil
	})
}
