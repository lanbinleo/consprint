import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ExternalLink, Link2, Maximize2, Minimize2, NotebookPen, RefreshCw } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { Header, ListSkeleton } from '../components/ui'
import type { NoteResource, NoteResourceItem } from '../lib/types'

export function Notes() {
  const { t } = useSession()
  const { data: resources = [], isPending } = useQuery({
    queryKey: ['note-resources'],
    queryFn: async () => (await api.request<NoteResource[] | null>('/api/note-resources')) ?? [],
  })
  const [activeId, setActiveId] = useState<string | null>(null)
  const current = useMemo(
    () => resources.find((resource) => resource.id === activeId) ?? resources[0] ?? null,
    [resources, activeId],
  )

  return (
    <section className="page notes-page">
      <Header eyebrow={t.notes} title={t.notesTitle} />
      {isPending ? (
        <ListSkeleton />
      ) : !current ? (
        <div className="empty-state">
          <NotebookPen size={28} />
          <p className="muted">{t.notesEmpty}</p>
        </div>
      ) : (
        <>
          <div className="note-tabs" role="tablist">
            {resources.map((resource) => (
              <button
                key={resource.id}
                role="tab"
                aria-selected={resource.id === current.id}
                className={`note-tab ${resource.id === current.id ? 'active' : ''}`}
                onClick={() => setActiveId(resource.id)}
              >
                {resource.title}
              </button>
            ))}
          </div>
          <TabBody key={current.id} resource={current} />
        </>
      )}
    </section>
  )
}

function TabBody({ resource }: { resource: NoteResource }) {
  const { t } = useSession()
  const [index, setIndex] = useState(0)
  const [expanded, setExpanded] = useState(false)
  const item = resource.items[Math.min(index, resource.items.length - 1)]

  // Esc leaves expanded mode; lock body scroll behind the overlay.
  useEffect(() => {
    if (!expanded) return
    function onKey(event: KeyboardEvent) {
      if (event.key === 'Escape') setExpanded(false)
    }
    document.addEventListener('keydown', onKey)
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = ''
    }
  }, [expanded])

  if (!item) {
    return (
      <div className="empty-state note-empty">
        <NotebookPen size={28} />
        <p className="muted">{t.notesEmpty}</p>
      </div>
    )
  }

  const embeddable = item.kind === 'embed' || item.kind === 'pdf'

  return (
    <div className="note-body">
      <div className="note-toolbar">
        {resource.items.length > 1 && (
          <div className="note-items" role="tablist">
            {resource.items.map((entry, i) => (
              <button key={entry.id} className={i === index ? 'active' : ''} onClick={() => setIndex(i)}>
                {entry.label}
              </button>
            ))}
          </div>
        )}
        <div className="note-toolbar-actions">
          <a className="icon-btn" href={item.url} target="_blank" rel="noreferrer noopener" title={t.notesOpenExternal}>
            <ExternalLink size={16} />
          </a>
          {embeddable && (
            <button
              className="icon-btn"
              title={expanded ? t.notesCollapse : t.notesExpand}
              onClick={() => setExpanded((value) => !value)}
            >
              {expanded ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
            </button>
          )}
        </div>
      </div>
      <ItemPanel key={item.id} item={item} expanded={expanded} onToggleExpand={() => setExpanded((value) => !value)} />
      {resource.description && (
        <p className="note-desc muted" title={resource.description}>
          {resource.description}
        </p>
      )}
    </div>
  )
}

// EMBED_LOAD_GRACE_MS is how long we wait for the iframe load event before
// deciding the source is unreachable and swapping in the fallback card.
// Cross-origin frames blocked by X-Frame-Options often fire "load" on an
// empty error page, which cannot be detected from the parent — that is why
// the open-in-new-tab button stays visible no matter what.
const EMBED_LOAD_GRACE_MS = 5000

function ItemPanel({
  item,
  expanded,
  onToggleExpand,
}: {
  item: NoteResourceItem
  expanded: boolean
  onToggleExpand: () => void
}) {
  const { t } = useSession()
  const [attempt, setAttempt] = useState(0)
  const [failed, setFailed] = useState(false)
  const loadedRef = useRef(false)

  useEffect(() => {
    if (item.kind !== 'embed' && item.kind !== 'pdf') return
    loadedRef.current = false
    setFailed(false)
    const timer = window.setTimeout(() => {
      if (!loadedRef.current) setFailed(true)
    }, EMBED_LOAD_GRACE_MS)
    return () => window.clearTimeout(timer)
  }, [item.id, item.kind, attempt])

  const openExternal = (
    <a className="secondary" href={item.url} target="_blank" rel="noreferrer noopener">
      <ExternalLink size={16} /> {t.notesOpenExternal}
    </a>
  )

  if (item.kind === 'link') {
    return (
      <div className="note-link-card">
        <Link2 size={22} />
        <div>
          <strong>{item.label}</strong>
          <p className="muted">{item.url}</p>
        </div>
        {openExternal}
      </div>
    )
  }

  if (item.kind === 'image') {
    return (
      <div className="note-image">
        <a href={item.url} target="_blank" rel="noreferrer noopener" title={t.notesOpenExternal}>
          <img src={item.url} alt={item.label} loading="lazy" />
        </a>
      </div>
    )
  }

  if (failed) {
    return (
      <div className="note-fallback">
        <NotebookPen size={28} />
        <strong>{item.label}</strong>
        <p className="muted">{t.notesEmbedBlocked}</p>
        <div className="action-row">
          {openExternal}
          <button className="secondary" onClick={() => setAttempt((n) => n + 1)}>
            <RefreshCw size={16} /> {t.notesRetryEmbed}
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className={`note-frame-wrap ${expanded ? 'expanded' : ''}`}>
      {expanded && (
        <div className="note-expanded-bar">
          <strong>{item.label}</strong>
          <div className="note-toolbar-actions">
            <a className="icon-btn" href={item.url} target="_blank" rel="noreferrer noopener" title={t.notesOpenExternal}>
              <ExternalLink size={16} />
            </a>
            <button className="icon-btn" title={t.notesCollapse} onClick={onToggleExpand}>
              <Minimize2 size={16} />
            </button>
          </div>
        </div>
      )}
      <div className="note-frame">
        <iframe
          key={attempt}
          src={item.url}
          title={item.label}
          loading="lazy"
          allow="fullscreen"
          onLoad={() => {
            loadedRef.current = true
          }}
        />
      </div>
    </div>
  )
}
