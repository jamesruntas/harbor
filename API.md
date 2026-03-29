# Harbor Server API

Base URL (LAN): `http://<pc-ip>:4242`
Base URL (remote): `http://harbor` *(via Tailscale — server tsnet node)*

All non-loopback requests require:
```
Authorization: Bearer <api_token>
```
The token is in `%AppData%\Harbor\settings.json` → `api_token`. Display it in Settings → Copy.

---

## Authentication

| Scenario | How |
|---|---|
| Wails desktop UI | Loopback (`127.0.0.1`) — no token required |
| iOS app (LAN) | `Authorization: Bearer <token>` header |
| iOS app (remote) | `Authorization: Bearer <token>` header over Tailscale |
| Browser (any) | ❌ Always `401 Unauthorized` — browsers cannot set `Authorization` headers on plain URL navigation. This is expected. The server has no web UI for external clients; that is the iOS app's job. |

---

## Photo Library

### `GET /api/media`

Paginated media list, newest first.

**Query params**

| Param | Type | Default | Description |
|---|---|---|---|
| `year` | int | `0` | Filter by year. `0` = no filter |
| `month` | int | `0` | Filter by month (1–12). `0` = no filter |
| `offset` | int | `0` | Pagination offset |

Page size is fixed at 100.

**Response**
```json
{
  "total": 3842,
  "items": [
    {
      "id": 1,
      "filename": "IMG_0042.HEIC",
      "date_taken": "2024:03:15 14:22:01",
      "latitude": 37.7749,
      "longitude": -122.4194,
      "make": "Apple",
      "model": "iPhone 13"
    }
  ]
}
```

`latitude` and `longitude` are omitted when 0. `make`, `model` are omitted when empty.
`date_taken` format: `"YYYY:MM:DD HH:MM:SS"` (ExifTool convention).

---

### `GET /api/months`

All year/month buckets that have media, newest first. Used to build a date sidebar.

**Response**
```json
[
  { "year": 2024, "month": 11, "count": 142 },
  { "year": 2024, "month": 10, "count": 89 }
]
```

---

### `GET /api/thumbnail/{id}`

200×200px JPEG thumbnail. `Cache-Control: immutable` — safe to cache indefinitely by ID.

**Path param:** `id` — media item ID from `/api/media`

---

### `GET /api/stream/{id}`

Original file (HEIC, JPG, PNG, MP4, MOV). Supports HTTP range requests — required for video seeking.

**Path param:** `id` — media item ID from `/api/media`

Content-Type is inferred from the file extension.

---

## iOS Upload

### `POST /api/upload`

Upload a single photo or video from the iOS app. File is saved to `media_folder`, indexed immediately, thumbnail generated.

**Request:** `multipart/form-data`, field name `file`. Max body 500 MB.

**Supported extensions:** `.jpg` `.jpeg` `.heic` `.png` `.gif` `.mp4` `.mov` `.m4v` `.avi` `.mkv` `.wmv` `.webm`

**Response**
```json
{
  "status": "ok",
  "id": 1024,
  "filename": "IMG_0099.HEIC"
}
```

**Errors**

| Status | Meaning |
|---|---|
| `400` | Missing `file` field, unsupported extension, or unparseable body |
| `500` | `media_folder` not configured, or disk write error |

---

## Movies & TV

### `GET /api/movies`

Paginated movie list, sorted alphabetically.

**Query params:** `offset` (int, default 0). Page size 100.

**Response**
```json
{
  "total": 58,
  "items": [
    {
      "id": 3,
      "filename": "Interstellar.mkv",
      "size": 8589934592,
      "modified_at": "2024-01-10T12:00:00Z"
    }
  ]
}
```

---

### `GET /api/movies/thumbnail/{id}`

200px JPEG first-frame thumbnail. Reliable for MP4/MOV; hit-and-miss on MKV.

---

### `GET /api/movies/stream/{id}`

Original movie file with range-request support.

---

## Real-time Events

### `GET /api/events`

Server-Sent Events stream. Connect once and keep open.

**Events**

| Event | When |
|---|---|
| `new-file` | A new file arrived in `media_folder` (watcher or upload) |
| `index-done` | Background photo index job completed |
| `movies-done` | Background movie index job completed |
| `backup-done` | Robocopy backup job completed |

**Usage (Swift)**
```swift
let url = URL(string: "http://\(host)/api/events")!
var request = URLRequest(url: url)
request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
let (bytes, _) = try await URLSession.shared.bytes(for: request)
for try await line in bytes.lines {
    if line.hasPrefix("data:") { /* handle event */ }
}
```

---

## Settings

### `GET /api/settings`

**Response**
```json
{
  "media_folder": "C:\\PhoneMedia",
  "movies_folder": "F:\\Movies & TV",
  "backup_dest": "E:\\Backup",
  "tools_dir": "",
  "api_token": "0e21fab4..."
}
```

The iOS app only needs `api_token` from this — retrieved once during QR pairing and stored in Keychain.

---

## Pairing (Planned — Phase 2)

### `GET /api/pairing`

Returns the LAN URL, Tailscale remote URL, and API token — encoded into the pairing QR code shown on the desktop.

**Response** *(planned)*
```json
{
  "lan_url": "http://192.168.1.50:4242",
  "tailscale_url": "http://harbor",
  "token": "0e21fab4..."
}
```

The iOS app scans the QR once, stores `lan_url`, `tailscale_url`, and `token` in Keychain. On each request it tries LAN first, falls back to Tailscale if unreachable.

---

## Not Exposed to iOS

These endpoints are Wails desktop UI only and not needed by the iOS app:

- `POST /api/index` — desktop-triggered re-index
- `GET /api/index/status`
- `POST /api/takeout/start` / `status` / `confirm` / `cancel` — Google Takeout import
- `GET /api/backup/drives` / `status` / `POST /api/backup/start` — backup management
- `POST /api/settings` — settings write
