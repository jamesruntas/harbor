package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

type MediaItem struct {
	ID        int64   `json:"id"`
	Filename  string  `json:"filename"`
	DateTaken string  `json:"date_taken"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Make      string  `json:"make,omitempty"`
	Model     string  `json:"model,omitempty"`
}

// job tracks the state of the background indexing job.
// Only one job runs at a time — HomeStream is single-user.
type job struct {
	mu      sync.Mutex
	status  string // "idle" | "running" | "done" | "error"
	indexed int
	errMsg  string
}

func (j *job) start(folderPath string, db *sql.DB, thumb *Thumbnailer, broker *Broker) {
	j.mu.Lock()
	j.status = "running"
	j.indexed = 0
	j.errMsg = ""
	j.mu.Unlock()

	go func() {
		err := indexFolder(folderPath, db, thumb, func() {
			j.mu.Lock()
			j.indexed++
			j.mu.Unlock()
		})
		j.mu.Lock()
		if err != nil {
			j.status = "error"
			j.errMsg = err.Error()
		} else {
			j.status = "done"
		}
		j.mu.Unlock()
		broker.publish("index-done")
	}()
}

func registerHandlers(mux *http.ServeMux, db *sql.DB, thumb *Thumbnailer, broker *Broker) {
	j := &job{status: "idle"}

	// GET /api/media?offset=0 — paginated media list (100 per page)
	mux.HandleFunc("GET /api/media", func(w http.ResponseWriter, r *http.Request) {
		const pageSize = 100
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		var total int
		db.QueryRow(`SELECT COUNT(*) FROM media`).Scan(&total)

		rows, err := db.Query(`SELECT id, filename, date_taken, latitude, longitude, make, model
			FROM media ORDER BY date_taken DESC LIMIT ? OFFSET ?`, pageSize, offset)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var items []MediaItem
		for rows.Next() {
			var item MediaItem
			rows.Scan(&item.ID, &item.Filename, &item.DateTaken, &item.Latitude, &item.Longitude, &item.Make, &item.Model)
			items = append(items, item)
		}
		if items == nil {
			items = []MediaItem{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"items": items,
			"total": total,
		})
	})

	// POST /api/index — start a background indexing job
	mux.HandleFunc("POST /api/index", func(w http.ResponseWriter, r *http.Request) {
		path := r.FormValue("path")
		if path == "" {
			http.Error(w, "missing path", http.StatusBadRequest)
			return
		}

		j.mu.Lock()
		running := j.status == "running"
		j.mu.Unlock()

		if running {
			http.Error(w, "index already running", http.StatusConflict)
			return
		}

		j.start(path, db, thumb, broker)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"started"}`))
	})

	// GET /api/index/status — poll indexing progress
	mux.HandleFunc("GET /api/index/status", func(w http.ResponseWriter, r *http.Request) {
		j.mu.Lock()
		status := j.status
		indexed := j.indexed
		errMsg := j.errMsg
		j.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  status,
			"indexed": indexed,
			"error":   errMsg,
		})
	})

	// GET /api/events — SSE stream, pushes "new-file" and "index-done" events
	mux.HandleFunc("GET /api/events", broker.ServeSSE)

	// GET /api/stream/{id} — serve original file with range-request support (photos + videos)
	mux.HandleFunc("GET /api/stream/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var filePath string
		if err := db.QueryRow(`SELECT path FROM media WHERE id = ?`, id).Scan(&filePath); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		f, err := os.Open(filePath)
		if err != nil {
			http.Error(w, "file error", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			http.Error(w, "file error", http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, filepath.Base(filePath), fi.ModTime(), f)
	})

	// GET /api/thumbnail/{id} — serve cached thumbnail JPEG
	mux.HandleFunc("GET /api/thumbnail/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		thumb.serve(w, r, id)
	})
}
