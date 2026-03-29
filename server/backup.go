package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BackupState is persisted to backup_state.json across server restarts.
type BackupState struct {
	LastBackupAt string `json:"last_backup_at"` // RFC3339, empty if never backed up
}

// backupJob manages the single background Robocopy job.
type backupJob struct {
	mu        sync.Mutex
	status    string // "idle" | "running" | "done" | "error"
	errMsg    string
	stateFile string
	state     BackupState
}

func newBackupJob(dataDir string) *backupJob {
	b := &backupJob{
		status:    "idle",
		stateFile: filepath.Join(dataDir, "backup_state.json"),
	}
	if raw, err := os.ReadFile(b.stateFile); err == nil {
		json.Unmarshal(raw, &b.state) //nolint — best effort
	}
	return b
}

func (b *backupJob) persistState() {
	raw, _ := json.MarshalIndent(b.state, "", "  ")
	os.WriteFile(b.stateFile, raw, 0644) //nolint — best effort
}

// start launches Robocopy in the background. Returns an error if already running.
func (b *backupJob) start(src, dest string, broker *Broker) error {
	b.mu.Lock()
	if b.status == "running" {
		b.mu.Unlock()
		return fmt.Errorf("backup already running")
	}
	b.status = "running"
	b.errMsg = ""
	b.mu.Unlock()

	go func() {
		// /E   = copy subdirs including empty  (NEVER /MIR — MIR deletes files not in src)
		// /R:1 /W:1 = one retry, one-second wait between retries
		// /NP /NDL /NFL = suppress per-file progress noise in the log
		cmd := exec.Command("robocopy", src, dest, "/E", "/R:1", "/W:1", "/NP", "/NDL", "/NFL")
		out, _ := cmd.CombinedOutput()
		exitCode := 0
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}

		b.mu.Lock()
		// Robocopy exit codes 0-7 = success variants; 8+ = at least one error
		if exitCode >= 8 {
			b.status = "error"
			b.errMsg = lastNonEmptyLine(string(out))
		} else {
			b.status = "done"
			b.state.LastBackupAt = time.Now().UTC().Format(time.RFC3339)
			b.persistState()
		}
		b.mu.Unlock()

		log.Printf("backup: robocopy finished (exit=%d)", exitCode)
		broker.publish("backup-done")
	}()

	return nil
}

func (b *backupJob) statusMap() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	return map[string]any{
		"status":         b.status,
		"error":          b.errMsg,
		"last_backup_at": b.state.LastBackupAt,
	}
}

// listDrives returns all drive roots (e.g. ["C:\\", "D:\\"]) that currently exist on Windows.
func listDrives() []string {
	var drives []string
	for c := 'A'; c <= 'Z'; c++ {
		d := string(c) + ":\\"
		if _, err := os.Stat(d); err == nil {
			drives = append(drives, d)
		}
	}
	return drives
}

// lastNonEmptyLine returns the last non-blank line from s (used to extract Robocopy error text).
func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return strings.TrimSpace(s)
}
