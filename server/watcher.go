package main

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	exiftool "github.com/barasher/go-exiftool"
	"github.com/fsnotify/fsnotify"
)

// startWatcher watches folderPath for new media files and indexes them
// automatically. A 2-second debounce per path ensures files are fully
// written before processing — important for Syncthing partial transfers.
//
// The watcher keeps a single ExifTool process alive for its lifetime,
// avoiding the per-file spawn overhead of the bulk indexer.
//
// Note: non-recursive — subdirectories added after startup are not watched.
// Phase 1 assumption: C:\PhoneMedia is flat (Syncthing default layout).
func startWatcher(folderPath string, db *sql.DB, thumb *Thumbnailer, broker *Broker) error {
	et, err := exiftool.NewExiftool(exiftool.SetExiftoolBinaryPath(`C:\Users\James\HarborTools\exiftool.exe`))
	if err != nil {
		return fmt.Errorf("watcher: exiftool init failed: %w", err)
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		et.Close()
		return fmt.Errorf("watcher: fsnotify init failed: %w", err)
	}

	if err := fw.Add(folderPath); err != nil {
		et.Close()
		fw.Close()
		return fmt.Errorf("watcher: cannot watch %s: %w", folderPath, err)
	}

	log.Printf("watcher: watching %s", folderPath)

	go func() {
		defer et.Close()
		defer fw.Close()

		var mu sync.Mutex
		pending := map[string]*time.Timer{}

		for {
			select {
			case event, ok := <-fw.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create == 0 {
					continue
				}

				mu.Lock()
				if t, ok := pending[event.Name]; ok {
					t.Stop()
				}
				pending[event.Name] = time.AfterFunc(2*time.Second, func() {
					mu.Lock()
					delete(pending, event.Name)
					mu.Unlock()

					log.Printf("watcher: new file detected: %s", event.Name)
					if indexFile(event.Name, et, db, thumb) {
						broker.publish("new-file")
					}
				})
				mu.Unlock()

			case err, ok := <-fw.Errors:
				if !ok {
					return
				}
				log.Printf("watcher error: %v", err)
			}
		}
	}()

	return nil
}
