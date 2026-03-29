import { useState, useEffect, useRef, useCallback } from 'react'
import {
  GetMedia, GetMonths, GetSettings, SaveSettings,
  IndexFolder, GetIndexStatus, PickFolder,
  GetMovies, IndexMovies, GetMoviesStatus,
  StartTakeout, GetTakeoutStatus, ConfirmTakeout, CancelTakeout,
  GetBackupDrives, GetBackupStatus, StartBackup,
} from '../wailsjs/go/main/App'

const MONTH_NAMES = [
  '', 'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
]

function formatDate(raw) {
  if (!raw) return '—'
  const normalized = raw.replace(/^(\d{4}):(\d{2}):(\d{2})/, '$1-$2-$3')
  const d = new Date(normalized)
  if (isNaN(d)) return raw
  return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
}

function formatSize(bytes) {
  if (bytes >= 1e9) return (bytes / 1e9).toFixed(1) + ' GB'
  if (bytes >= 1e6) return (bytes / 1e6).toFixed(0) + ' MB'
  return (bytes / 1e3).toFixed(0) + ' KB'
}

function formatTimeAgo(isoStr) {
  if (!isoStr) return 'never'
  const diff = Date.now() - new Date(isoStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 2) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  return `${Math.floor(hrs / 24)}d ago`
}

function stripExt(filename) {
  return filename.replace(/\.[^.]+$/, '')
}

function isVideo(filename) {
  return /\.(mp4|mov)$/i.test(filename)
}

// ── Lightbox ──────────────────────────────────────────────────────────────────

