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
10. [Next Steps — Phase 1](#10-next-steps--phase-1)

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
| File sync | Custom HTTPS (V1) |
| Remote access | tsnet via gomobile |
| Background sync | Network Extension |

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
- [ ] Onboarding flow wireframe *(Phase 1)*
- [ ] Drive detection logic spec *(Phase 1)*

---

### Phase 1 — Windows Core *(in progress)*

**Success criterion:** non-technical user installs and browses their media library within 10 minutes.

#### Media Engine
- [x] ExifTool integration — extract date, GPS, make, model per file
- [x] SQLite schema — media table with path, filename, date_taken, latitude, longitude, make, model
- [x] Async indexing with job state — `POST /api/index` returns immediately, poll `/api/index/status`
- [x] Case-insensitive extension matching — handles `.MOV`, `.HEIC`, `.JPG` from iPhone
- [x] Silent drop fix — ExifTool errors log and insert with empty metadata rather than skipping
- [x] Video date fix — falls back to `CreateDate` when `DateTimeOriginal` absent (MP4/MOV)
- [x] FFmpeg thumbnail generation — 200px JPEG, 4-worker pool, skip-if-cached, async per file
- [x] Thumbnail cache — `server/thumbnails/{id}.jpg`, keyed by DB id
- [x] File watcher (fsnotify) — auto-index on new files arriving in the configured media folder
- [x] SSE auto-refresh — EventSource in frontend, broker publishes `new-file` / `index-done`
- [x] Configurable tool paths — `HARBOR_TOOLS` env var or `settings.json`, no more hardcoded user paths
- [ ] Preview size thumbnail (800px) — deferred until lightbox view
- [x] FFmpeg video streaming — `GET /api/stream/{id}` and `GET /api/movies/stream/{id}`, range-request support for seeking
- [ ] Movie thumbnail reliability — FFmpeg first-frame generation works for MP4/MOV but is hit-and-miss on MKV and other containers; needs investigation and fallback handling *(known issue)*
- [ ] TMDB metadata scraper for movies and TV

#### Google Takeout Importer
- [x] ZIP extraction — extracts all ZIPs in a selected folder into a temp workspace
- [x] GPTH integration — runs `gpth.exe --albums nothing` to reconcile JSON sidecars and restore EXIF dates/GPS
- [x] Duplicate detection — filename + DB lookup; flags files already in the library *(TODO: content-hash for higher accuracy)*
- [x] Preview before import — shows new count vs duplicate count, user confirms or cancels
- [x] Non-destructive — originals never modified; new files copied flat into `media_folder`, temp workspace cleaned up
- [x] Indexed automatically after copy — ExifTool runs on imported files, thumbnails generated
- [x] SSE refresh — grid updates automatically when import completes
- [ ] In-app guided Takeout request walkthrough (link to takeout.google.com with instructions)

#### Backup Story
- [ ] External drive detection on startup
- [ ] One-click backup destination setup
- [ ] Background Robocopy-equivalent sync
- [ ] Confidence dashboard — "X copies · last backed up Y ago"
- [ ] Single-drive persistent warning
- [ ] Storage Saver prompt — Google free tier as offsite redundancy

#### Dashboard UI (Wails + React)
- [x] Photo grid with real thumbnails (FFmpeg-generated JPEG)
- [x] Video cards — play icon overlay, first-frame thumbnail
- [x] Lightbox view — click to open full-size photo or video, keyboard navigation (←/→/Esc)
- [x] Paginated media list (100 per page, "Load more N remaining")
- [x] Index button with live file counter ("Indexing… 42 files")
- [x] Index button uses `media_folder` from settings — no hardcoded path
- [x] Error state clears automatically on server recovery
- [x] Color palette — `#222222 / #1c5d99 / #bbcde5 / #fbfaef / #23967f`
- [x] Date-based browsing — year/month sidebar, click to filter grid
- [x] Settings panel — media folder + tools directory, native OS folder picker
- [ ] Basic search (filename, date range)
- [x] Movies & TV tab — separate from Phone Media, folder configured via Settings, scan + stream + lightbox player
- [ ] Backup status widget

#### Installer
- [ ] Inno Setup installer
- [ ] ExifTool bundled in `tools\` alongside server.exe
- [ ] FFmpeg bundled in `tools\` alongside server.exe
- [ ] First-run onboarding wizard

#### Remote Access *(pulled forward from Phase 2)*
- [x] tsnet integration — embedded Tailscale node in server binary
- [x] Zero-config remote access — reachable at `http://harbor/` with no router config
- [x] Auth state persisted in `server\tsnet-state\` — no re-auth on restart
- [ ] QR code generation for iOS pairing
- [ ] Auth token / API key for iOS requests

---

### Phase 2 — Remote Access + iOS MVP *(2–3 months)*

**Success criterion:** user views home photo library and streams a movie from mobile data.

#### Remote Access (Windows)
- [x] tsnet integration — pulled forward to Phase 1 (see above)
- [x] Zero-config remote access — pulled forward to Phase 1
- [ ] QR code generation for iOS pairing
- [ ] Auth token generation and management
- [ ] HTTPS endpoint for iOS API

#### iOS App
- [ ] Xcode project setup, Swift + SwiftUI
- [ ] QR code pairing flow on first launch
- [ ] Photo library browser (SwiftUI grid)
- [ ] Video player (AVPlayer)
- [ ] Photo upload iOS → Windows via HTTPS
- [ ] Background upload via Network Extension (VPN entitlement required)
- [ ] Upload queue with retry logic
- [ ] Progress indicators

#### Google Positioning
- [ ] Storage Saver onboarding flow
- [ ] Side-by-side mode (Harbor + Google running in parallel)
- [ ] "Keep Google, stop paying" framing in UI copy
- [ ] No deletion pressure in any UI surface

---

### Phase 3 — Polish + iCloud + Sync *(2–3 months)*

**Success criterion:** experience is indistinguishable from a commercial product.

#### iCloud Importer
- [ ] privacy.apple.com export guide (in-app walkthrough)
- [ ] ZIP batch download handling (1,000-photo Apple limit workaround)
- [ ] Metadata parsing — Apple's export format differs from Google's
- [ ] Album structure preservation
- [ ] Windows iCloud-user specific UX (no Apple Photos app available)

#### Product Polish
- [ ] Face detection and grouping (on-device ML)
- [ ] Search by date, location, content tags
- [ ] Shared albums
- [ ] Auto-update mechanism (Sparkle-equivalent for Windows)
- [ ] Onboarding refinement based on user feedback
- [ ] Performance optimisation for large libraries (100k+ photos)
- [ ] Memory and disk usage profiling

#### Sync
- [ ] Embedded Syncthing for automatic background sync (replaces custom HTTPS V1)
- [ ] Conflict resolution UI
- [ ] Sync status in dashboard

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
├── harbor.db                    ← SQLite DB (written by server, read via server API)
├── frontend\
│   ├── src\
│   │   ├── App.jsx              ← Photo grid, sidebar, lightbox, settings modal
│   │   └── style.css
│   └── wailsjs\go\main\
│       ├── App.js               ← Wails JS bindings (hand-maintained)
│       └── App.d.ts
└── server\                      ← Go server binary (Go 1.26.1)
    ├── main.go                  ← Starts local :4242 listener + tsnet remote listener
    ├── db.go                    ← SQLite init (media + movies tables)
    ├── settings.go              ← Settings struct, load/save settings.json
    ├── indexer.go               ← ExifTool integration, filepath.Walk, DB inserts
    ├── movies.go                ← Movie filesystem scanner (no ExifTool)
    ├── takeout.go               ← Google Takeout import: extract ZIPs → GPTH → preview → copy → index
    ├── thumbnailer.go           ← FFmpeg thumbnail generation, 4-worker pool, cache
    ├── watcher.go               ← fsnotify file watcher, 2-second debounce per file
    ├── broker.go                ← SSE pub/sub broker (new-file, index-done, movies-done events)
    ├── api.go                   ← HTTP handlers + job state for indexing, movies, takeout
    ├── go.mod                   ← Separate module: harbor/server (Go 1.26.1 + tsnet)
    ├── thumbnails\              ← Cached media thumbnail JPEGs keyed by DB id
    ├── movie-thumbnails\        ← Cached movie thumbnail JPEGs (separate to avoid ID collisions)
    └── tsnet-state\             ← Tailscale auth state (persisted across runs)
        └── tailscaled.state
```

### How the two binaries communicate

```
Wails UI (harbor.exe)
    │
    │  app.go spawns server.exe as child process on startup
    │  app.go kills server.exe on Wails window close
    │
    ↓ HTTP on 127.0.0.1:4242
server.exe
    ├── GET  /api/media?year=N&month=N&offset=N  → paginated JSON, filtered by date
    ├── GET  /api/months                          → [{year, month, count}] for sidebar
    ├── POST /api/index (path=...)                → starts background indexing job
    ├── GET  /api/index/status                    → {status, indexed, error}
    ├── GET  /api/events                          → SSE stream (new-file, index-done)
    ├── GET  /api/stream/{id}                     → original file, range-request support
    ├── GET  /api/thumbnail/{id}                  → 200px JPEG, Cache-Control immutable
    ├── GET  /api/movies                          → paginated movie list
    ├── POST /api/movies/index                    → scan movies_folder, background job
    ├── GET  /api/movies/index/status             → {status, indexed, error}
    ├── GET  /api/movies/stream/{id}              → original movie file, range-request support
    └── GET  /api/movies/thumbnail/{id}           → 200px first-frame JPEG (hit-and-miss on some containers)
    ├── GET  /api/settings                        → current settings JSON
    ├── POST /api/settings                        → update and persist settings
    ├── POST /api/takeout/start                   → begin Takeout import (folder of ZIPs)
    ├── GET  /api/takeout/status                  → {phase, progress, new_count, dup_count, error}
    ├── POST /api/takeout/confirm                 → approve preview, begin file copy
    └── POST /api/takeout/cancel                  → cancel at preview, clean up temp files
    │
    │  also listens via tsnet on port 80
    │
    ↓ Tailscale network (no router config)
http://harbor/                      → same API endpoints, accessible remotely
```

### Wails binding methods (app.go)

| Method | What it does |
|---|---|
| `GetMedia(year, month, offset int)` | Proxies `GET /api/media`, returns JSON page |
| `GetMonths()` | Proxies `GET /api/months`, returns [{year,month,count}] |
| `IndexFolder(path string)` | Proxies `POST /api/index`, returns immediately |
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

### Settings (settings.json next to server.exe)

```json
{
  "media_folder": "C:\\PhoneMedia",
  "movies_folder": "F:\\Movies & TV",
  "tools_dir": ""
}
```

`tools_dir` empty → falls back to `HARBOR_TOOLS` env var → then `tools\` next to server.exe.
Changes to `tools_dir` take effect on server restart. Changes to `media_folder` and `movies_folder` take effect on the next scan.

### SQLite schema

```sql
CREATE TABLE IF NOT EXISTS movies (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    path        TEXT UNIQUE,
    filename    TEXT,
    size        INTEGER,
    modified_at TEXT
)

CREATE TABLE IF NOT EXISTS media (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    path       TEXT UNIQUE,
    filename   TEXT,
    date_taken TEXT,      -- ExifTool DateTimeOriginal, format: "2024:03:15 14:22:01"
    latitude   REAL,      -- GPSLatitude (0 if unavailable)
    longitude  REAL,      -- GPSLongitude (0 if unavailable)
    make       TEXT,      -- Camera make (e.g. "Apple")
    model      TEXT       -- Camera model (e.g. "iPhone 13")
)
```

### Dev workflow

```powershell
# Set tool paths for dev (skip if tools\ exists next to server.exe)
$env:HARBOR_TOOLS = "C:\Users\James\HarborTools"

# Terminal 1 — build and start the server
cd server
go build -o server.exe .
.\server.exe

# Terminal 2 — run the Wails UI (hot-reloads React on save)
cd ..
wails dev
```

After changing server code: stop `wails dev`, rebuild `server.exe`, restart `wails dev`.
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
| ExifTool | 13.53 — path resolved via `HARBOR_TOOLS` env or `settings.json` |
| FFmpeg | BtbN LGPL static build — path resolved via `HARBOR_TOOLS` env or `settings.json` |
| GPTH | Latest release — `gpth.exe` in `HarborTools\`, used for Google Takeout import |
| TDM-GCC | Latest — required for go-sqlite3 CGO compilation |
| DB Browser for SQLite | Latest — inspect `harbor.db` |
| Project path | `C:\Users\James\harbor` |
| DB path | `C:\Users\James\harbor\harbor.db` |
| Media folder | `C:\PhoneMedia` (iPhone photos synced via Syncthing) |
| Movies folder | `F:\Movies & TV` |
| Tailscale PC IP | `100.92.73.84` |
| Tailscale iPhone IP | `100.75.158.126` |
| tsnet hostname | `harbor` (reachable at `http://harbor/` on Tailscale) |

### Go Module Dependencies

**harbor/ (Wails UI)**
```
github.com/wailsapp/wails/v2   — Desktop app framework
```

**harbor/server/**
```
github.com/barasher/go-exiftool   — ExifTool process management
github.com/fsnotify/fsnotify      — File system watcher
github.com/mattn/go-sqlite3       — SQLite driver (requires CGO / TDM-GCC)
tailscale.com/tsnet               — Embedded Tailscale node for zero-config remote access
```

---

## 9. Known Gotchas

- **Two Go versions required** — Wails v2 breaks on Go ≥ 1.25 (bindings generation fails: `%1 is not a valid Win32 application`). tsnet requires Go ≥ 1.26. Solution: separate `go.mod` per binary. The Wails UI is pinned to Go 1.23; the server uses Go 1.26.1.

- **go-sqlite3 requires CGO** — TDM-GCC must be installed and on `PATH` before `go get github.com/mattn/go-sqlite3`. Both binaries use SQLite, both need TDM-GCC at compile time.

- **Tool path resolution order** — `settings.json` → `HARBOR_TOOLS` env var → `tools\` next to `server.exe`. In dev, set `$env:HARBOR_TOOLS = "C:\Users\James\HarborTools"`. In production, the installer places tools in `tools\`.

- **Server binary must be built before `wails dev`** — the Wails UI finds `server\server.exe` relative to the working directory. Run `cd server && go build -o server.exe .` first.

- **Wails bindings are hand-maintained** — `frontend\wailsjs\go\main\App.js` and `App.d.ts` are currently hand-edited. Run `wails generate module` after adding new bound methods to regenerate them properly.

- **tsnet-state path is relative** — `server\main.go` uses `tsnet-state` as a relative directory. The server sets `cmd.Dir` to the binary's own directory at launch, so this resolves correctly. Do not move the server binary without also moving `tsnet-state\`.

- **Tailscale hostname changed** — tsnet hostname is now `harbor` (was `homestream`). If you have an existing tsnet-state from before this rename, delete `server\tsnet-state\` and re-authenticate to pick up the new hostname.

- **DB renamed** — database is now `harbor.db` (was `homestream.db`). Rename the existing file or re-index.

- **FFmpeg HEIC thumbnailing** — `-vf "scale=200:-1"` fails on HEIC due to multiple embedded image streams. Use `-filter_complex "[0:v:0]scale=200:-1[out]" -map "[out]"` instead. This selects the first video stream explicitly and works universally across JPEG, PNG, HEIC, MP4, and MOV.

- **GPTH must be in tools directory** — `gpth.exe` is a required dependency for Google Takeout import. Download from the [GPTH releases page](https://github.com/TheLastGimbus/GooglePhotosTakeoutHelper/releases) and place alongside `exiftool.exe` and `ffmpeg.exe`. The server checks for its existence before starting an import and returns a clear error if missing.

- **Takeout duplicate detection is filename-based** *(known limitation)* — duplicates are identified by filename match against existing DB rows and the media folder on disk. This can miss duplicates with different filenames (e.g. Google renaming during export) and falsely flag unrelated files with the same name. A content-hash approach (xxHash of first 64 KB) would be more accurate — deferred.

- **Movie thumbnail reliability** *(known issue)* — FFmpeg first-frame extraction is reliable for MP4 and MOV but fails silently on some MKV files and other containers. Likely causes: codec not in FFmpeg LGPL build, no keyframe at position 0, or container-specific seeking issues. Fix needed: probe with `-ss 00:00:05` offset, retry on failure, and verify the FFmpeg build includes the required decoders.

- **File watcher is non-recursive** — `server\watcher.go` watches the top-level media folder only. Subdirectories added after startup are not watched. Phase 1 assumption: `C:\PhoneMedia` is flat (Syncthing default layout).

- **Snapchat filenames** — iOS saves Snapchat videos with colons in filenames, which are reserved on Windows. Harbor must sanitise filenames at ingestion (not yet implemented).

- **Incomplete transfers** — iOS can transfer incomplete files if a photo is still processing after capture. Harbor must handle partial files gracefully (not yet implemented).

---

## 10. Next Steps — Phase 1

### Remaining items (priority order)

1. **FFmpeg video streaming** — `GET /api/stream/{id}` already serves original files with range-request support. Frontend lightbox uses `<video>` for `.mp4`/`.mov`. Clicking a video card plays it inline in the lightbox. ✅ Done via the stream endpoint.

2. **Basic search** — filename substring filter + date range picker in the header. Server-side `LIKE` query on filename and `BETWEEN` on date_taken.

3. **Google Takeout importer** — ZIP extraction, JSON sidecar reconciliation, preview before import.

4. **Backup story** — external drive detection, one-click setup, confidence dashboard.

5. **Installer** — Inno Setup, bundle ExifTool + FFmpeg in `tools\`, first-run onboarding wizard.
