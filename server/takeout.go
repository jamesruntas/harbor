package main

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	exiftool "github.com/barasher/go-exiftool"
)

// takeoutJob manages a single Google Takeout import from start to finish.
// States: idle → extracting → processing → preview → importing → done | error
type takeoutJob struct {
	mu sync.Mutex

	phase    string // current state
	message  string // human-readable progress line
	progress int    // files processed so far
	total    int    // total files in current phase
	newCount int    // files to import (set at preview)
	dupCount int    // files already in library (set at preview)
	errMsg   string

	confirmCh chan bool // preview gate: true=import, false=cancel
	tempBase  string   // os.MkdirTemp root, removed on finish
	newFiles  []string // staging paths to copy (set at preview)
}

func newTakeoutJob() *takeoutJob {
	return &takeoutJob{phase: "idle"}
}

func (t *takeoutJob) statusMap() map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()
	return map[string]any{
		"phase":     t.phase,
		"message":   t.message,
		"progress":  t.progress,
		"total":     t.total,
		"new_count": t.newCount,
		"dup_count": t.dupCount,
		"error":     t.errMsg,
	}
}

func (t *takeoutJob) begin(
	inputFolder, gpthPath, mediaFolder, exiftoolPath string,
	db *sql.DB, thumb *Thumbnailer, broker *Broker,
) error {
	t.mu.Lock()
	if t.phase != "idle" && t.phase != "done" && t.phase != "error" {
		t.mu.Unlock()
		return fmt.Errorf("import already in progress (phase: %s)", t.phase)
	}
	t.phase = "extracting"
	t.message = "Starting…"
	t.progress = 0
	t.total = 0
	t.newCount = 0
	t.dupCount = 0
	t.errMsg = ""
	t.confirmCh = make(chan bool, 1)
	t.newFiles = nil
	t.tempBase = ""
	t.mu.Unlock()

	go t.run(inputFolder, gpthPath, mediaFolder, exiftoolPath, db, thumb, broker)
	return nil
}

func (t *takeoutJob) confirm() { t.confirmCh <- true }
func (t *takeoutJob) cancel()  { t.confirmCh <- false }

func (t *takeoutJob) cleanup() {
	if t.tempBase != "" {
		os.RemoveAll(t.tempBase)
		t.tempBase = ""
	}
}

func (t *takeoutJob) fail(msg string) {
	log.Printf("takeout error: %s", msg)
	t.mu.Lock()
	t.phase = "error"
	t.errMsg = msg
	t.mu.Unlock()
	t.cleanup()
}

func (t *takeoutJob) set(phase, message string) {
	t.mu.Lock()
	t.phase = phase
	t.message = message
	t.mu.Unlock()
}

