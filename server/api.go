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

type MonthBucket struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Count int `json:"count"`
}

// job tracks the state of the background indexing job.
// Only one job runs at a time — Harbor is single-user.
type job struct {
	mu      sync.Mutex
	status  string // "idle" | "running" | "done" | "error"
	indexed int
	errMsg  string
}

func (j *job) start(folderPath string, exiftoolPath string, db *sql.DB, thumb *Thumbnailer, broker *Broker) {
	j.mu.Lock()
	j.status = "running"
	j.indexed = 0
	j.errMsg = ""
	j.mu.Unlock()

	go func() {
		err := indexFolder(folderPath, exiftoolPath, db, thumb, func() {
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

func registerHandlers(mux *http.ServeMux, exiftoolPath string, gpthPath string, cfg *settingsStore, db *sql.DB, thumb *Thumbnailer, movieThumb *Thumbnailer, broker *Broker) {
	j := &job{status: "idle"}
	tj := newTakeoutJob()

	// GET /api/media?year=0&month=0&offset=0 — paginated media list (100 per page).
	// year=0 and month=0 mean no filter (return all).
	mux.HandleFunc("GET /api/media", func(w http.ResponseWriter, r *http.Request) {
		const pageSize = 100
		q := r.URL.Query()
		offset, _ := strconv.Atoi(q.Get("offset"))
		year, _   := strconv.Atoi(q.Get("year"))
		month, _  := strconv.Atoi(q.Get("month"))

		// Build WHERE clause dynamically based on filters.
		where := `WHERE (? = 0 OR CAST(SUBSTR(date_taken, 1, 4) AS INTEGER) = ?)
		      AND (? = 0 OR CAST(SUBSTR(date_taken, 6, 2) AS INTEGER) = ?)`
		args := []any{year, year, month, month}

		var total int
		db.QueryRow(`SELECT COUNT(*) FROM media `+where, args...).Scan(&total)

		rows, err := db.Query(
			`SELECT id, filename, date_taken, latitude, longitude, make, model
			 FROM media `+where+` ORDER BY date_taken DESC LIMIT ? OFFSET ?`,
			append(args, pageSize, offset)...,
		)
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

	// GET /api/months — list of year/month buckets with counts, newest first.
	// date_taken format: "2024:03:15 14:22:01" — year at pos 1-4, month at pos 6-7.
	mux.HandleFunc("GET /api/months", func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT
				CAST(SUBSTR(date_taken, 1, 4) AS INTEGER) AS year,
				CAST(SUBSTR(date_taken, 6, 2) AS INTEGER) AS month,
				COUNT(*) AS count
			FROM media
			WHERE date_taken != '' AND LENGTH(date_taken) >= 7
			GROUP BY year, month
			ORDER BY year DESC, month DESC
		`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var buckets []MonthBucket
		for rows.Next() {
			var b MonthBucket
			rows.Scan(&b.Year, &b.Month, &b.Count)
			buckets = append(buckets, b)
		}
		if buckets == nil {
			buckets = []MonthBucket{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buckets)
	})

	// GET /api/settings — return current settings as JSON.
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg.get())
	})

	// POST /api/settings — update and persist settings.
	// Tool path changes take effect on next server restart.
	// MediaFolder change takes effect immediately for the next index run.
	mux.HandleFunc("POST /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var s Settings
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if s.MediaFolder == "" {
			http.Error(w, "media_folder is required", http.StatusBadRequest)
			return
		}
		if err := cfg.set(s); err != nil {
			http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg.get())
	})

	// POST /api/index — start a background indexing job.
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

		j.start(path, exiftoolPath, db, thumb, broker)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"started"}`))
	})

	// GET /api/index/status — poll indexing progress.
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

	// GET /api/events — SSE stream, pushes "new-file" and "index-done" events.
	mux.HandleFunc("GET /api/events", broker.ServeSSE)

	// GET /api/stream/{id} — serve original file with range-request support (photos + videos).
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

	// GET /api/thumbnail/{id} — serve cached thumbnail JPEG.
	mux.HandleFunc("GET /api/thumbnail/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		thumb.serve(w, r, id)
	})

	// ── Movies & TV ───────────────────────────────────────────────────────────

	mj := &job{status: "idle"}

	// GET /api/movies?offset=N — paginated movie list.
	mux.HandleFunc("GET /api/movies", func(w http.ResponseWriter, r *http.Request) {
		const pageSize = 100
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		var total int
		db.QueryRow(`SELECT COUNT(*) FROM movies`).Scan(&total)

		rows, err := db.Query(
			`SELECT id, filename, size, modified_at FROM movies ORDER BY filename ASC LIMIT ? OFFSET ?`,
			pageSize, offset,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type MovieItem struct {
			ID         int64  `json:"id"`
			Filename   string `json:"filename"`
			Size       int64  `json:"size"`
			ModifiedAt string `json:"modified_at"`
		}
		var items []MovieItem
		for rows.Next() {
			var m MovieItem
			rows.Scan(&m.ID, &m.Filename, &m.Size, &m.ModifiedAt)
			items = append(items, m)
		}
		if items == nil {
			items = []MovieItem{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"items": items, "total": total})
	})

	// POST /api/movies/index — scan movies_folder, background job.
	mux.HandleFunc("POST /api/movies/index", func(w http.ResponseWriter, r *http.Request) {
		folder := cfg.get().MoviesFolder
		if folder == "" {
			http.Error(w, "movies_folder not set — save it in Settings first", http.StatusBadRequest)
			return
		}

		mj.mu.Lock()
		running := mj.status == "running"
		mj.mu.Unlock()
		if running {
			http.Error(w, "index already running", http.StatusConflict)
			return
		}

		mj.mu.Lock()
		mj.status = "running"
		mj.indexed = 0
		mj.errMsg = ""
		mj.mu.Unlock()

		go func() {
			err := indexMovies(folder, db, movieThumb, func() {
				mj.mu.Lock()
				mj.indexed++
				mj.mu.Unlock()
			})
			mj.mu.Lock()
			if err != nil {
				mj.status = "error"
				mj.errMsg = err.Error()
			} else {
				mj.status = "done"
			}
			mj.mu.Unlock()
			broker.publish("movies-done")
		}()

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"started"}`))
	})

	// GET /api/movies/index/status — poll movie indexing progress.
	mux.HandleFunc("GET /api/movies/index/status", func(w http.ResponseWriter, r *http.Request) {
		mj.mu.Lock()
		status := mj.status
		indexed := mj.indexed
		errMsg := mj.errMsg
		mj.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  status,
			"indexed": indexed,
			"error":   errMsg,
		})
	})

	// GET /api/movies/stream/{id} — serve original movie file with range-request support.
	mux.HandleFunc("GET /api/movies/stream/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var filePath string
		if err := db.QueryRow(`SELECT path FROM movies WHERE id = ?`, id).Scan(&filePath); err != nil {
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

	// GET /api/movies/thumbnail/{id} — serve cached movie thumbnail JPEG.
	mux.HandleFunc("GET /api/movies/thumbnail/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		movieThumb.serve(w, r, id)
	})

	// ── Google Takeout import ─────────────────────────────────────────────────

	// POST /api/takeout/start — begin import from a folder of Takeout ZIPs.
	// Body: JSON {"folder": "C:\\path\\to\\zips"}
	mux.HandleFunc("POST /api/takeout/start", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Folder string `json:"folder"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Folder == "" {
			http.Error(w, "missing folder", http.StatusBadRequest)
			return
		}
		mediaFolder := cfg.get().MediaFolder
		if err := tj.begin(req.Folder, gpthPath, mediaFolder, exiftoolPath, db, thumb, broker); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"started"}`))
	})

	// GET /api/takeout/status — poll import progress.
	mux.HandleFunc("GET /api/takeout/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tj.statusMap())
	})

	// POST /api/takeout/confirm — user approves the preview, begins file copy.
	mux.HandleFunc("POST /api/takeout/confirm", func(w http.ResponseWriter, r *http.Request) {
		tj.confirm()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"importing"}`))
	})

	// POST /api/takeout/cancel — user cancels at preview, cleans up temp files.
	mux.HandleFunc("POST /api/takeout/cancel", func(w http.ResponseWriter, r *http.Request) {
		tj.cancel()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"cancelled"}`))
	})
}