function Lightbox({ item, items, streamUrl, onClose, onPrev, onNext }) {
  useEffect(() => {
    function onKey(e) {
      if (e.key === 'Escape') onClose()
      if (e.key === 'ArrowLeft') onPrev()
      if (e.key === 'ArrowRight') onNext()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose, onPrev, onNext])

  const src = streamUrl(item.id)
  const idx = items.indexOf(item)
  const forceVideo = !('date_taken' in item) // movies always video

  return (
    <div className="lb-overlay" onClick={onClose}>
      <button className="lb-close" onClick={onClose}>✕</button>
      {idx > 0 && (
        <button className="lb-nav lb-prev" onClick={e => { e.stopPropagation(); onPrev() }}>‹</button>
      )}
      {idx < items.length - 1 && (
        <button className="lb-nav lb-next" onClick={e => { e.stopPropagation(); onNext() }}>›</button>
      )}
      <div className="lb-content" onClick={e => e.stopPropagation()}>
        {(forceVideo || isVideo(item.filename)) ? (
          <video key={item.id} className="lb-media" controls autoPlay src={src} />
        ) : (
          <img className="lb-media" src={src} alt={item.filename} />
        )}
        <div className="lb-info">
          <span className="lb-name">{item.filename}</span>
          {item.date_taken && <span className="lb-date">{formatDate(item.date_taken)}</span>}
          {item.model && <span className="lb-camera">{item.model}</span>}
          {item.size && <span className="lb-date">{formatSize(item.size)}</span>}
        </div>
      </div>
    </div>
  )
}

// ── Settings modal ─────────────────────────────────────────────────────────────

function SettingsModal({ settings, onSave, onClose }) {
  const [mediaFolder, setMediaFolder] = useState(settings.media_folder || '')
  const [moviesFolder, setMoviesFolder] = useState(settings.movies_folder || '')
  const [backupDest, setBackupDest]   = useState(settings.backup_dest || '')
  const [drives, setDrives]           = useState([])
  const [toolsDir, setToolsDir]       = useState(settings.tools_dir || '')
  const [saving, setSaving]           = useState(false)
  const [error, setError]             = useState(null)
  const [copied, setCopied]           = useState(false)
  const token = settings.api_token || ''

  useEffect(() => {
    GetBackupDrives().then(raw => {
      try { setDrives(JSON.parse(raw)) } catch {}
    })
  }, [])

  function copyToken() {
    navigator.clipboard.writeText(token).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  async function pick(setter) {
    const p = await PickFolder()
    if (p) setter(p)
  }

  async function handleSave() {
    if (!mediaFolder.trim()) { setError('Media folder is required.'); return }
    setSaving(true)
    setError(null)
    try {
      const raw = await SaveSettings(JSON.stringify({
        media_folder: mediaFolder,
        movies_folder: moviesFolder,
        backup_dest: backupDest,
        tools_dir: toolsDir,
        api_token: token,
      }))
      const result = JSON.parse(raw)
      if (result.error) { setError(result.error); return }
      onSave(result)
    } catch (e) {
      setError(String(e))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <span className="modal-title">Settings</span>
          <button className="modal-close" onClick={onClose}>✕</button>
        </div>

        <div className="modal-body">
          <label className="setting-label">Phone Media Folder</label>
          <div className="setting-row">
            <input className="setting-input" value={mediaFolder}
              onChange={e => setMediaFolder(e.target.value)} placeholder="C:\PhoneMedia" />
            <button className="setting-pick" onClick={() => pick(setMediaFolder)}>Browse</button>
          </div>

          <label className="setting-label" style={{ marginTop: 16 }}>Movies & TV Folder</label>
          <div className="setting-row">
            <input className="setting-input" value={moviesFolder}
              onChange={e => setMoviesFolder(e.target.value)} placeholder="F:\Movies & TV" />
            <button className="setting-pick" onClick={() => pick(setMoviesFolder)}>Browse</button>
          </div>

          <label className="setting-label" style={{ marginTop: 16 }}>Backup Destination</label>
          <div className="setting-row">
            <input className="setting-input" value={backupDest}
              onChange={e => setBackupDest(e.target.value)} placeholder="E:\ or F:\Backup" />
            <button className="setting-pick" onClick={() => pick(setBackupDest)}>Browse</button>
          </div>
          {drives.length > 1 && (
            <div className="backup-drives">
              {drives.filter(d => d !== (mediaFolder.slice(0,3) || '')).map(d => (
                <button key={d} className={`drive-chip ${backupDest.startsWith(d) ? 'active' : ''}`}
                  onClick={() => setBackupDest(d)}>
                  {d}
                </button>
              ))}
            </div>
          )}
          <p className="setting-hint">Harbor will Robocopy your phone media here on demand. Use an external drive for best protection.</p>

          <label className="setting-label" style={{ marginTop: 16 }}>Tools Directory</label>
          <div className="setting-row">
            <input className="setting-input" value={toolsDir}
              onChange={e => setToolsDir(e.target.value)}
              placeholder="Leave blank to use tools\ next to server.exe" />
            <button className="setting-pick" onClick={() => pick(setToolsDir)}>Browse</button>
          </div>
          <p className="setting-hint">ExifTool and FFmpeg must be in this folder. Changes take effect on restart.</p>

          <label className="setting-label" style={{ marginTop: 16 }}>Remote Access Token</label>
          <div className="setting-row">
            <input className="setting-input" value={token} readOnly style={{ fontFamily: 'monospace', fontSize: 12 }} />
            <button className="setting-pick" onClick={copyToken}>{copied ? 'Copied!' : 'Copy'}</button>
          </div>
          <p className="setting-hint">Use this token to authenticate the iOS app over Tailscale.</p>

          {error && <p className="setting-error">{error}</p>}
        </div>

        <div className="modal-footer">
          <button className="index-btn secondary" onClick={onClose}>Cancel</button>
          <button className="index-btn" onClick={handleSave} disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ── Sidebar ────────────────────────────────────────────────────────────────────

function Sidebar({ months, filter, onFilter }) {
  const years = []
  const byYear = {}
  for (const b of months) {
    if (!byYear[b.year]) { byYear[b.year] = []; years.push(b.year) }
    byYear[b.year].push(b)
  }

  const [collapsed, setCollapsed] = useState({})
  const toggle = year => setCollapsed(c => ({ ...c, [year]: !c[year] }))
  const totalCount = months.reduce((s, b) => s + b.count, 0)

  return (
    <nav className="sidebar">
      <button
        className={`sidebar-all ${!filter.year ? 'active' : ''}`}
        onClick={() => onFilter({ year: 0, month: 0 })}
      >
        All Photos
        <span className="sidebar-count">{totalCount}</span>
      </button>
      <div className="sidebar-divider" />
      {years.map(year => {
        const isOpen = !collapsed[year]
        const yearActive = filter.year === year && !filter.month
        return (
          <div key={year} className="sidebar-year-group">
            <button
              className={`sidebar-year ${yearActive ? 'active' : ''}`}
              onClick={() => { toggle(year); onFilter({ year, month: 0 }) }}
            >
              <span className="sidebar-chevron">{isOpen ? '▾' : '▸'}</span>
              {year}
            </button>
            {isOpen && byYear[year].map(b => {
              const monthActive = filter.year === year && filter.month === b.month
              return (
                <button
                  key={b.month}
                  className={`sidebar-month ${monthActive ? 'active' : ''}`}
                  onClick={() => onFilter({ year, month: b.month })}
                >
                  {MONTH_NAMES[b.month]}
                  <span className="sidebar-count">{b.count}</span>
                </button>
              )
            })}
          </div>
        )
      })}
    </nav>
  )
}

// ── Pairing Modal ─────────────────────────────────────────────────────────────

function PairingModal({ onClose }) {
  const [info, setInfo] = useState(null)

  useEffect(() => {
    GetPairingInfo().then(raw => {
      try { setInfo(JSON.parse(raw)) } catch {}
    })
  }, [])

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <span className="modal-title">Pair iPhone</span>
          <button className="modal-close" onClick={onClose}>✕</button>
        </div>
        <div className="modal-body" style={{ textAlign: 'center' }}>
          {!info && <p style={{ color: '#888' }}>Loading…</p>}
          {info?.error && <p className="setting-error">{info.error}</p>}
          {info && !info.error && (
            <>
              <p style={{ color: '#aaa', fontSize: 13, marginBottom: 20, lineHeight: 1.5 }}>
                Make sure your iPhone is on the same Wi-Fi network, then scan this QR code in Safari.
                When prompted, tap <strong style={{ color: '#eee' }}>Visit Website</strong> to trust the local security certificate (one-time only).
              </p>
              <img
                src="http://127.0.0.1:4242/api/pairing/qr"
                alt="Pairing QR code"
                style={{ width: 200, height: 200, display: 'block', margin: '0 auto 20px', imageRendering: 'pixelated', borderRadius: 8 }}
              />
              <p style={{ fontSize: 11, color: '#555', wordBreak: 'break-all', fontFamily: 'monospace', padding: '0 8px' }}>
                {info.url}
              </p>
            </>
          )}
        </div>
        <div className="modal-footer">
          <button className="index-btn" onClick={onClose}>Done</button>
        </div>
      </div>
    </div>
  )
}

// ── Takeout Modal ──────────────────────────────────────────────────────────────

function TakeoutModal({ onClose }) {
  const [status, setStatus] = useState(null)
  const pollRef = useRef(null)

  async function pickAndStart() {
    const folder = await PickFolder()
    if (!folder) return

    const raw = await StartTakeout(folder)
    const res = JSON.parse(raw)
    if (res.error) { setStatus({ phase: 'error', error: res.error }); return }
    startPolling()
  }

  function startPolling() {
    if (pollRef.current) clearInterval(pollRef.current)
    pollRef.current = setInterval(async () => {
      const raw = await GetTakeoutStatus()
      const s = JSON.parse(raw)
      setStatus(s)
      if (s.phase === 'done' || s.phase === 'error' || s.phase === 'idle') {
        clearInterval(pollRef.current)
        pollRef.current = null
      }
    }, 800)
  }

  useEffect(() => () => { if (pollRef.current) clearInterval(pollRef.current) }, [])

  async function handleConfirm() {
    await ConfirmTakeout()
    startPolling()
  }

  async function handleCancel() {
    await CancelTakeout()
    clearInterval(pollRef.current)
    pollRef.current = null
    onClose()
  }

  const phase = status?.phase

  return (
    <div className="modal-overlay" onClick={phase === 'preview' ? undefined : onClose}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <span className="modal-title">Import from Google Takeout</span>
          {(!phase || phase === 'done' || phase === 'error') && (
            <button className="modal-close" onClick={onClose}>✕</button>
          )}
        </div>

        <div className="modal-body">

          {/* Initial state — no job started yet */}
          {!phase && (
            <div className="takeout-start">
              <p className="takeout-desc">
                Select the folder containing your Google Takeout ZIP files.
                Harbor will extract them, reconcile metadata using GPTH, and
                import new photos into your Phone Media library.
              </p>
              <button className="index-btn" style={{ marginTop: 16 }} onClick={pickAndStart}>
                Select Takeout Folder
              </button>
            </div>
          )}

          {/* Extracting / processing */}
          {(phase === 'extracting' || phase === 'processing') && (
            <div className="takeout-progress">
              <div className="spinner" />
              <p className="takeout-phase-label">
                {phase === 'extracting' ? 'Extracting ZIPs' : 'Processing with GPTH'}
              </p>
              <p className="takeout-message">{status.message}</p>
              {phase === 'extracting' && status.total > 0 && (
                <div className="progress-bar">
                  <div className="progress-fill" style={{ width: `${(status.progress / status.total) * 100}%` }} />
                </div>
              )}
            </div>
          )}

          {/* Preview — waiting for confirmation */}
          {phase === 'preview' && (
            <div className="takeout-preview">
              <div className="preview-stat new">
                <span className="preview-num">{status.new_count}</span>
                <span className="preview-label">new photos ready to import</span>
              </div>
              {status.dup_count > 0 && (
                <div className="preview-stat dup">
                  <span className="preview-num">{status.dup_count}</span>
                  <span className="preview-label">already in your library — skipped</span>
                </div>
              )}
            </div>
          )}

          {/* Importing */}
          {phase === 'importing' && (
            <div className="takeout-progress">
              <div className="spinner" />
              <p className="takeout-phase-label">Importing</p>
              <p className="takeout-message">{status.message}</p>
              {status.total > 0 && (
                <div className="progress-bar">
                  <div className="progress-fill" style={{ width: `${(status.progress / status.total) * 100}%` }} />
                </div>
              )}
            </div>
          )}

          {/* Done */}
          {phase === 'done' && (
            <p className="takeout-done">{status.message}</p>
          )}

          {/* Error */}
          {phase === 'error' && (
            <p className="setting-error">{status.error}</p>
          )}

        </div>

        {phase === 'preview' && (
          <div className="modal-footer">
            <button className="index-btn secondary" onClick={handleCancel}>Cancel</button>
            <button className="index-btn" onClick={handleConfirm} disabled={status.new_count === 0}>
              Import {status.new_count} photos
            </button>
          </div>
        )}

        {phase === 'done' && (
          <div className="modal-footer">
            <button className="index-btn" onClick={onClose}>Done</button>
          </div>
        )}
      </div>
    </div>
  )
}

// ── Movies Tab ─────────────────────────────────────────────────────────────────

function MoviesTab({ settings, onOpenSettings }) {
  const [movies, setMovies]         = useState([])
  const [total, setTotal]           = useState(0)
  const [loading, setLoading]       = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [indexing, setIndexing]     = useState(false)
  const [indexCount, setIndexCount] = useState(0)
  const [selected, setSelected]     = useState(null)
  const pollRef = useRef(null)

  const hasFolder = !!settings?.movies_folder

  async function loadMovies() {
    if (!hasFolder) { setLoading(false); return }
    try {
      const raw = await GetMovies(0)
      const page = JSON.parse(raw)
      setMovies(page.items)
      setTotal(page.total)
    } catch {}
    setLoading(false)
  }

  async function loadMore() {
    setLoadingMore(true)
    try {
      const raw = await GetMovies(movies.length)
      const page = JSON.parse(raw)
      setMovies(prev => [...prev, ...page.items])
      setTotal(page.total)
    } finally {
      setLoadingMore(false)
    }
  }

  useEffect(() => { loadMovies() }, [hasFolder])

  useEffect(() => {
    const es = new EventSource('http://127.0.0.1:4242/api/events')
    es.onmessage = e => { if (e.data === 'movies-done') loadMovies() }
    return () => es.close()
  }, [])

  async function handleIndex() {
    if (pollRef.current) clearInterval(pollRef.current)
    setIndexing(true)
    setIndexCount(0)
    await IndexMovies()

    pollRef.current = setInterval(async () => {
      const raw = await GetMoviesStatus()
      const s = JSON.parse(raw)
      setIndexCount(s.indexed)
      if (s.status === 'done' || s.status === 'error') {
        clearInterval(pollRef.current)
        pollRef.current = null
        setIndexing(false)
        loadMovies()
      }
    }, 1000)
  }

  const selectedIdx = selected ? movies.indexOf(selected) : -1
  const handlePrev = useCallback(() => {
    if (selectedIdx > 0) setSelected(movies[selectedIdx - 1])
  }, [selectedIdx, movies])
  const handleNext = useCallback(() => {
    if (selectedIdx < movies.length - 1) setSelected(movies[selectedIdx + 1])
  }, [selectedIdx, movies])

  const streamUrl = id => `http://127.0.0.1:4242/api/movies/stream/${id}`

  // No folder configured
  if (!hasFolder) {
    return (
      <div className="empty-state">
        <p className="empty-icon">🎬</p>
        <p className="empty-title">No Movies folder set</p>
        <p className="empty-sub">Set the path to your Movies &amp; TV folder in Settings.</p>
        <button className="index-btn" style={{ marginTop: 16 }} onClick={onOpenSettings}>
          Open Settings
        </button>
      </div>
    )
  }

  return (
    <div className="movies-pane">
      <div className="movies-toolbar">
        <span className="count">
          {!loading && `${movies.length} of ${total} titles`}
        </span>
        <button className="index-btn" onClick={handleIndex} disabled={indexing}>
          {indexing ? `Scanning… ${indexCount} files` : 'Scan Movies & TV'}
        </button>
      </div>

      {loading && <p className="status">Loading…</p>}
      {!loading && movies.length === 0 && (
        <div className="empty-state">
          <p className="empty-icon">🎬</p>
          <p className="empty-title">No titles found</p>
          <p className="empty-sub">Click Scan to index your Movies &amp; TV folder.</p>
        </div>
      )}
      {!loading && movies.length > 0 && (
        <>
          <div className="movie-grid">
            {movies.map(m => (
              <div key={m.id} className="movie-card" onClick={() => setSelected(m)}>
                <div className="movie-thumb">
                  <img
                    src={`http://127.0.0.1:4242/api/movies/thumbnail/${m.id}`}
                    alt={m.filename}
                    onError={e => { e.currentTarget.style.display = 'none' }}
                  />
                  <span className="play-icon">▶</span>
                </div>
                <div className="movie-info">
                  <p className="movie-title" title={m.filename}>{stripExt(m.filename)}</p>
                  <p className="movie-meta">{formatSize(m.size)}</p>
                </div>
              </div>
            ))}
          </div>
          {movies.length < total && (
            <button
              className="index-btn"
              onClick={loadMore}
              disabled={loadingMore}
              style={{ margin: '20px auto', display: 'block' }}
            >
              {loadingMore ? 'Loading…' : `Load more (${total - movies.length} remaining)`}
            </button>
          )}
        </>
      )}

      {selected && (
        <Lightbox
          item={selected}
          items={movies}
          streamUrl={streamUrl}
          onClose={() => setSelected(null)}
          onPrev={handlePrev}
          onNext={handleNext}
        />
      )}
    </div>
  )
}

// ── App ────────────────────────────────────────────────────────────────────────

export default function App() {
  const [tab, setTab]               = useState('media')
  const [items, setItems]           = useState([])
  const [total, setTotal]           = useState(0)
  const [loading, setLoading]       = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [indexing, setIndexing]     = useState(false)
  const [indexCount, setIndexCount] = useState(0)
  const [error, setError]           = useState(null)
  const [selected, setSelected]     = useState(null)
  const [months, setMonths]         = useState([])
  const [filter, setFilter]         = useState({ year: 0, month: 0 })
  const [settings, setSettings]     = useState(null)
  const [showSettings, setShowSettings] = useState(false)
  const [showTakeout, setShowTakeout]   = useState(false)
  const [showPairing, setShowPairing]   = useState(false)
  const [backupStatus, setBackupStatus] = useState(null)
  const [backing, setBacking]           = useState(false)
  const pollRef    = useRef(null)
  const backupPoll = useRef(null)

  async function loadMedia(f = filter, retries = 8) {
    try {
      const raw = await GetMedia(f.year, f.month, 0)
      const page = JSON.parse(raw)
      setItems(page.items)
      setTotal(page.total)
      setError(null)
      setLoading(false)
    } catch (e) {
      if (retries > 0) setTimeout(() => loadMedia(f, retries - 1), 1000)
      else { setError('Could not reach server.'); setLoading(false) }
    }
  }

  async function loadMore() {
    setLoadingMore(true)
    try {
      const raw = await GetMedia(filter.year, filter.month, items.length)
      const page = JSON.parse(raw)
      setItems(prev => [...prev, ...page.items])
      setTotal(page.total)
    } finally {
      setLoadingMore(false)
    }
  }

  async function loadMonths() {
    try { setMonths(JSON.parse(await GetMonths())) } catch {}
  }

  async function loadSettings() {
    try { setSettings(JSON.parse(await GetSettings())) } catch {}
  }

  async function loadBackupStatus() {
    try { setBackupStatus(JSON.parse(await GetBackupStatus())) } catch {}
  }

  useEffect(() => { loadMedia(); loadMonths(); loadSettings(); loadBackupStatus() }, [])

  useEffect(() => {
    const es = new EventSource('http://127.0.0.1:4242/api/events')
    es.onmessage = e => {
      if (e.data === 'backup-done') { loadBackupStatus(); setBacking(false) }
      else if (e.data !== 'movies-done') { loadMedia(); loadMonths() }
    }
    return () => es.close()
  }, [])

  function handleFilter(f) {
    setFilter(f)
    setLoading(true)
    loadMedia(f)
  }

  async function handleIndex() {
    const folder = settings?.media_folder || 'C:\\PhoneMedia'
    if (pollRef.current) clearInterval(pollRef.current)
    setIndexing(true)
    setIndexCount(0)
    await IndexFolder(folder)

    pollRef.current = setInterval(async () => {
      const raw = await GetIndexStatus()
      const s = JSON.parse(raw)
      setIndexCount(s.indexed)
      if (s.status === 'done' || s.status === 'error') {
        clearInterval(pollRef.current)
        pollRef.current = null
        setIndexing(false)
        loadMedia()
        loadMonths()
      }
    }, 1000)
  }

  const selectedIdx = selected ? items.indexOf(selected) : -1
  const handlePrev = useCallback(() => {
    if (selectedIdx > 0) setSelected(items[selectedIdx - 1])
  }, [selectedIdx, items])
  const handleNext = useCallback(() => {
    if (selectedIdx < items.length - 1) setSelected(items[selectedIdx + 1])
  }, [selectedIdx, items])

  async function handleBackup() {
    setBacking(true)
    await StartBackup()
    // SSE backup-done will call loadBackupStatus and set backing=false
  }

  const mediaFolder = settings?.media_folder || 'C:\\PhoneMedia'
  const folderLabel = mediaFolder.split(/[\\/]/).pop() || mediaFolder
  const mediaStreamUrl = id => `http://127.0.0.1:4242/api/stream/${id}`

  return (
    <div className="app">
      <header className="header">
        <span className="logo">Harbor</span>
        <div className="header-right">
          {tab === 'media' && !loading && !error && (
            <span className="count">{items.length} of {total}</span>
          )}
          {tab === 'media' && (
            <>
              <button className="index-btn secondary" onClick={() => setShowTakeout(true)}>
                Google Takeout
              </button>
              <button className="index-btn secondary" onClick={() => setShowPairing(true)}>
                Pair iPhone
              </button>
              {settings?.backup_dest ? (
                <button className="index-btn secondary backup-btn" onClick={handleBackup} disabled={backing || backupStatus?.status === 'running'}>
                  {(backing || backupStatus?.status === 'running') ? 'Backing up…' : `Back Up Now`}
                </button>
              ) : null}
              <button className="index-btn" onClick={handleIndex} disabled={indexing}>
                {indexing ? `Indexing… ${indexCount} files` : `Index ${folderLabel}`}
              </button>
            </>
          )}
          <button className="icon-btn" title="Settings" onClick={() => setShowSettings(true)}>⚙</button>
        </div>
      </header>

      <div className="tab-strip">
        <button className={`tab ${tab === 'media' ? 'active' : ''}`} onClick={() => setTab('media')}>
          Phone Media
        </button>
        <button className={`tab ${tab === 'movies' ? 'active' : ''}`} onClick={() => setTab('movies')}>
          Movies &amp; TV
        </button>
      </div>

      {tab === 'media' && !settings?.backup_dest && (
        <div className="backup-warning-bar">
          Only 1 copy of your phone media — no backup destination set.{' '}
          <button className="backup-warning-link" onClick={() => setShowSettings(true)}>
            Set up backup
          </button>
        </div>
      )}
      {tab === 'media' && settings?.backup_dest && backupStatus && (
        <div className="backup-confidence-bar">
          2 copies
          <span className="backup-sep">·</span>
          last backed up {formatTimeAgo(backupStatus.last_backup_at)}
          {backupStatus.status === 'error' && (
            <span className="backup-error-inline"> — error: {backupStatus.error}</span>
          )}
        </div>
      )}

      <div className="body">
        {tab === 'media' && months.length > 0 && (
          <Sidebar months={months} filter={filter} onFilter={handleFilter} />
        )}

        {tab === 'media' && (
          <main className="main">
            {loading && <p className="status">Loading…</p>}
            {error && <p className="status error">{error}</p>}
            {!loading && !error && items.length === 0 && (
              <p className="status">No photos found. Click Index to scan your library.</p>
            )}
            {!loading && !error && items.length > 0 && (
              <>
                <div className="grid">
                  {items.map(item => (
                    <div key={item.filename} className="card" onClick={() => setSelected(item)}>
                      <div className="card-thumb">
                        <img
                          src={`http://127.0.0.1:4242/api/thumbnail/${item.id}`}
                          alt={item.filename}
                          onError={e => { e.currentTarget.style.display = 'none' }}
                        />
                        {isVideo(item.filename) && <span className="play-icon">▶</span>}
                      </div>
                      <div className="card-info">
                        <p className="card-name" title={item.filename}>{item.filename}</p>
                        <p className="card-date">{formatDate(item.date_taken)}</p>
                        {item.model && <p className="card-camera">{item.model}</p>}
                      </div>
                    </div>
                  ))}
                </div>
                {items.length < total && (
                  <button
                    className="index-btn"
                    onClick={loadMore}
                    disabled={loadingMore}
                    style={{ margin: '20px auto', display: 'block' }}
                  >
                    {loadingMore ? 'Loading…' : `Load more (${total - items.length} remaining)`}
                  </button>
                )}
              </>
            )}
          </main>
        )}

        {tab === 'movies' && (
          <MoviesTab settings={settings} onOpenSettings={() => setShowSettings(true)} />
        )}
      </div>

      {tab === 'media' && selected && (
        <Lightbox
          item={selected}
          items={items}
          streamUrl={mediaStreamUrl}
          onClose={() => setSelected(null)}
          onPrev={handlePrev}
          onNext={handleNext}
        />
      )}

      {showTakeout && (
        <TakeoutModal onClose={() => setShowTakeout(false)} />
      )}

      {showPairing && (
        <PairingModal onClose={() => setShowPairing(false)} />
      )}

      {showSettings && settings && (
        <SettingsModal
          settings={settings}
          onSave={s => { setSettings(s); setShowSettings(false) }}
          onClose={() => setShowSettings(false)}
        />
      )}
    </div>
  )
}
