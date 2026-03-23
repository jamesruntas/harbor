package main

import (
	"log"
	"os"
	"path/filepath"
)

// appDataDir returns (and creates if needed) %AppData%\Harbor.
// All mutable data lives here so the app can install to %ProgramFiles%
// without needing write access there after first install.
func appDataDir() string {
	base, err := os.UserConfigDir() // %AppData% on Windows
	if err != nil {
		log.Fatalf("cannot resolve AppData directory: %v", err)
	}
	dir := filepath.Join(base, "Harbor")
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("cannot create app data directory %s: %v", dir, err)
	}
	return dir
}
