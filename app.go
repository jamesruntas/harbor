package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	cmd.Dir = filepath.Dir(bin) // server resolves ../harbor.db relative to its own location
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

// PickFolder opens a native OS folder picker dialog and returns the selected path.
func (a *App) PickFolder() string {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Folder",
	})
	if err != nil || path == "" {
		return ""
	}
	// Normalise to Windows backslash separator.
	return filepath.FromSlash(strings.ReplaceAll(path, "/", "\\"))
}

// GetSettings returns the server's current settings as a JSON string.
func (a *App) GetSettings() string {
	resp, err := http.Get(serverBase + "/api/settings")
	if err != nil {
		return `{"error":"server unreachable"}`
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// SaveSettings sends updated settings to the server and returns the saved state.
// Expects a JSON string matching the Settings struct: {"media_folder":"...","tools_dir":"..."}.
func (a *App) SaveSettings(jsonStr string) string {
	resp, err := http.Post(
		serverBase+"/api/settings",
		"application/json",
		strings.NewReader(jsonStr),
	)
	if err != nil {
		return fmt.Sprintf(`{"error":"%v"}`, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// GetMonths returns the list of year/month buckets as a JSON string.
func (a *App) GetMonths() string {
	resp, err := http.Get(serverBase + "/api/months")
	if err != nil {
		return "[]"
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// GetMedia returns one page of media as a JSON string.
// year=0 and month=0 return all items with no date filter.
func (a *App) GetMedia(year, month, offset int) string {
	u := fmt.Sprintf("%s/api/media?year=%d&month=%d&offset=%d", serverBase, year, month, offset)
	resp, err := http.Get(u)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// StartTakeout begins a Google Takeout import from a folder of ZIPs.
func (a *App) StartTakeout(folder string) string {
	body := strings.NewReader(`{"folder":"` + strings.ReplaceAll(folder, `\`, `\\`) + `"}`)
	resp, err := http.Post(serverBase+"/api/takeout/start", "application/json", body)
	if err != nil {
		return fmt.Sprintf(`{"error":"%v"}`, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// GetTakeoutStatus returns the current takeout job state as a JSON string.
func (a *App) GetTakeoutStatus() string {
	resp, err := http.Get(serverBase + "/api/takeout/status")
	if err != nil {
		return `{"phase":"error","error":"server unreachable"}`
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// ConfirmTakeout approves the import preview and starts copying files.
func (a *App) ConfirmTakeout() string {
	resp, err := http.Post(serverBase+"/api/takeout/confirm", "", nil)
	if err != nil {
		return fmt.Sprintf(`{"error":"%v"}`, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// CancelTakeout cancels the import at the preview stage and cleans up.
func (a *App) CancelTakeout() string {
	resp, err := http.Post(serverBase+"/api/takeout/cancel", "", nil)
	if err != nil {
		return fmt.Sprintf(`{"error":"%v"}`, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// GetMovies returns one page of indexed movies as a JSON string.
func (a *App) GetMovies(offset int) string {
	resp, err := http.Get(fmt.Sprintf("%s/api/movies?offset=%d", serverBase, offset))
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// IndexMovies triggers the server to scan the configured movies folder.
func (a *App) IndexMovies() string {
	resp, err := http.Post(serverBase+"/api/movies/index", "", nil)
	if err != nil {
		return fmt.Sprintf(`{"error":"%v"}`, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// GetMoviesStatus returns the current movie indexing job state as a JSON string.
func (a *App) GetMoviesStatus() string {
	resp, err := http.Get(serverBase + "/api/movies/index/status")
	if err != nil {
		return `{"status":"error","error":"server unreachable"}`
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
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

// jsonGet is a helper used by SaveSettings to detect error responses.
func jsonGet(body []byte, key string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return ""
	}
	if v, ok := m[key]; ok {
		var s string
		json.Unmarshal(v, &s)
		return s
	}
	return ""
}
