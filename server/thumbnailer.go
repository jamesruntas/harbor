package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

// Thumbnailer generates and serves JPEG thumbnails via FFmpeg.
// Thumbnails are cached as {id}.jpg in cacheDir and never regenerated
// unless the cache file is deleted.
type Thumbnailer struct {
	ffmpegPath string
	cacheDir   string
	sem        chan struct{} // limits concurrent FFmpeg processes
}

func newThumbnailer(ffmpegPath, cacheDir string) *Thumbnailer {
	os.MkdirAll(cacheDir, 0755)
	return &Thumbnailer{
		ffmpegPath: ffmpegPath,
		cacheDir:   cacheDir,
		sem:        make(chan struct{}, 4),
	}
}

func (t *Thumbnailer) thumbPath(id int64) string {
	return filepath.Join(t.cacheDir, fmt.Sprintf("%d.jpg", id))
}

// generateAsync fires thumbnail generation in a goroutine, respecting
// the semaphore limit. Safe to call from the indexer hot path.
func (t *Thumbnailer) generateAsync(id int64, sourcePath string) {
	outPath := t.thumbPath(id)
	if _, err := os.Stat(outPath); err == nil {
		return // already cached
	}

	t.sem <- struct{}{}
	go func() {
		defer func() { <-t.sem }()

		// -filter_complex "[0:v:0]..." required for HEIC — see README gotchas.
		// -vframes 1 extracts the first frame (works for video too).
		cmd := exec.Command(t.ffmpegPath,
			"-i", sourcePath,
			"-filter_complex", "[0:v:0]scale=200:-1[out]",
			"-map", "[out]",
			"-vframes", "1",
			"-q:v", "3",
			outPath,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("thumbnail failed for %s: %v\n%s", sourcePath, err, out)
		}
	}()
}

// serve writes the cached thumbnail to the response, or 404 if not yet generated.
func (t *Thumbnailer) serve(w http.ResponseWriter, r *http.Request, id int64) {
	path := t.thumbPath(id)
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}
