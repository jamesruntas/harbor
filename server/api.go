package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
)

type MediaItem struct {
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

func (j *job) start(folderPath string, db *sql.DB) {
	j.mu.Lock()
	j.status = "running"
	j.indexed = 0
	j.errMsg = ""
	j.mu.Unlock()

	go func() {
		err := indexFolder(folderPath, db, func() {
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
	}()
}

func registerHandlers(mux *http.ServeMux, db *sql.DB) {
	j := &job{status: "idle"}

	// GET /api/media?offset=0 — paginated media list (100 per page)
	mux.HandleFunc("GET /api/media", func(w http.ResponseWriter, r *http.Request) {
		const pageSize = 100
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		var total int
		db.QueryRow(`SELECT COUNT(*) FROM media`).Scan(&total)

		rows, err := db.Query(`SELECT filename, date_taken, latitude, longitude, make, model
			FROM media ORDER BY date_taken DESC LIMIT ? OFFSET ?`, pageSize, offset)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var items []MediaItem
		for rows.Next() {
			var item MediaItem
			rows.Scan(&item.Filename, &item.DateTaken, &item.Latitude, &item.Longitude, &item.Make, &item.Model)
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

		j.start(path, db)

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
}
