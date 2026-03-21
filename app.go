package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
)

const serverBase = "http://127.0.0.1:4242"

type App struct {
	ctx       context.Context
	serverCmd *exec.Cmd
}

// findServerBinary looks for the server binary next to the running executable
// (production) or in the server/ subdirectory relative to the working directory
// (development — run `go build -o server.exe .` inside server/ first).
func findServerBinary() (string, error) {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "server.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, "server", "server.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("server binary not found — build it first: cd server && go build -o server.exe .")
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	bin, err := findServerBinary()
	if err != nil {
		log.Printf("startup: %v", err)
		return
	}

	cmd := exec.Command(bin)
	cmd.Dir = filepath.Dir(bin) // server resolves ../homestream.db relative to its own location
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Printf("startup: failed to start server: %v", err)
		return
	}

	a.serverCmd = cmd
	log.Printf("startup: server started (pid %d)", cmd.Process.Pid)
}

func (a *App) shutdown(ctx context.Context) {
	if a.serverCmd != nil && a.serverCmd.Process != nil {
		log.Printf("shutdown: stopping server (pid %d)", a.serverCmd.Process.Pid)
		a.serverCmd.Process.Kill()
	}
}

// IndexFolder triggers the server to index a folder.
func (a *App) IndexFolder(path string) string {
	resp, err := http.PostForm(serverBase+"/api/index", url.Values{"path": {path}})
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// GetIndexStatus returns the current indexing job state as a JSON string.
func (a *App) GetIndexStatus() string {
	resp, err := http.Get(serverBase + "/api/index/status")
	if err != nil {
		return `{"status":"error","error":"server unreachable"}`
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// GetMedia returns one page of indexed media as a JSON string.
func (a *App) GetMedia(offset int) string {
	resp, err := http.Get(fmt.Sprintf("%s/api/media?offset=%d", serverBase, offset))
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}
