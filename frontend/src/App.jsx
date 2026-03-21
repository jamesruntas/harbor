import { useState, useEffect, useRef } from 'react'
import { GetMedia, IndexFolder, GetIndexStatus } from '../wailsjs/go/main/App'

function formatDate(raw) {
  if (!raw) return '—'
  // ExifTool format: "2024:03:15 14:22:01"
  const normalized = raw.replace(/^(\d{4}):(\d{2}):(\d{2})/, '$1-$2-$3')
  const d = new Date(normalized)
  if (isNaN(d)) return raw
  return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
}

export default function App() {
  const [items, setItems] = useState([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [indexing, setIndexing] = useState(false)
  const [indexCount, setIndexCount] = useState(0)
  const [error, setError] = useState(null)
  const pollRef = useRef(null)

  async function loadMedia(retries = 8) {
    try {
      const raw = await GetMedia(0)
      const page = JSON.parse(raw)
      setItems(page.items)
      setTotal(page.total)
      setError(null)
      setLoading(false)
    } catch (e) {
      if (retries > 0) {
        setTimeout(() => loadMedia(retries - 1), 1000)
      } else {
        setError('Could not reach server.')
        setLoading(false)
      }
    }
  }

  async function loadMore() {
    setLoadingMore(true)
    try {
      const raw = await GetMedia(items.length)
      const page = JSON.parse(raw)
      setItems(prev => [...prev, ...page.items])
      setTotal(page.total)
    } finally {
      setLoadingMore(false)
    }
  }

  useEffect(() => { loadMedia() }, [])

  async function handleIndex() {
    if (pollRef.current) clearInterval(pollRef.current)
    setIndexing(true)
    setIndexCount(0)
    await IndexFolder('C:\\PhoneMedia')

    pollRef.current = setInterval(async () => {
      const raw = await GetIndexStatus()
      const s = JSON.parse(raw)
      setIndexCount(s.indexed)
      if (s.status === 'done' || s.status === 'error') {
        clearInterval(pollRef.current)
        pollRef.current = null
        setIndexing(false)
        await loadMedia()
      }
    }, 1000)
  }

  return (
    <div className="app">
      <header className="header">
        <span className="logo">HomeStream</span>
        <div className="header-right">
          {!loading && !error && (
            <span className="count">{items.length} of {total} photos</span>
          )}
          <button className="index-btn" onClick={handleIndex} disabled={indexing}>
            {indexing ? `Indexing… ${indexCount} files` : 'Index C:\\PhoneMedia'}
          </button>
        </div>
      </header>

      <main className="main">
        {loading && <p className="status">Loading…</p>}
        {error && <p className="status error">{error}</p>}
        {!loading && !error && items.length === 0 && (
          <p className="status">No photos indexed yet. Click Index to scan your library.</p>
        )}
        {!loading && !error && items.length > 0 && (
          <>
            <div className="grid">
              {items.map((item) => (
                <div key={item.filename} className="card">
                  <div className="card-thumb">
                    <span className="card-ext">{item.filename.split('.').pop().toUpperCase()}</span>
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
              <button className="index-btn" onClick={loadMore} disabled={loadingMore} style={{margin: '20px auto', display: 'block'}}>
                {loadingMore ? 'Loading…' : `Load more (${total - items.length} remaining)`}
              </button>
            )}
          </>
        )}
      </main>
    </div>
  )
}
