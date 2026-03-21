package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"tailscale.com/tsnet"
	"tailscale.com/types/logger"
)

const (
	localAddr  = "127.0.0.1:4242"
	tsHostname = "homestream"
	tsnetDir   = "tsnet-state"
	dbPath     = "../homestream.db"
)

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
	db := initDB(dbPath)

	mux := http.NewServeMux()
	registerHandlers(mux, db)

	// Local listener — used by the Wails UI
	go func() {
		log.Printf("local server listening on %s", localAddr)
		if err := http.ListenAndServe(localAddr, mux); err != nil {
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

	// Block until authenticated and online. On first run, tsnet logs a single
	// auth URL — visit it to add this device to your Tailscale network.
	// Auth state is persisted in tsnet-state/ so subsequent runs skip this.
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

	log.Fatal(http.Serve(ln, mux))
}
