import { StrictMode, useEffect, useState, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import {
  BookOpen,
  Check,
  Database,
  Eye,
  Gauge,
  LogOut,
  Search,
  Sparkles,
} from 'lucide-react'
import { api, type AuthPayload, type Block, type Concept, type ConceptRow, type Unit } from './api'
import './styles.css'

type View = 'dashboard' | 'browse' | 'review' | 'data'

type Dashboard = {
  totalConcepts: number
  readyConcepts: number
  reviewedConcepts: number
  weakConcepts: number
  averageMastery: number
}

type ImportStatus = {
  units: number
  topics: number
  concepts: number
  readyConcepts: number
  byUnit: { unitId: string; unit: string; concepts: number; ready: number }[]
  runs: { id: string; source: string; status: string; message: string; counts: string; createdAt: string }[]
}

function App() {
  const [auth, setAuth] = useState<AuthPayload | null>(null)
  const [view, setView] = useState<View>('dashboard')

  useEffect(() => {
    if (!api.token) return
    api
      .request<Omit<AuthPayload, 'token'>>('/api/me')
      .then((payload) => setAuth({ ...payload, token: api.token }))
      .catch(() => api.logout())
  }, [])

  if (!auth) {
    return <AuthScreen onAuth={setAuth} />
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">AP</div>
          <div>
            <strong>Psych Sprint</strong>
            <span>{auth.tenant.name}</span>
          </div>
        </div>
        <nav>
          <NavButton active={view === 'dashboard'} icon={<Gauge />} label="Today" onClick={() => setView('dashboard')} />
          <NavButton active={view === 'browse'} icon={<BookOpen />} label="Browse" onClick={() => setView('browse')} />
          <NavButton active={view === 'review'} icon={<Eye />} label="Review" onClick={() => setView('review')} />
          <NavButton active={view === 'data'} icon={<Database />} label="Data" onClick={() => setView('data')} />
        </nav>
        <button
          className="sidebar-action"
          onClick={() => {
            api.logout()
            setAuth(null)
          }}
        >
          <LogOut size={16} /> Log out
        </button>
      </aside>
      <main className="workspace">
        {view === 'dashboard' && <DashboardView goReview={() => setView('review')} />}
        {view === 'browse' && <BrowseView />}
        {view === 'review' && <ReviewView />}
        {view === 'data' && <DataView />}
      </main>
    </div>
  )
}

function AuthScreen({ onAuth }: { onAuth: (payload: AuthPayload) => void }) {
  const [mode, setMode] = useState<'login' | 'register'>('register')
  const [tenantName, setTenantName] = useState('Personal')
  const [name, setName] = useState('Student')
  const [email, setEmail] = useState('student@example.com')
  const [password, setPassword] = useState('ap-psych')
  const [error, setError] = useState('')

  async function submit() {
    setError('')
    try {
      const payload =
        mode === 'register'
          ? await api.register({ tenantName, name, email, password })
          : await api.login({ email, password })
      api.setToken(payload.token)
      onAuth(payload)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Authentication failed')
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-panel">
        <div className="auth-copy">
          <div className="badge">AP Psychology</div>
          <h1>Final Sprint</h1>
          <p>Rate every concept, review the weak ones, and keep the whole knowledge map in one quiet workspace.</p>
        </div>
        <div className="auth-card">
          <div className="segmented">
            <button className={mode === 'register' ? 'active' : ''} onClick={() => setMode('register')}>Register</button>
            <button className={mode === 'login' ? 'active' : ''} onClick={() => setMode('login')}>Login</button>
          </div>
          {mode === 'register' && (
            <>
              <label>Tenant<input value={tenantName} onChange={(e) => setTenantName(e.target.value)} /></label>
              <label>Name<input value={name} onChange={(e) => setName(e.target.value)} /></label>
            </>
          )}
          <label>Email<input value={email} onChange={(e) => setEmail(e.target.value)} /></label>
          <label>Password<input type="password" value={password} onChange={(e) => setPassword(e.target.value)} /></label>
          {error && <div className="error">{error}</div>}
          <button className="primary" onClick={submit}>{mode === 'register' ? 'Create workspace' : 'Enter workspace'}</button>
        </div>
      </section>
    </main>
  )
}

function DashboardView({ goReview }: { goReview: () => void }) {
  const [data, setData] = useState<Dashboard | null>(null)
  useEffect(() => {
    api.request<Dashboard>('/api/dashboard').then(setData)
  }, [])
  return (
    <section>
      <Header eyebrow="Today" title="Your sprint cockpit" action={<button className="primary" onClick={goReview}><Sparkles size={16} /> Start review</button>} />
      <div className="metrics">
        <Metric label="Concepts" value={data?.totalConcepts ?? 0} />
        <Metric label="Ready content" value={data?.readyConcepts ?? 0} />
        <Metric label="Reviewed" value={data?.reviewedConcepts ?? 0} />
        <Metric label="Weak under 3" value={data?.weakConcepts ?? 0} />
      </div>
      <div className="progress-band">
        <div>
          <span>Average mastery</span>
          <strong>{(data?.averageMastery ?? 0).toFixed(2)} / 5</strong>
        </div>
        <div className="mastery-bar"><i style={{ width: `${((data?.averageMastery ?? 0) / 5) * 100}%` }} /></div>
      </div>
    </section>
  )
}

function BrowseView() {
  const [units, setUnits] = useState<Unit[]>([])
  const [concepts, setConcepts] = useState<ConceptRow[]>([])
  const [selectedTopic, setSelectedTopic] = useState('')
  const [search, setSearch] = useState('')
  const [active, setActive] = useState<ConceptRow | null>(null)

  useEffect(() => {
    api.request<Unit[]>('/api/units').then(setUnits)
  }, [])
  useEffect(() => {
    const params = new URLSearchParams()
    if (selectedTopic) params.set('topicId', selectedTopic)
    if (search) params.set('search', search)
    api.request<ConceptRow[]>(`/api/concepts?${params.toString()}`).then(setConcepts)
  }, [selectedTopic, search])

  async function rate(concept: ConceptRow, rating: number) {
    const state = await api.request<ConceptRow['state']>(`/api/concepts/${concept.id}/rating`, {
      method: 'PATCH',
      body: JSON.stringify({ rating }),
    })
    setConcepts((rows) => rows.map((row) => (row.id === concept.id ? { ...row, state } : row)))
    if (active?.id === concept.id) setActive({ ...concept, state })
  }

  return (
    <section>
      <Header eyebrow="Browse" title="Concept map" />
      <div className="browse-layout">
        <aside className="topic-list">
          <button className={selectedTopic === '' ? 'active' : ''} onClick={() => setSelectedTopic('')}>All concepts</button>
          {units.map((unit) => (
            <div key={unit.id}>
              <h3>{unit.title}</h3>
              {unit.topics.map((topic) => (
                <button key={topic.id} className={selectedTopic === topic.id ? 'active' : ''} onClick={() => setSelectedTopic(topic.id)}>
                  {topic.title}
                </button>
              ))}
            </div>
          ))}
        </aside>
        <div className="concept-list">
          <label className="search"><Search size={16} /><input placeholder="Search terms" value={search} onChange={(e) => setSearch(e.target.value)} /></label>
          <div className="table">
            {concepts.map((concept) => (
              <button className="concept-row" key={concept.id} onClick={() => setActive(concept)}>
                <span>
                  <strong>{concept.term}</strong>
                  <small>{concept.topic?.title}</small>
                </span>
                <Rating value={concept.state.mastery} onRate={(rating) => rate(concept, rating)} />
              </button>
            ))}
          </div>
        </div>
        <ConceptPanel concept={active} onRate={(rating) => active && rate(active, rating)} />
      </div>
    </section>
  )
}

function ReviewView() {
  const [queue, setQueue] = useState<Concept[]>([])
  const [index, setIndex] = useState(0)
  const [revealed, setRevealed] = useState(false)
  const [done, setDone] = useState(0)
  const current = queue[index]

  useEffect(() => {
    api.request<Concept[]>('/api/review/next').then(setQueue)
  }, [])

  async function answer(response: 'know' | 'fuzzy' | 'unknown') {
    if (!current) return
    await api.request('/api/review/events', {
      method: 'POST',
      body: JSON.stringify({ conceptId: current.id, cardId: current.cards?.[0]?.id ?? '', response }),
    })
    setDone((value) => value + 1)
    setRevealed(false)
    setIndex((value) => value + 1)
  }

  if (!current) {
    return (
      <section>
        <Header eyebrow="Review" title="Session complete" />
        <div className="empty-state">
          <Check size={28} />
          <strong>{done} concepts reviewed</strong>
          <p>Nice. The weakest concepts will float back into future sessions.</p>
        </div>
      </section>
    )
  }

  return (
    <section>
      <Header eyebrow="Review" title={`${index + 1} / ${queue.length}`} />
      <div className="review-card">
        <small>{current.topic?.title}</small>
        <h2>{current.term}</h2>
        {!revealed ? (
          <div className="review-actions">
            <button className="success" onClick={() => answer('know')}>Know</button>
            <button className="secondary" onClick={() => setRevealed(true)}>Fuzzy</button>
            <button className="danger" onClick={() => setRevealed(true)}>Don't know</button>
          </div>
        ) : (
          <>
            <RichContent content={current.content} />
            <div className="review-actions">
              <button className="secondary" onClick={() => answer('fuzzy')}>Remembered after reveal</button>
              <button className="danger" onClick={() => answer('unknown')}>Still weak</button>
            </div>
          </>
        )}
      </div>
    </section>
  )
}

function DataView() {
  const [status, setStatus] = useState<ImportStatus | null>(null)
  const [busy, setBusy] = useState(false)
  const load = () => api.request<ImportStatus>('/api/import/status').then(setStatus)
  useEffect(() => {
    load()
  }, [])
  async function runImport() {
    setBusy(true)
    await api.request('/api/import/run', { method: 'POST', body: '{}' })
    await load()
    setBusy(false)
  }
  return (
    <section>
      <Header eyebrow="Data" title="Import health" action={<button className="primary" disabled={busy} onClick={runImport}><Database size={16} /> Re-import</button>} />
      <div className="metrics">
        <Metric label="Units" value={status?.units ?? 0} />
        <Metric label="Topics" value={status?.topics ?? 0} />
        <Metric label="Concepts" value={status?.concepts ?? 0} />
        <Metric label="Enriched" value={status?.readyConcepts ?? 0} />
      </div>
      <div className="unit-health">
        {status?.byUnit.map((unit) => (
          <div key={unit.unitId}>
            <span>{unit.unit}</span>
            <strong>{unit.ready} / {unit.concepts}</strong>
            <div className="mastery-bar"><i style={{ width: `${unit.concepts ? (unit.ready / unit.concepts) * 100 : 0}%` }} /></div>
          </div>
        ))}
      </div>
      <div className="run-list">
        {status?.runs.map((run) => (
          <div key={run.id} className="run-row">
            <strong>{run.source}</strong>
            <span>{run.message}</span>
            <code>{run.counts}</code>
          </div>
        ))}
      </div>
    </section>
  )
}

function ConceptPanel({ concept, onRate }: { concept: ConceptRow | null; onRate: (rating: number) => void }) {
  if (!concept) {
    return <aside className="concept-panel empty">Select a concept to inspect its card back.</aside>
  }
  return (
    <aside className="concept-panel">
      <small>{concept.unit?.title}</small>
      <h2>{concept.term}</h2>
      <Rating value={concept.state.mastery} onRate={onRate} />
      <RichContent content={concept.content} />
    </aside>
  )
}

function RichContent({ content }: { content?: Concept['content'] }) {
  if (!content) return <p className="muted">No enriched content yet. The term is still available for recognition review.</p>
  return (
    <div className="rich-content">
      <BlockGroup title="Definition" blocks={content.definition} />
      <BlockGroup title="Examples" blocks={content.examples} />
      <BlockGroup title="Pitfalls" blocks={content.pitfalls} tone="warn" />
      <BlockGroup title="Notes" blocks={content.notes} />
      <small className="source-line">Source: {content.source}</small>
    </div>
  )
}

function BlockGroup({ title, blocks, tone = '' }: { title: string; blocks?: Block[] | null; tone?: string }) {
  if (!blocks?.length) return null
  return (
    <div className={`block-group ${tone}`}>
      <h4>{title}</h4>
      {blocks.map((block, index) => <p key={`${title}-${index}`}>{block.text}</p>)}
    </div>
  )
}

function Rating({ value, onRate }: { value: number; onRate: (rating: number) => void }) {
  return (
    <span className="rating" onClick={(event) => event.stopPropagation()}>
      {[0, 1, 2, 3, 4, 5].map((rating) => (
        <button key={rating} className={Math.round(value) === rating ? 'active' : ''} onClick={() => onRate(rating)} title={`${rating}/5`}>
          {rating}
        </button>
      ))}
    </span>
  )
}

function Header({ eyebrow, title, action }: { eyebrow: string; title: string; action?: ReactNode }) {
  return (
    <div className="page-header">
      <div>
        <span>{eyebrow}</span>
        <h1>{title}</h1>
      </div>
      {action}
    </div>
  )
}

function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div className="metric">
      <span>{label}</span>
      <strong>{Number.isInteger(value) ? value : value.toFixed(2)}</strong>
    </div>
  )
}

function NavButton({ active, icon, label, onClick }: { active: boolean; icon: ReactNode; label: string; onClick: () => void }) {
  return (
    <button className={active ? 'active' : ''} onClick={onClick}>
      {icon}
      {label}
    </button>
  )
}

createRoot(document.getElementById('app')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
