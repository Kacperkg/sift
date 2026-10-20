import React, { useEffect, useMemo, useState } from 'react'
import { createRoot } from 'react-dom/client'
import './styles.css'

type Application = { id: string; company: string; role: string; status: string; closed?: boolean; lastEvent: string; updatedAt: string }
type AIStatus = { enabled: boolean; available: boolean; modelInstalled: boolean; model: string }
type BlockedRole = { role?: string; messageId?: string; label?: string; createdAt: string }
type SyncProgress = { running: boolean; phase: string; processed: number; total: number }
const statuses = ['Applied', 'No response', 'Interviewing', 'Offer', 'Accepted', 'Declined'] as const
const filters = ['All', ...statuses] as const

function ArrowIcon({ direction = 'up' }: { direction?: 'up' | 'left' }) {
  return direction === 'left'
    ? <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M19 12H5M11 18l-6-6 6-6" /></svg>
    : <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 17 17 7M8 7h9v9" /></svg>
}
function SearchIcon() { return <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-4-4" /></svg> }
function RefreshIcon() { return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 11a8 8 0 1 0-2.35 5.65M20 4v7h-7" /></svg> }
function LockIcon() { return <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="5" y="10" width="14" height="10" rx="2" /><path d="M8 10V7a4 4 0 0 1 8 0v3" /></svg> }

function formatDate(value: string, long = false) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Unknown date'
  return date.toLocaleDateString('en-GB', long ? { day: 'numeric', month: 'long', year: 'numeric' } : { day: '2-digit', month: 'short', year: '2-digit' })
}
function initials(company: string) { return company.split(/\s+/).filter(Boolean).slice(0, 2).map(word => word[0]).join('').toUpperCase() || '—' }
function statusClass(status: string) { return status.toLowerCase().replaceAll(' ', '-') }
function routeId() {
  const match = window.location.hash.match(/^#\/application\/(.+)$/)
  if (!match) return ''
  try { return decodeURIComponent(match[1]) } catch { return '' }
}

function App() {
  const [connected, setConnected] = useState(false)
  const [apps, setApps] = useState<Application[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [filter, setFilter] = useState<(typeof filters)[number]>('All')
  const [query, setQuery] = useState('')
  const [selectedId, setSelectedId] = useState(routeId)
  const [ai, setAI] = useState<AIStatus>({ enabled: false, available: false, modelInstalled: false, model: 'qwen3:4b' })
  const [aiBusy, setAIBusy] = useState(false)
  const [blockedRoles, setBlockedRoles] = useState<BlockedRole[]>([])
  const [blockBusy, setBlockBusy] = useState(false)
  const [outcomeBusy, setOutcomeBusy] = useState(false)
  const [scanProgress, setScanProgress] = useState<SyncProgress | null>(null)

  const load = async () => {
    const [authResponse, appsResponse, aiResponse, blocksResponse] = await Promise.all([fetch('/api/auth/status'), fetch('/api/applications'), fetch('/api/ai/status'), fetch('/api/blocked-roles')])
    if (!authResponse.ok || !appsResponse.ok) throw new Error('Could not reach the local Job Inbox service.')
    const auth = await authResponse.json()
    const data = await appsResponse.json()
    setConnected(Boolean(auth.connected)); setApps(Array.isArray(data) ? data : [])
    if (aiResponse.ok) setAI(await aiResponse.json())
    if (blocksResponse.ok) setBlockedRoles(await blocksResponse.json())
  }
  useEffect(() => {
    load().catch(error => setError(error instanceof Error ? error.message : 'Start the local Go server to use Job Inbox.'))
    const handleRoute = () => setSelectedId(routeId())
    window.addEventListener('hashchange', handleRoute)
    return () => window.removeEventListener('hashchange', handleRoute)
  }, [])

  const sync = async (mode: 'update' | 'full') => {
    setBusy(true); setError('')
    setScanProgress({ running: true, phase: mode === 'full' ? 'Starting full scan' : 'Checking for updates', processed: 0, total: 0 })
    const pollProgress = async () => {
      try {
        const response = await fetch('/api/sync/status')
        if (response.ok) setScanProgress(await response.json())
      } catch { /* The main sync request reports actionable errors. */ }
    }
    const progressTimer = window.setInterval(pollProgress, 450)
    try {
      const response = await fetch(`/api/sync?mode=${mode}`, { method: 'POST' })
      if (!response.ok) throw new Error(await response.text())
      const updatedApps = await response.json()
      setApps(updatedApps)
      setScanProgress(current => ({ running: false, phase: mode === 'full' ? 'Full scan complete' : 'Inbox updated', processed: current?.total || updatedApps.length, total: current?.total || updatedApps.length }))
    } catch (error) {
      setError(error instanceof Error ? error.message : 'Sync failed')
      setScanProgress(current => ({ running: false, phase: 'Scan stopped', processed: current?.processed || 0, total: current?.total || 0 }))
    } finally {
      window.clearInterval(progressTimer)
      setBusy(false)
      window.setTimeout(() => setScanProgress(null), 2200)
    }
  }

  const toggleAI = async () => {
    setAIBusy(true); setError('')
    try {
      const response = await fetch('/api/ai/status', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ enabled: !ai.enabled }) })
      if (!response.ok) throw new Error(await response.text())
      setAI(await response.json())
    } catch (error) { setError(error instanceof Error ? error.message : 'Could not update local AI.') }
    finally { setAIBusy(false) }
  }

  const visibleApps = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return apps.filter(app => (filter === 'All' || app.status === filter) && (!needle || [app.company, app.role, app.lastEvent].some(value => value.toLowerCase().includes(needle))))
  }, [apps, filter, query])
  const selected = apps.find(app => app.id === selectedId)
  const openApplication = (id: string) => { window.location.hash = `/application/${encodeURIComponent(id)}` }
  const closeApplication = () => { window.history.pushState(null, '', `${window.location.pathname}${window.location.search}`); setSelectedId('') }
  const setItemBlocked = async (item: { role?: string; messageId?: string; label?: string }, blocked: boolean) => {
    setBlockBusy(true); setError('')
    try {
      const response = await fetch('/api/blocked-roles', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...item, blocked }) })
      if (!response.ok) throw new Error(await response.text())
      setBlockedRoles(await response.json())
      if (blocked) {
        setApps(current => current.filter(app => item.messageId ? app.id !== item.messageId : app.role.trim().toLowerCase() !== item.role?.trim().toLowerCase()))
        closeApplication()
      }
    } catch (error) { setError(error instanceof Error ? error.message : 'Could not update blocked roles.') }
    finally { setBlockBusy(false) }
  }
  const closeInterview = async (application: Application, status: 'Accepted' | 'Declined') => {
    setOutcomeBusy(true); setError('')
    try {
      const response = await fetch('/api/applications/status', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id: application.id, status }) })
      if (!response.ok) throw new Error(await response.text())
      const updated = await response.json()
      setApps(current => current.map(item => item.id === updated.id ? updated : item))
    } catch (error) { setError(error instanceof Error ? error.message : 'Could not close this interview.') }
    finally { setOutcomeBusy(false) }
  }
  if (selectedId) return <Detail application={selected} onBack={closeApplication} onBlock={application => setItemBlocked(application.role === 'Role not detected' ? { messageId: application.id, label: application.lastEvent || application.company } : { role: application.role }, true)} blockBusy={blockBusy} onCloseInterview={closeInterview} outcomeBusy={outcomeBusy} error={error} />

  return <main className="shell">
    <nav className="topbar" aria-label="Primary navigation">
      <a className="brand" href="#top" aria-label="Job Inbox home"><span className="brand-mark">JI</span><span>Job Inbox</span></a>
      <div className="privacy-state"><span className="live-dot" /> Local vault · Gmail read-only</div>
    </nav>

    <header className="hero" id="top">
      <div className="hero-copy"><span className="eyebrow">PRIVATE CAREER INTELLIGENCE / 01</span><h1>Your job search,<br /><em>out of the inbox.</em></h1></div>
      <div className="hero-aside">
        <p>A calm, private view of every application signal found in your Gmail.</p>
        {connected
          ? <div className="scan-actions"><button className="primary-action" onClick={() => sync('update')} disabled={busy}><RefreshIcon /> {busy ? 'Reading Gmail…' : 'Update inbox'}</button><button className="secondary-action" onClick={() => sync('full')} disabled={busy}>Full rescan</button></div>
          : <a className="primary-action" href="/auth/google">Connect Gmail <ArrowIcon /></a>}
        <div className="ai-control">
          <div><span className={`ai-dot ${ai.enabled ? 'on' : ''}`} /><span><strong>Local AI assist</strong><small>{!ai.available ? 'Ollama offline' : !ai.modelInstalled ? `${ai.model} not installed` : ai.enabled ? `${ai.model} · ambiguous emails only` : `${ai.model} · ready`}</small></span></div>
          <button className={ai.enabled ? 'enabled' : ''} onClick={toggleAI} disabled={aiBusy || !ai.available || !ai.modelInstalled} aria-pressed={ai.enabled}>{aiBusy ? '…' : ai.enabled ? 'On' : 'Off'}</button>
        </div>
      </div>
    </header>

    <section className="metrics" aria-label="Application overview">
      <div className="metric metric-total"><span className="metric-index">00</span><strong>{String(apps.length).padStart(2, '0')}</strong><span>Total tracked</span></div>
      {statuses.map((status, index) => <button className={`metric ${filter === status ? 'is-active' : ''}`} key={status} onClick={() => setFilter(filter === status ? 'All' : status)}><span className="metric-index">0{index + 1}</span><strong>{String(apps.filter(app => app.status === status).length).padStart(2, '0')}</strong><span>{status}</span></button>)}
    </section>

    {error && <div className="error" role="alert"><span>!</span>{error}</div>}

    <section className="workspace">
      <div className="workspace-heading">
        <div><span className="section-kicker">Application index</span><h2>Every opportunity,<br />in one place.</h2></div>
        <div className="workspace-tools">
          <label className="search"><SearchIcon /><span className="sr-only">Search applications</span><input value={query} onChange={event => setQuery(event.target.value)} placeholder="Search company, role, email…" /></label>
          <div className="filters" aria-label="Filter by status">{filters.map(item => <button key={item} className={filter === item ? 'active' : ''} onClick={() => setFilter(item)}>{item}</button>)}</div>
        </div>
      </div>

      {apps.length === 0 ? <div className="empty"><span className="empty-star">✦</span><h3>Your index is ready.</h3><p>{connected ? 'Scan Gmail to find your first application.' : 'Connect Gmail, then run your first private scan.'}</p></div>
      : visibleApps.length === 0 ? <div className="empty compact"><h3>No matches found.</h3><button className="text-button" onClick={() => { setQuery(''); setFilter('All') }}>Clear filters</button></div>
      : <div className="application-list">
          <div className="list-labels" aria-hidden="true"><span>Company / role</span><span>Latest email</span><span>Status</span><span>Updated</span><span /></div>
          {visibleApps.map((app, index) => <button className="application-row" key={app.id} onClick={() => openApplication(app.id)} aria-label={`Open ${app.role} at ${app.company}`}>
            <span className="company-cell"><span className="company-avatar">{initials(app.company)}</span><span><strong>{app.company}</strong><small>{app.role}</small></span></span>
            <span className="event-cell">{app.lastEvent || 'Application email'}</span>
            <span><i className={`pill ${statusClass(app.status)}`}><span />{app.status}</i></span>
            <time dateTime={app.updatedAt}>{formatDate(app.updatedAt)}</time>
            <span className="row-arrow"><ArrowIcon /></span><span className="row-number">{String(index + 1).padStart(2, '0')}</span>
          </button>)}
        </div>}
    </section>

    {blockedRoles.length > 0 && <section className="blocked-panel">
      <div><span className="section-kicker">Blocked items</span><p>Blocked role titles and hidden emails will stay out of future scans.</p></div>
      <div className="blocked-list">{blockedRoles.map(item => <button key={item.messageId || item.role?.toLowerCase() || item.createdAt} onClick={() => setItemBlocked({ role: item.role, messageId: item.messageId, label: item.label }, false)} disabled={blockBusy} title="Allow this item again">{item.role || item.label || 'Hidden email'}<span>×</span></button>)}</div>
    </section>}

    {scanProgress && <div className={`scan-toast ${scanProgress.running ? 'running' : 'complete'} ${scanProgress.total === 0 ? 'indeterminate' : ''}`} role="status" aria-live="polite">
      <div className="scan-toast-heading"><span className="scan-indicator" /><div><strong>{scanProgress.phase}</strong><small>{scanProgress.total > 0 ? `${scanProgress.processed} of ${scanProgress.total} emails` : 'Preparing your private inbox scan'}</small></div><b>{scanProgress.total > 0 ? `${Math.min(100, Math.round((scanProgress.processed / scanProgress.total) * 100))}%` : '•••'}</b></div>
      <div className="scan-track"><span style={{ width: scanProgress.total > 0 ? `${Math.min(100, (scanProgress.processed / scanProgress.total) * 100)}%` : '18%' }} /></div>
    </div>}

    <footer><div><LockIcon /><span><strong>Your inbox stays yours.</strong> Email is read locally and job records are encrypted with a key held in macOS Keychain.</span></div><span>JOB INBOX / PRIVATE INDEX</span></footer>
  </main>
}

