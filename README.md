# Harbor

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Completed Personal Media Server](#2-completed-personal-media-server)
3. [Product Architecture](#3-product-architecture)
4. [Tech Stack](#4-tech-stack)
5. [Key Product Decisions](#5-key-product-decisions)
6. [Phased Development Plan](#6-phased-development-plan)
7. [Current Codebase State](#7-current-codebase-state)
8. [Development Environment](#8-development-environment)
9. [Known Gotchas](#9-known-gotchas)
10. [Next Steps — Phase 2](#10-next-steps--phase-2)

---

## 1. Project Overview

Harbor is a **free, open-source alternative to Plex** — a single `.exe` installer that sets up a complete self-hosted media server on Windows with no Docker, no terminal, and no networking knowledge required.

### One-Line Pitch
> *"Keep Google Photos. Stop paying for it."*

Harbor is positioned as a **Google complement, not a replacement**. Users keep compressed copies on Google's free 15 GB tier for redundancy and store full-resolution originals on Harbor. This removes the deletion anxiety problem entirely — no behaviour change required from the user.

### The Problem
- Google Photos and iCloud charge recurring fees with storage limits
- Plex solved the self-hosted UX problem but moved remote streaming behind a paid subscription
- Free alternatives (Immich, Jellyfin, Syncthing, Tailscale) match or exceed Plex's functionality but require Docker, terminal work, and networking knowledge — a barrier that excludes mainstream users
- Nothing equivalent to CasaOS/Umbrel exists for Windows
- The homelab setup James completed firsthand took days and required significant technical knowledge — that friction is the product gap

### Target Users
- Non-technical users who want to stop paying Google/Apple but can't navigate Docker
- Technical users who find the Jellyfin + Immich + Syncthing + Möbius Sync + Tailscale stack inefficient and fragmented
- People who tried Plex, found it's now paid, Googled free alternatives, saw Docker requirements, and gave up
- iCloud users on Windows *(secondary market, Phase 3)* — no good Windows iCloud experience exists

### What It Is Not
- Not a new media server — the underlying technology already exists
- Not trying to beat Plex on features
- Not for the homelab community — they already know how to use Jellyfin
- Not a decentralised storage solution (Filecoin assessed and ruled out — immutability is a dealbreaker for photo management)

---

## 2. Completed Personal Media Server

James built and validated the full self-hosted stack firsthand on a Windows PC. This hands-on experience is the direct source of the product insight — the friction of this setup is what Harbor eliminates.

### Hardware
- **PC:** Daily-use Windows machine, 12-core CPU
- **SSD #1 (C:):** Primary storage — `C:\PhoneMedia` (iPhone media), `C:\Docker\` (services)
- **SSD #2:** Redundant backup via Robocopy
- **SSD #3 (F:):** `F:\Movies & TV` — local movie and TV library
- **iPhone 13**

### Completed Stack

| Component | Tool | Status |
|---|---|---|
| iPhone → PC sync | Syncthing + Möbius Sync (iOS) | ✅ Live |
| Primary media folder | `C:\PhoneMedia` | ✅ Live |
| Nightly backup | Robocopy /E → SSD #2 | ✅ Live |
| Snapchat file filter | Syncthing ignore pattern `cm-chat-media-*` | ✅ Live |
| Container runtime | Docker Desktop v29.2.1 | ✅ Live |
| Photo management | Immich (Docker) on port 2283 | ✅ Live |
| Media server | Jellyfin (Docker) on port 8096 | ✅ Live |
| Remote access | Tailscale (PC + iPhone) | ✅ Live |
| WSL2 memory cap | `.wslconfig` — 6 GB cap | ✅ Live |

### Key Lessons Learned (Inform Product Design)
- **Snapchat filenames** — iOS saves Snapchat videos with colons in filenames. Colons are reserved on Windows. Harbor must handle this at ingestion.
- **Robocopy /MIR vs /E** — /MIR mirrors deletions, defeating the purpose of backup. Always /E.
- **Immich standalone Windows exe** — does not exist. Immich requires Docker. Harbor's value prop is being the native Windows alternative.
- **WSL2 memory** — Docker Desktop on Windows uses a WSL2 VM with no memory cap by default. Was consuming 16 GB uncapped.
- **Incomplete transfers** — iOS can transfer incomplete files if the photo is still processing after capture. Harbor must handle partial files gracefully.

---

## 3. Product Architecture

### Windows App
A single `.exe` installer that sets up a complete native media server:
- Photo and video indexing (SQLite + ExifTool + libvips)
- Movie and TV library with automatic metadata (FFmpeg + TMDB API)
- File sync from iPhone (custom HTTPS V1, embedded Syncthing V2)
- Remote access with no router config (tsnet — Tailscale's embeddable Go library)
- Google Takeout importer (native GPTH-equivalent logic built in)
- Dashboard UI (Wails + React)

### iOS App
A single app replacing Möbius Sync + Tailscale + Immich + Jellyfin:
- Automatic photo backup to Windows server
- Photo and video browser
- Movie and TV streaming
- QR code pairing on first launch
- Background operation via Network Extension (VPN entitlement)

### Google Takeout Importer — Key Feature
Built natively into the Windows app. Four steps:
1. **Guided Takeout request** — annotated in-app walkthrough of takeout.google.com
2. **Drag and drop ZIPs** — user drops entire Takeout folder, app handles extraction
3. **Automated JSON reconciliation** — native GPTH-equivalent logic, runs silently
4. **Preview before import** — shows what will be imported, flags duplicates, never auto-deletes

Privacy angle: the app never connects to Google. User retrieves their own data, hands it to the app, processed entirely offline.

---

## 4. Tech Stack

### Windows

| Component | Technology | Licence |
|---|---|---|
| App framework | Go + Wails | MIT |
| Photo indexing | SQLite + ExifTool + libvips | Public domain / LGPL |
| Media serving | FFmpeg + TMDB API | LGPL / Free |
| File sync | Custom HTTPS (V1), Syncthing (V2) | MPL-2.0 |
| Remote access | tsnet (Tailscale Go library) | BSD-3 |
| Installer | Inno Setup | Free |

### iOS

| Component | Technology |
|---|---|
| Framework | Swift + SwiftUI |
| Photo browser | Custom SwiftUI |
| Media player | AVPlayer |
| File sync | Custom HTTPS — `POST /api/upload` on server |
| Remote access | Tailscale (tsnet on server side, Tailscale app removed from iOS path) |
| Background sync | Background App Refresh / URLSession background tasks |

### Licence Note
Immich (AGPL) and Jellyfin (GPL) are deliberately **not bundled** — native replacements avoid licence complications for commercial use. All components in the proposed stack are commercially compatible.

---

## 5. Key Product Decisions

### Backup Strategy
Default recommendation is both:

1. **External drive** — app detects plugged-in drives, one-click backup setup, confidence dashboard showing "X copies · last backed up Y hours ago"
2. **Storage Saver** — keep compressed copies on Google's free 15 GB as offsite redundancy, framed as parallel safety net not migration

**Single-drive case:** persistent warning UI ("Your library has 1 copy. If this drive fails, your photos are gone.") — non-blocking, gives path forward. Not a product flaw to hide, a constraint to surface honestly.

**Cloud backup (Backblaze B2):** deferred to Phase 3 as optional community plugin. Reintroduces subscription and cloud dependency — contradicts core premise for most users.

### iOS Remote Access — No Tailscale App Required
The final iOS app will **not** require Tailscale installed on the iPhone:
- **LAN (same WiFi):** server binds `0.0.0.0`, iOS app uses plain HTTP with `NSAllowsLocalNetworking`
- **Remote:** tsnet runs inside the server process; iOS app connects via the tsnet Tailscale IP — no Tailscale app on the phone needed
- Native apps can use plain HTTP on LAN; self-signed cert complexity only applies to browser-based access (not pursued)

### Google vs iCloud Positioning

**Google Photos users** are the primary Phase 1–2 target:
- Takeout flow is well-understood, GPTH-equivalent logic is buildable
- Storage Saver pitch removes deletion anxiety entirely
- 15 GB free compressed tier is meaningful runway

**iCloud users on Windows** are the secondary Phase 3 target:
- No good Windows iCloud experience exists — stronger pain point than Google users
- iCloud export is significantly harder: 1,000-photo-at-a-time cap on web, no official bulk export API, albums not preserved
- No Storage Saver equivalent — only 5 GB free, no compressed tier
- iCloud importer is a more differentiated feature but harder to build — Phase 3

### Harbor Positioning
Positioned as **Google complement** (Storage Saver angle) for onboarding. The user never has to answer "what if my PC dies" — Google is still there. Augmentation, not replacement.

---

## 6. Phased Development Plan

### Phase 0 — Foundation ✅ COMPLETE *(March 2026)*

Pre-build validation before writing production code. All critical items done.

**Homelab tasks**
- [x] Go + Wails hello-world — window opens, React renders, hot reload works
- [x] ExifTool → SQLite core loop — indexing `C:\PhoneMedia`, DB rows confirmed
- [x] tsnet proof of concept — server authenticates to Tailscale, reachable at `http://harbor/` with no router config
- [ ] Tailscale direct connection — fix relay via UDP port 41641 *(non-blocking, deferred)*

**Architecture decisions made this phase**
- [x] Two-binary split: Wails UI + Go server (required by Go version incompatibility — see Section 9)
- [x] Server HTTP API design: `GET /api/media`, `POST /api/index`, `GET /api/index/status`
- [x] Async indexing with job state — fire-and-forget POST, poll status endpoint
- [x] Backup UX strategy — decided (see Section 5)

---

### Phase 1 — Windows Core ✅ MOSTLY COMPLETE *(March 2026)*

**Success criterion:** non-technical user installs and browses their media library within 10 minutes.

#### Media Engine
- [x] ExifTool integration — extract date, GPS, make, model per file
- [x] SQLite schema — media table with path, filename, date_taken, latitude, longitude, make, model
- [x] Async indexing with job state — `POST /api/index` returns immediately, poll `/api/index/status`
- [x] Case-insensitive extension matching — handles `.MOV`, `.HEIC`, `.JPG` from iPhone
- [x] Silent drop fix — ExifTool errors log and insert with empty metadata rather than skipping
- [x] Video date fix — falls back to `CreateDate` when `DateTimeOriginal` absent (MP4/MOV)
- [x] FFmpeg thumbnail generation — 200px JPEG, 4-worker pool, skip-if-cached, async per file
- [x] Thumbnail cache — `thumbnails/{id}.jpg` in AppData, keyed by DB id
- [x] File watcher (fsnotify) — auto-index on new files arriving in the configured media folder
- [x] SSE auto-refresh — EventSource in frontend, broker publishes `new-file` / `index-done`
- [x] Configurable tool paths — `HARBOR_TOOLS` env var or `settings.json`, no more hardcoded user paths
- [x] FFmpeg video streaming — `GET /api/stream/{id}` and `GET /api/movies/stream/{id}`, range-request support for seeking
- [x] Movies & TV tab — separate folder, scan + stream + lightbox player
- [ ] Preview size thumbnail (800px) — deferred until lightbox view
- [ ] Movie thumbnail reliability — hit-and-miss on MKV *(known issue)*
- [ ] TMDB metadata scraper for movies and TV

#### Google Takeout Importer
- [x] ZIP extraction — extracts all ZIPs in a selected folder into a temp workspace
- [x] GPTH integration — runs `gpth.exe --albums nothing` to reconcile JSON sidecars and restore EXIF dates/GPS
- [x] Duplicate detection — filename + DB lookup; flags files already in the library
- [x] Preview before import — shows new count vs duplicate count, user confirms or cancels
- [x] Non-destructive — originals never modified; new files copied flat into `media_folder`, temp workspace cleaned up
- [x] Indexed automatically after copy — ExifTool runs on imported files, thumbnails generated
- [x] SSE refresh — grid updates automatically when import completes
- [ ] In-app guided Takeout request walkthrough *(deferred to Phase 3 polish)*

#### Backup Story
- [x] External drive detection on startup
- [x] One-click backup destination setup (Settings modal with drive chips)
- [x] Background Robocopy sync (`/E` not `/MIR`) with SSE completion event
- [x] Confidence dashboard — "2 copies · last backed up Y ago"
- [x] Single-drive persistent warning banner
- [ ] Storage Saver prompt — Google free tier as offsite redundancy *(Phase 3)*

#### Dashboard UI (Wails + React)
- [x] Photo grid with real thumbnails (FFmpeg-generated JPEG)
- [x] Video cards — play icon overlay, first-frame thumbnail
- [x] Lightbox view — click to open full-size photo or video, keyboard navigation (←/→/Esc)
- [x] Paginated media list (100 per page, "Load more N remaining")
- [x] Index button with live file counter
- [x] Error state clears automatically on server recovery
- [x] Date-based browsing — year/month sidebar, click to filter grid
- [x] Settings panel — media folder, movies folder, backup destination, tools directory, API token
- [x] Backup status — warning banner (no backup set) + confidence bar (copies + last backup time)
- [x] Google Takeout import modal with progress + preview
- [ ] Basic search (filename, date range) *(deferred)*

#### Installer
- [ ] Inno Setup installer
- [ ] ExifTool + FFmpeg + GPTH bundled in `tools\` alongside server.exe
- [ ] First-run onboarding wizard

#### iOS Upload Receiver *(pulled forward)*
- [x] `POST /api/upload` — receives single file from iOS app, saves to `media_folder`, indexes immediately, returns `{id, filename, status}`

---

### Phase 2 — iOS App *(next)*

**Success criterion:** user views home photo library and streams a movie from mobile data.

#### Server — iOS Readiness
- [ ] LAN access — bind `0.0.0.0`, firewall rule, `NSAllowsLocalNetworking` HTTP (no TLS needed for native app)
- [ ] QR code pairing endpoint — `GET /api/pairing` returns `{url, token}` for the iOS app to scan and configure itself
- [ ] Auth token displayed in Settings for manual entry fallback

#### iOS App
- [ ] Xcode project setup, Swift + SwiftUI
- [ ] QR code scanner on first launch → store server URL + token
- [ ] Photo library browser (SwiftUI grid, same month-bucketed layout)
- [ ] Video player (AVPlayer)
- [ ] Photo upload iOS → Windows via `POST /api/upload`
- [ ] Background upload (URLSession background task)
- [ ] Upload queue with retry logic
- [ ] Progress indicators
- [ ] Movie browser + AVPlayer streaming

---

### Phase 3 — Polish + iCloud + Sync *(2–3 months)*

**Success criterion:** experience is indistinguishable from a commercial product.

#### iCloud Importer
- [ ] privacy.apple.com export guide (in-app walkthrough)
- [ ] ZIP batch download handling (1,000-photo Apple limit workaround)
- [ ] Metadata parsing — Apple's export format differs from Google's
- [ ] Album structure preservation

#### Product Polish
- [ ] Face detection and grouping (on-device ML)
- [ ] Search by date, location, content tags
- [ ] Shared albums
- [ ] Auto-update mechanism
- [ ] Onboarding refinement based on user feedback
- [ ] Performance optimisation for large libraries (100k+ photos)

#### Sync
- [ ] Embedded Syncthing for automatic background sync (replaces custom HTTPS V1)
- [ ] Conflict resolution UI

#### Optional / Community
- [ ] Backblaze B2 cloud backup plugin
- [ ] Android app
- [ ] Multi-user support

---

## 7. Current Codebase State

### Binary Architecture

Harbor runs as **two separate binaries** that must both be running. The Wails UI auto-launches the server on startup and kills it on close.

```
harbor\                          ← Wails UI binary (Go 1.23)
├── main.go                      ← Wails bootstrap, registers OnStartup/OnShutdown
├── app.go                       ← App struct: finds + spawns server, HTTP proxy methods
├── wails.json
├── frontend\
│   ├── src\
│   │   ├── App.jsx              ← Photo grid, sidebar, lightbox, settings modal, backup UI
│   │   └── style.css
│   └── wailsjs\go\main\
│       ├── App.js               ← Wails JS bindings (hand-maintained)
│       └── App.d.ts
└── server\                      ← Go server binary (Go 1.26.1)
    ├── main.go                  ← Starts local 127.0.0.1:4242 listener + tsnet remote listener
    ├── db.go                    ← SQLite init (media + movies tables)
    ├── settings.go              ← Settings struct, load/save settings.json
    ├── indexer.go               ← ExifTool integration, filepath.Walk, DB inserts
    ├── movies.go                ← Movie filesystem scanner (no ExifTool)
    ├── takeout.go               ← Google Takeout import: extract ZIPs → GPTH → preview → copy → index
    ├── backup.go                ← Robocopy backup job, drive detection, backup_state.json persistence
    ├── thumbnailer.go           ← FFmpeg thumbnail generation, 4-worker pool, cache
    ├── watcher.go               ← fsnotify file watcher, 2-second debounce per file
    ├── broker.go                ← SSE pub/sub broker (new-file, index-done, movies-done, backup-done)
    ├── api.go                   ← HTTP handlers, AuthMiddleware, job state
    ├── paths.go                 ← AppData directory resolution
    └── go.mod                   ← Separate module: harbor/server (Go 1.26.1 + tsnet)
```

**AppData (`%AppData%\Roaming\Harbor\`)**
```
harbor.db              ← SQLite database
settings.json          ← User settings (media_folder, backup_dest, api_token, ...)
backup_state.json      ← Last successful backup timestamp
thumbnails\            ← 200px JPEG thumbnails keyed by media DB id
movie-thumbnails\      ← 200px JPEG thumbnails keyed by movies DB id
tsnet-state\           ← Tailscale auth state (persisted across restarts)
```

### API Endpoints

```
Wails UI (harbor.exe)
    │
    │  app.go spawns server.exe as child process on startup
    │  app.go kills server.exe on Wails window close
    │
    ↓ HTTP on 127.0.0.1:4242
server.exe
    │
    ├── GET  /api/media?year=N&month=N&offset=N  → paginated JSON, filtered by date
    ├── GET  /api/months                          → [{year, month, count}] for sidebar
    ├── POST /api/index?path=...                  → starts background indexing job
    ├── GET  /api/index/status                    → {status, indexed, error}
    ├── GET  /api/events                          → SSE stream (new-file, index-done, movies-done, backup-done)
    ├── GET  /api/stream/{id}                     → original file, range-request support
    ├── GET  /api/thumbnail/{id}                  → 200px JPEG
    ├── GET  /api/movies?offset=N                 → paginated movie list
    ├── POST /api/movies/index                    → scan movies_folder, background job
    ├── GET  /api/movies/index/status             → {status, indexed, error}
    ├── GET  /api/movies/stream/{id}              → original movie file, range-request support
    ├── GET  /api/movies/thumbnail/{id}           → 200px first-frame JPEG
    ├── GET  /api/settings                        → current settings JSON
    ├── POST /api/settings                        → update and persist settings
    ├── POST /api/takeout/start                   → begin Takeout import (folder of ZIPs)
    ├── GET  /api/takeout/status                  → {phase, progress, new_count, dup_count, error}
    ├── POST /api/takeout/confirm                 → approve preview, begin file copy
    ├── POST /api/takeout/cancel                  → cancel at preview, clean up temp files
    ├── POST /api/upload                          → receive single file from iOS (multipart), index immediately
    ├── GET  /api/backup/drives                   → list drive roots on this machine
    ├── GET  /api/backup/status                   → {status, last_backup_at, error}
    └── POST /api/backup/start                    → run Robocopy media_folder → backup_dest
    │
    │  also listens via tsnet on port 80
    │
    ↓ Tailscale network (no router config)
http://harbor/                      → same API, auth via Bearer token header
```

### Wails Bridge Methods (app.go)

| Method | What it does |
|---|---|
| `GetMedia(year, month, offset int)` | Proxies `GET /api/media` |
| `GetMonths()` | Proxies `GET /api/months` |
| `IndexFolder(path string)` | Proxies `POST /api/index` |
| `GetIndexStatus()` | Proxies `GET /api/index/status` |
| `GetSettings()` | Proxies `GET /api/settings` |
| `SaveSettings(json string)` | Proxies `POST /api/settings` |
| `PickFolder()` | Opens native OS folder picker dialog |
| `GetMovies(offset int)` | Proxies `GET /api/movies` |
| `IndexMovies()` | Proxies `POST /api/movies/index` |
| `GetMoviesStatus()` | Proxies `GET /api/movies/index/status` |
| `StartTakeout(folder string)` | Proxies `POST /api/takeout/start` |
| `GetTakeoutStatus()` | Proxies `GET /api/takeout/status` |
| `ConfirmTakeout()` | Proxies `POST /api/takeout/confirm` |
| `CancelTakeout()` | Proxies `POST /api/takeout/cancel` |
| `GetBackupDrives()` | Proxies `GET /api/backup/drives` |
| `GetBackupStatus()` | Proxies `GET /api/backup/status` |
| `StartBackup()` | Proxies `POST /api/backup/start` |

### Settings (AppData\Roaming\Harbor\settings.json)

```json
{
  "media_folder": "C:\\PhoneMedia",
  "movies_folder": "F:\\Movies & TV",
  "backup_dest": "F:\\iPhone-Media-Backup",
  "tools_dir": "",
  "api_token": "<32-char hex, generated on first run>"
}
```

`tools_dir` empty → falls back to `HARBOR_TOOLS` env var → then `tools\` next to server.exe.

### SQLite Schema

```sql
CREATE TABLE IF NOT EXISTS media (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    path       TEXT UNIQUE,
    filename   TEXT,
    date_taken TEXT,      -- ExifTool DateTimeOriginal: "2024:03:15 14:22:01"
    latitude   REAL,      -- GPSLatitude (0 if unavailable)
    longitude  REAL,      -- GPSLongitude (0 if unavailable)
    make       TEXT,      -- Camera make (e.g. "Apple")
    model      TEXT       -- Camera model (e.g. "iPhone 13")
)

CREATE TABLE IF NOT EXISTS movies (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    path        TEXT UNIQUE,
    filename    TEXT,
    size        INTEGER,
    modified_at TEXT
)
```

### Dev Workflow

```powershell
# Terminal 1 — build and start the server
$env:HARBOR_TOOLS = "C:\Users\James\HarborTools"
$env:HARBOR_NO_TSNET = "1"   # skip Tailscale auth in dev
cd server
go build -o server.exe .
.\server.exe

# Terminal 2 — run the Wails UI (hot-reloads React on save)
cd ..
wails dev
```

After changing server code: rebuild `server.exe` and restart it (Wails dev picks up automatically via its own process).
After changing `app.go` or `main.go`: `wails dev` recompiles Go automatically.

---

## 8. Development Environment

| Setting | Value |
|---|---|
| OS | Windows 11 |
| Machine | Daily-use PC, 12-core CPU |
| Go (Wails UI) | **1.23** — Wails v2 breaks on Go ≥ 1.25 |
| Go (server) | **1.26.1** — tsnet requires ≥ 1.26 |
| Wails | v2.11.0 |
| Node | 22.x |
| ExifTool | 13.53 |
| FFmpeg | BtbN LGPL static build |
| GPTH | Latest release — `gpth.exe` in `HarborTools\` |
| TDM-GCC | Latest — required for go-sqlite3 CGO compilation |
| Project path | `C:\Users\James\harbor` |
| AppData path | `C:\Users\James\AppData\Roaming\Harbor\` |
| Media folder | `C:\PhoneMedia` |
| Movies folder | `F:\Movies_TV\...` |
| tsnet hostname | `harbor` (reachable at `http://harbor/` on Tailscale) |

### Go Module Dependencies

**harbor/ (Wails UI)**
```
github.com/wailsapp/wails/v2
```

**harbor/server/**
```
github.com/barasher/go-exiftool   — ExifTool process management
github.com/fsnotify/fsnotify      — File system watcher
github.com/mattn/go-sqlite3       — SQLite driver (requires CGO / TDM-GCC)
tailscale.com/tsnet               — Embedded Tailscale node
```

---

## 9. Known Gotchas

- **Two Go versions required** — Wails v2 breaks on Go ≥ 1.25. tsnet requires Go ≥ 1.26. Solution: separate `go.mod` per binary.

- **go-sqlite3 requires CGO** — TDM-GCC must be on `PATH` before building.

- **Server binary must be built before `wails dev`** — the Wails UI finds `server\server.exe` relative to the working directory.

- **Wails bindings are hand-maintained** — `frontend\wailsjs\go\main\App.js` and `App.d.ts` are hand-edited. Run `wails generate module` after adding new bound methods to regenerate properly.

- **Tool path resolution order** — `settings.json` → `HARBOR_TOOLS` env var → `tools\` next to `server.exe`.

- **Robocopy exit codes** — exit codes 0–7 are success variants (0 = nothing to copy, 1 = copied OK). Exit 8+ is an error. Do not treat non-zero as failure.

- **Robocopy /E not /MIR** — `/MIR` mirrors deletions and will delete files from the backup that were removed from source. Always `/E`.

- **FFmpeg HEIC thumbnailing** — use `-filter_complex "[0:v:0]scale=200:-1[out]" -map "[out]"` instead of `-vf "scale=200:-1"`. Selects first video stream explicitly; works universally across JPEG, PNG, HEIC, MP4, MOV.

- **Movie thumbnail reliability** *(known issue)* — FFmpeg first-frame extraction is reliable for MP4/MOV but fails silently on some MKV files. Fix: probe with `-ss 00:00:05` offset, retry on failure.

- **File watcher is non-recursive** — watches the top-level media folder only. Subdirectories added after startup are not watched. Assumption: `C:\PhoneMedia` is flat (Syncthing default layout).

- **Snapchat filenames** — iOS saves Snapchat videos with colons in filenames (reserved on Windows). Must sanitise at ingestion *(not yet implemented)*.

- **Takeout duplicate detection is filename-based** — can miss duplicates with different filenames. Content-hash (xxHash of first 64 KB) would be more accurate — deferred.

- **tsnet-state is path-relative** — server sets `cmd.Dir` to its own directory. Do not move `server.exe` without also moving `tsnet-state\`.

- **iOS LAN access needs no TLS** — native iOS apps can use plain HTTP for local network via `NSAllowsLocalNetworking` in Info.plist. Self-signed certs are a browser-only concern.

---

## 10. Next Steps — Phase 2

Phase 1 Windows core is functionally complete. The installer is the only remaining Phase 1 item, but it is **not blocking iOS development** — dev builds can run directly.

### Before starting iOS app (light touch — 1–2 hours)

1. **LAN binding** — change server `localAddr` from `127.0.0.1:4242` to `0.0.0.0:4242` and add Windows Firewall rule for port 4242. Required so the iOS app can reach the server on the same WiFi network.

2. **QR pairing endpoint** — add `GET /api/pairing` returning `{url, token}`. The iOS app scans this QR on first launch to configure itself. No browser UI needed — just the JSON endpoint and a QR image endpoint.

### iOS app work (Phase 2)

3. **Xcode project** — Swift + SwiftUI, target iOS 16+.

4. **QR scanner + pairing flow** — first-launch screen, scan QR, store `serverURL` + `apiToken` in Keychain.

5. **Photo browser** — `GET /api/media` + `GET /api/thumbnail/{id}`, SwiftUI LazyVGrid, month sidebar.

6. **Video player** — AVPlayer, `GET /api/stream/{id}` with range request.

7. **Upload** — `POST /api/upload` multipart, background URLSession task.

8. **Movies tab** — `GET /api/movies` + AVPlayer streaming.

### Deferred (not blocking iOS)

- Installer (Inno Setup)
- TMDB metadata scraper
- Basic search
- Storage Saver onboarding prompt
