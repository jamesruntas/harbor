package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"tailscale.com/tsnet"
	"tailscale.com/types/logger"
)

const (
	localAddr  = "0.0.0.0:4242"
	tsHostname = "harbor"
)

// resolveToolPath returns the path to a tool binary.
// Priority: settings.ToolsDir → HARBOR_TOOLS env var → tools/ next to executable.
func resolveToolPath(name string, toolsDir string) string {
	if toolsDir != "" {
		return filepath.Join(toolsDir, name)
	}
	if dir := os.Getenv("HARBOR_TOOLS"); dir != "" {
		return filepath.Join(dir, name)
	}
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("cannot resolve executable path: %v", err)
	}
	return filepath.Join(filepath.Dir(exe), "tools", name)
}

// dedupLogger returns a Logf func that prints each unique message only once.
// Prevents tsnet from spamming the auth URL every 5 seconds.
func dedupLogger() logger.Logf {
	var mu sync.Mutex
	seen := map[string]bool{}
	return func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		mu.Lock()
		first := !seen[msg]
		seen[msg] = true
		mu.Unlock()
		if first {
			log.Print(msg)
		}
	}
}

func main() {
	dataDir := appDataDir()
	log.Printf("data directory: %s", dataDir)

	cfg := newSettingsStore()
	s := cfg.get()

	exiftoolPath := resolveToolPath("exiftool.exe", s.ToolsDir)
	ffmpegPath   := resolveToolPath("ffmpeg.exe", s.ToolsDir)
	gpthPath     := resolveToolPath("gpth.exe", s.ToolsDir)
	log.Printf("tools: exiftool=%s  ffmpeg=%s  gpth=%s", exiftoolPath, ffmpegPath, gpthPath)
	log.Printf("media folder: %s", s.MediaFolder)

	dbPath        := filepath.Join(dataDir, "harbor.db")
	thumbDir      := filepath.Join(dataDir, "thumbnails")
	movieThumbDir := filepath.Join(dataDir, "movie-thumbnails")
	tsnetDir      := filepath.Join(dataDir, "tsnet-state")

	db         := initDB(dbPath)
	thumb      := newThumbnailer(ffmpegPath, thumbDir)
	movieThumb := newThumbnailer(ffmpegPath, movieThumbDir)
	broker     := newBroker()
	bj         := newBackupJob(dataDir)

	if err := startWatcher(s.MediaFolder, exiftoolPath, db, thumb, broker); err != nil {
		log.Printf("watcher disabled: %v", err)
	}

	mux := http.NewServeMux()
	registerHandlers(mux, exiftoolPath, gpthPath, cfg, db, thumb, movieThumb, broker, bj)
	handler := AuthMiddleware(cfg.get().APIToken, mux)

	// Local listener — used by the Wails UI
	go func() {
		log.Printf("local server listening on %s", localAddr)
		if err := http.ListenAndServe(localAddr, handler); err != nil {
			log.Fatalf("local server error: %v", err)
		}
	}()

	// tsnet listener — remote access via Tailscale (no router config needed).
	// Set HARBOR_NO_TSNET=1 in dev to skip this and avoid auth spam.
	if os.Getenv("HARBOR_NO_TSNET") != "" {
		log.Print("tsnet disabled (HARBOR_NO_TSNET set) — local only")
		select {} // block forever; local server goroutine keeps running
	}

	srv := &tsnet.Server{
		Hostname: tsHostname,
		Dir:      tsnetDir, // %AppData%\Harbor\tsnet-state
		Logf:     dedupLogger(),
	}
	defer srv.Close()

	log.Print("tsnet: waiting for Tailscale auth...")
	if _, err := srv.Up(context.Background()); err != nil {
		log.Fatalf("tsnet auth failed: %v", err)
	}

	ip4, _ := srv.TailscaleIPs()
	log.Printf("tsnet ready — remote access at http://%s  (or http://%s)", ip4, tsHostname)

	ln, err := srv.Listen("tcp", ":80")
	if err != nil {
		log.Fatalf("tsnet listen error: %v", err)
	}
	defer ln.Close()

	log.Fatal(http.Serve(ln, handler))
}
