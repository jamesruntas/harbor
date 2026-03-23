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
	localAddr      = "127.0.0.1:4242"
	tsHostname     = "harbor"
	tsnetDir       = "tsnet-state"
	dbPath         = "../harbor.db"
	thumbDir       = "thumbnails"
	movieThumbDir  = "movie-thumbnails"
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
	cfg := newSettingsStore()
	s := cfg.get()

	exiftoolPath := resolveToolPath("exiftool.exe", s.ToolsDir)
	ffmpegPath   := resolveToolPath("ffmpeg.exe", s.ToolsDir)
	gpthPath     := resolveToolPath("gpth.exe", s.ToolsDir)
	log.Printf("tools: exiftool=%s  ffmpeg=%s  gpth=%s", exiftoolPath, ffmpegPath, gpthPath)
	log.Printf("media folder: %s", s.MediaFolder)

	db         := initDB(dbPath)
	thumb      := newThumbnailer(ffmpegPath, thumbDir)
	movieThumb := newThumbnailer(ffmpegPath, movieThumbDir)
	broker     := newBroker()

	if err := startWatcher(s.MediaFolder, exiftoolPath, db, thumb, broker); err != nil {
		log.Printf("watcher disabled: %v", err)
	}

	mux := http.NewServeMux()
	registerHandlers(mux, exiftoolPath, gpthPath, cfg, db, thumb, movieThumb, broker)
	handler := AuthMiddleware(cfg.get().APIToken, mux)

	// Local listener — used by the Wails UI
	go func() {
		log.Printf("local server listening on %s", localAddr)
		if err := http.ListenAndServe(localAddr, handler); err != nil {
			log.Fatalf("local server error: %v", err)
		}
	}()

	// tsnet listener — remote access via Tailscale (no router config needed)
	srv := &tsnet.Server{
		Hostname: tsHostname,
		Dir:      tsnetDir,
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
