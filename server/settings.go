package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Settings holds user-configurable server options persisted to settings.json.
type Settings struct {
	MediaFolder  string `json:"media_folder"`
	MoviesFolder string `json:"movies_folder"`
	ToolsDir     string `json:"tools_dir"`
}

// settingsStore wraps Settings with a mutex for concurrent API access.
type settingsStore struct {
	mu   sync.RWMutex
	data Settings
	path string
}

func newSettingsStore() *settingsStore {
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("settings: cannot resolve executable path: %v", err)
	}
	p := filepath.Join(filepath.Dir(exe), "settings.json")

	defaults := Settings{
		MediaFolder:  `C:\PhoneMedia`,
		MoviesFolder: "",
		ToolsDir:     "",
	}

	raw, err := os.ReadFile(p)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("settings: read error: %v — using defaults", err)
		}
		return &settingsStore{data: defaults, path: p}
	}

	s := defaults
	if err := json.Unmarshal(raw, &s); err != nil {
		log.Printf("settings: parse error: %v — using defaults", err)
	}
	log.Printf("settings: loaded from %s", p)
	return &settingsStore{data: s, path: p}
}

func (ss *settingsStore) get() Settings {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.data
}

func (ss *settingsStore) set(s Settings) error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(ss.path, raw, 0644); err != nil {
		return err
	}
	ss.mu.Lock()
	ss.data = s
	ss.mu.Unlock()
	log.Printf("settings: saved to %s", ss.path)
	return nil
}
