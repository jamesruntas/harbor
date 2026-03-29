package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"sync"
)

// Settings holds user-configurable server options persisted to settings.json.
type Settings struct {
	MediaFolder  string `json:"media_folder"`
	MoviesFolder string `json:"movies_folder"`
	BackupDest   string `json:"backup_dest"`
	ToolsDir     string `json:"tools_dir"`
	APIToken     string `json:"api_token"`
}

// settingsStore wraps Settings with a mutex for concurrent API access.
type settingsStore struct {
	mu   sync.RWMutex
	data Settings
	path string
}

func newSettingsStore() *settingsStore {
	p := appDataDir() + string(os.PathSeparator) + "settings.json"

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
		// First run — generate a token and persist defaults immediately.
		b := make([]byte, 16)
		if _, err2 := rand.Read(b); err2 == nil {
			defaults.APIToken = hex.EncodeToString(b)
		}
		store := &settingsStore{data: defaults, path: p}
		store.set(defaults)
		log.Printf("settings: created %s with new API token", p)
		return store
	}

	s := defaults
	if err := json.Unmarshal(raw, &s); err != nil {
		log.Printf("settings: parse error: %v — using defaults", err)
	}
	log.Printf("settings: loaded from %s", p)

	store := &settingsStore{data: s, path: p}
	if s.APIToken == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err == nil {
			store.data.APIToken = hex.EncodeToString(b)
			store.set(store.data) //nolint — best effort; log if it fails
			log.Printf("settings: generated new API token")
		}
	}
	return store
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