func (t *takeoutJob) run(
	inputFolder, gpthPath, mediaFolder, exiftoolPath string,
	db *sql.DB, thumb *Thumbnailer, broker *Broker,
) {
	// Create temp workspace
	base, err := os.MkdirTemp("", "harbor-takeout-*")
	if err != nil {
		t.fail("could not create temp directory: " + err.Error())
		return
	}
	t.mu.Lock()
	t.tempBase = base
	t.mu.Unlock()

	extractDir := filepath.Join(base, "extracted")
	stagingDir := filepath.Join(base, "staging")
	os.MkdirAll(extractDir, 0755)
	os.MkdirAll(stagingDir, 0755)

	// ── Phase 1: find and extract ZIPs ───────────────────────────────────────

	zips, err := findZips(inputFolder)
	if err != nil {
		t.fail("could not scan folder: " + err.Error())
		return
	}
	if len(zips) == 0 {
		t.fail("no ZIP files found in the selected folder")
		return
	}

	t.mu.Lock()
	t.total = len(zips)
	t.mu.Unlock()

	for i, z := range zips {
		t.mu.Lock()
		t.message = fmt.Sprintf("Extracting %s (%d of %d)…", filepath.Base(z), i+1, len(zips))
		t.progress = i + 1
		t.mu.Unlock()

		if err := extractZip(z, extractDir); err != nil {
			log.Printf("takeout: extract error %s: %v — skipping", filepath.Base(z), err)
		}
	}

	// ── Phase 2: run GPTH ────────────────────────────────────────────────────

	t.set("processing", "Running Google Photos Takeout Helper…")

	if _, err := os.Stat(gpthPath); os.IsNotExist(err) {
		t.fail("gpth.exe not found at " + gpthPath + " — add it to your tools directory")
		return
	}

	cmd := exec.Command(gpthPath,
		"--input", extractDir,
		"--output", stagingDir,
		"--albums", "nothing",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("takeout: gpth output:\n%s", string(out))
		t.fail("GPTH failed: " + err.Error())
		return
	}
	log.Printf("takeout: gpth completed")

	// ── Phase 3: scan output, classify new vs duplicate ──────────────────────

	t.set("processing", "Scanning output…")

	newFiles, dupCount, err := classifyStaging(stagingDir, mediaFolder, db)
	if err != nil {
		t.fail("failed to scan GPTH output: " + err.Error())
		return
	}

	t.mu.Lock()
	t.phase = "preview"
	t.message = ""
	t.newCount = len(newFiles)
	t.dupCount = dupCount
	t.newFiles = newFiles
	t.mu.Unlock()

	// ── Phase 4: wait for user to confirm or cancel ──────────────────────────

	if confirmed := <-t.confirmCh; !confirmed {
		t.mu.Lock()
		t.phase = "idle"
		t.mu.Unlock()
		t.cleanup()
		return
	}

	// ── Phase 5: copy files into media folder ────────────────────────────────

	t.mu.Lock()
	t.phase = "importing"
	t.total = len(newFiles)
	t.progress = 0
	t.mu.Unlock()

	var copied []string
	for i, src := range newFiles {
		t.mu.Lock()
		t.progress = i + 1
		t.message = fmt.Sprintf("Copying %d of %d…", i+1, len(newFiles))
		t.mu.Unlock()

		dest := filepath.Join(mediaFolder, filepath.Base(src))
		if err := copyFile(src, dest); err != nil {
			log.Printf("takeout: copy error %s: %v", filepath.Base(src), err)
			continue
		}
		copied = append(copied, dest)
	}

	// ── Phase 6: index copied files ──────────────────────────────────────────

	t.set("importing", "Indexing imported photos…")
	indexCopied(copied, exiftoolPath, db, thumb)

	t.mu.Lock()
	t.phase = "done"
	t.message = fmt.Sprintf("Import complete — %d photos added.", len(copied))
	t.mu.Unlock()

	broker.publish("index-done")
	t.cleanup()
}

// findZips returns all .zip files in dir (non-recursive).
func findZips(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.ToLower(filepath.Ext(e.Name())) == ".zip" {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out, nil
}

// extractZip extracts a ZIP file into destDir, preserving internal structure.
// Includes a path-traversal guard.
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	base := filepath.Clean(destDir) + string(os.PathSeparator)

	for _, f := range r.File {
		dest := filepath.Join(destDir, f.Name)
		if !strings.HasPrefix(filepath.Clean(dest)+string(os.PathSeparator), base) {
			log.Printf("takeout: skipping suspicious path in ZIP: %s", f.Name)
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(dest, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}

		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			continue
		}

		io.Copy(out, rc)
		out.Close()
		rc.Close()
	}
	return nil
}

// classifyStaging walks the GPTH output folder and separates files into
// new (not yet in the library) vs duplicate (already present by filename).
// TODO: replace filename-based detection with content hash for higher accuracy.
func classifyStaging(stagingDir, mediaFolder string, db *sql.DB) (newFiles []string, dupCount int, err error) {
	err = filepath.Walk(stagingDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return walkErr
		}
		if !supportedExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}

		filename := filepath.Base(path)

		// Duplicate if the file already exists in mediaFolder on disk.
		if _, statErr := os.Stat(filepath.Join(mediaFolder, filename)); statErr == nil {
			dupCount++
			return nil
		}

		// Duplicate if any DB row shares the filename.
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM media WHERE filename = ?`, filename).Scan(&count)
		if count > 0 {
			dupCount++
			return nil
		}

		newFiles = append(newFiles, path)
		return nil
	})
	return
}

// indexCopied runs ExifTool on a list of already-copied files and inserts them
// into the DB, triggering thumbnail generation for each.
func indexCopied(paths []string, exiftoolPath string, db *sql.DB, thumb *Thumbnailer) {
	if len(paths) == 0 {
		return
	}
	et, err := exiftool.NewExiftool(exiftool.SetExiftoolBinaryPath(exiftoolPath))
	if err != nil {
		log.Printf("takeout: exiftool init failed: %v", err)
		return
	}
	defer et.Close()

	for _, p := range paths {
		indexFile(p, et, db, thumb)
	}
}

// copyFile copies src to dest, creating dest if it does not exist.
func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