function Detail({ application, onBack, onBlock, blockBusy, onCloseInterview, outcomeBusy, error }: { application?: Application; onBack: () => void; onBlock: (application: Application) => void; blockBusy: boolean; onCloseInterview: (application: Application, status: 'Accepted' | 'Declined') => void; outcomeBusy: boolean; error: string }) {
  if (!application) return <main className="detail-shell missing"><button className="back-button" onClick={onBack}><ArrowIcon direction="left" /> Back to index</button><h1>Application not found.</h1><p>It may have been removed by a more recent Gmail scan.</p></main>
  const gmailUrl = `https://mail.google.com/mail/u/0/#all/${encodeURIComponent(application.id)}`
  return <main className="detail-shell">
    <nav className="topbar detail-nav"><button className="back-button" onClick={onBack}><ArrowIcon direction="left" /> Back to index</button><div className="privacy-state"><span className="live-dot" /> Local vault</div></nav>
    <section className="detail-hero">
      <div className="detail-avatar">{initials(application.company)}</div>
      <div className="detail-title"><span className="eyebrow">APPLICATION RECORD / {application.id.slice(-6).toUpperCase()}</span><h1>{application.company}</h1><p>{application.role}</p></div>
      <i className={`pill detail-status ${statusClass(application.status)}`}><span />{application.status}</i>
    </section>
    <section className="detail-grid">
      <article className="email-card"><span className="section-kicker">Latest matched email</span><h2>{application.lastEvent || 'Application update'}</h2><div className="email-meta"><span>Gmail message</span><time dateTime={application.updatedAt}>{formatDate(application.updatedAt, true)}</time></div><a className="gmail-button" href={gmailUrl} target="_blank" rel="noreferrer">Open original in Gmail <ArrowIcon /></a><p className="link-note">Opens the exact message in your signed-in Gmail account.</p></article>
      <aside className="record-card"><span className="section-kicker">Record details</span><dl><div><dt>Company</dt><dd>{application.company}</dd></div><div><dt>Role</dt><dd>{application.role}</dd></div><div><dt>Status</dt><dd>{application.status}</dd></div><div><dt>Last activity</dt><dd>{formatDate(application.updatedAt, true)}</dd></div></dl>{application.status === 'Interviewing' && !application.closed && <div className="outcome-action"><span className="section-kicker">Close interview</span><div><button className="accept" onClick={() => onCloseInterview(application, 'Accepted')} disabled={outcomeBusy}>{outcomeBusy ? 'Saving…' : 'Accepted'}</button><button className="decline" onClick={() => onCloseInterview(application, 'Declined')} disabled={outcomeBusy}>{outcomeBusy ? 'Saving…' : 'Declined'}</button></div><p>Sets a private manual outcome. Nothing is changed in Gmail.</p>{error && <small role="alert">{error}</small>}</div>}<div className="security-note"><LockIcon /><p><strong>Stored privately</strong><br />This record is encrypted on your Mac. Job Inbox only has read-only Gmail access.</p></div><div className="block-action"><button onClick={() => onBlock(application)} disabled={blockBusy}>{blockBusy ? 'Removing…' : application.role === 'Role not detected' ? 'Not my application · Hide email' : 'Not my application · Block role'}</button><p>{application.role === 'Role not detected' ? 'Hides this Gmail message permanently from future scans.' : `Removes every exact “${application.role}” match and prevents it returning after a scan.`}</p></div></aside>
    </section>
  </main>
}

createRoot(document.getElementById('root')!).render(<React.StrictMode><App /></React.StrictMode>)
