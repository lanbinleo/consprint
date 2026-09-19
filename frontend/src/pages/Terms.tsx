import { useEffect, useState, type CSSProperties, type PointerEvent as ReactPointerEvent } from 'react'
import { useSearchParams } from 'react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Eye, EyeOff, PanelLeftClose, PanelLeftOpen, Search, Star, X } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { Header, ListSkeleton, StatusPill } from '../components/ui'
import { RichContent } from '../components/RichContent'
import { useUnits } from '../components/ScopePicker'
import type { Concept, ConceptRow, ConceptStatus } from '../lib/types'

// Column resize bounds (px), matched by the grid-template fallbacks in styles.css.
const OUTLINE_RANGE: [min: number, max: number] = [200, 420]
const PANEL_RANGE: [min: number, max: number] = [300, 560]

export function Terms() {
  const { t } = useSession()
  const queryClient = useQueryClient()
  const [searchParams] = useSearchParams()
  const { data: units = [] } = useUnits()
  const [selectedTopic, setSelectedTopic] = useState('')
  const [selectedUnit, setSelectedUnit] = useState('')
  const [search, setSearch] = useState('')
  const [active, setActive] = useState<ConceptRow | null>(null)
  const [panelHidden, setPanelHidden] = useState(false)
  const [outlineHidden, setOutlineHidden] = useState(false)
  const [outlineWidth, setOutlineWidth] = useState(260)
  const [panelWidth, setPanelWidth] = useState(380)
  const [error, setError] = useState('')
  const [savingMark, setSavingMark] = useState(false)

  // Deep link (?concept=<id>, used by the dashboard starred card): open the
  // concept directly on first mount.
  useEffect(() => {
    const conceptId = searchParams.get('concept')
    if (!conceptId) return
    api
      .request<{ concept: Concept; state: ConceptRow['state'] }>(`/api/concepts/${conceptId}`)
      .then((payload) => setActive({ ...payload.concept, state: payload.state }))
      .catch(() => {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const params = new URLSearchParams()
  if (selectedTopic) params.set('topicId', selectedTopic)
  else if (selectedUnit) params.set('unitId', selectedUnit)
  if (search) params.set('search', search)
  const { data: concepts = [], isPending } = useQuery({
    queryKey: ['concepts', selectedUnit, selectedTopic, search],
    // Normalize null (Go nil slice) so the default above actually applies.
    queryFn: async () => (await api.request<ConceptRow[] | null>(`/api/concepts?${params.toString()}`)) ?? [],
  })

  function applyState(conceptId: string, state: ConceptRow['state']) {
    setActive((current) => (current?.id === conceptId ? { ...current, state } : current))
    queryClient.setQueryData<ConceptRow[]>(['concepts', selectedUnit, selectedTopic, search], (rows) =>
      (rows ?? []).map((row) => (row.id === conceptId ? { ...row, state } : row)),
    )
  }

  // Optimistic marking: the pill flips instantly, the request follows. On
  // failure the previous state is restored — unless another mark has landed
  // since — and the banner invites a retry (clicking again re-sends).
  async function mark(concept: ConceptRow, status: ConceptStatus) {
    setError('')
    const prev = concept.state
    const optimistic: ConceptRow['state'] = {
      ...prev,
      status,
      shortTermReview: status === 'fuzzy' || status === 'unknown',
    }
    applyState(concept.id, optimistic)
    setSavingMark(true)
    try {
      const state = await api.request<ConceptRow['state']>(`/api/concepts/${concept.id}/status`, {
        method: 'PATCH',
        body: JSON.stringify({ status }),
      })
      applyState(concept.id, state)
      void queryClient.invalidateQueries({ queryKey: ['concepts'] })
      void queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      void queryClient.invalidateQueries({ queryKey: ['units'] })
    } catch (err) {
      // Roll back only if the optimistic state is still current — a newer
      // mark (clicked while this request was in flight) must survive.
      const rollback = (row: ConceptRow) => (row.state === optimistic ? { ...row, state: prev } : row)
      setActive((current) => (current?.id === concept.id ? rollback(current) : current))
      queryClient.setQueryData<ConceptRow[]>(['concepts', selectedUnit, selectedTopic, search], (rows) =>
        (rows ?? []).map((row) => (row.id === concept.id ? rollback(row) : row)),
      )
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setSavingMark(false)
    }
  }

  // Optimistic star toggle: the star flips instantly, the request follows, and
  // the dashboard starred card refreshes behind it.
  async function toggleStar(concept: ConceptRow) {
    const prev = concept.state
    applyState(concept.id, { ...prev, starred: !prev.starred })
    try {
      const state = await api.request<ConceptRow['state']>(`/api/concepts/${concept.id}/star`, {
        method: 'PATCH',
        body: JSON.stringify({ starred: !prev.starred }),
      })
      applyState(concept.id, state)
      void queryClient.invalidateQueries({ queryKey: ['dashboard', 'starred'] })
    } catch (err) {
      applyState(concept.id, prev)
      setError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  // Drag a column edge: pointer capture keeps tracking on the handle until release.
  function startResize(event: ReactPointerEvent<HTMLDivElement>, side: 'outline' | 'panel') {
    event.preventDefault()
    const handle = event.currentTarget
    const startX = event.clientX
    const startWidth = side === 'outline' ? outlineWidth : panelWidth
    const [min, max] = side === 'outline' ? OUTLINE_RANGE : PANEL_RANGE
    handle.setPointerCapture(event.pointerId)
    const onMove = (move: PointerEvent) => {
      const delta = side === 'outline' ? move.clientX - startX : startX - move.clientX
      const width = Math.round(Math.min(Math.max(startWidth + delta, min), max))
      if (side === 'outline') setOutlineWidth(width)
      else setPanelWidth(width)
    }
    const onUp = () => {
      handle.removeEventListener('pointermove', onMove)
      handle.removeEventListener('pointerup', onUp)
      handle.removeEventListener('pointercancel', onUp)
    }
    handle.addEventListener('pointermove', onMove)
    handle.addEventListener('pointerup', onUp)
    handle.addEventListener('pointercancel', onUp)
  }

  const layoutStyle = {
    '--outline-w': `${outlineWidth}px`,
    '--panel-w': `${panelWidth}px`,
  } as CSSProperties

  return (
    <section className="page browse-page">
      <Header
        eyebrow={t.keyTerms}
        title={t.glossaryTitle}
        action={
          <div className="header-actions">
            <button className="secondary" onClick={() => setOutlineHidden((value) => !value)}>
              {outlineHidden ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
              {outlineHidden ? t.showOutline : t.collapseOutline}
            </button>
            <button className="secondary" onClick={() => setPanelHidden((value) => !value)}>
              {panelHidden ? <Eye size={16} /> : <EyeOff size={16} />} {panelHidden ? t.showCard : t.hideCard}
            </button>
          </div>
        }
      />
      {error && <div className="error">{error}</div>}
      <div
        className={`browse-layout ${outlineHidden ? 'outline-off' : ''} ${panelHidden ? 'panel-off' : ''}`}
        style={layoutStyle}
      >
        {!outlineHidden && (
          <aside className="topic-list">
            <button
              className={selectedTopic === '' && selectedUnit === '' ? 'active' : ''}
              onClick={() => {
                setSelectedTopic('')
                setSelectedUnit('')
              }}
            >
              {t.filterAll}
            </button>
            {units.map((unit) => (
              <div key={unit.id}>
                <h3>{unit.title}</h3>
                <button
                  className={selectedUnit === unit.id && !selectedTopic ? 'active' : ''}
                  onClick={() => {
                    setSelectedUnit(unit.id)
                    setSelectedTopic('')
                  }}
                >
                  {t.all}
                </button>
                {unit.topics.map((topic) => (
                  <button
                    key={topic.id}
                    className={selectedTopic === topic.id ? 'active' : ''}
                    onClick={() => setSelectedTopic(topic.id)}
                  >
                    {topic.title}
                  </button>
                ))}
              </div>
            ))}
          </aside>
        )}
        {!outlineHidden && (
          <div
            className="col-resizer resizer-outline"
            role="separator"
            aria-orientation="vertical"
            onPointerDown={(event) => startResize(event, 'outline')}
          />
        )}
        <div className="concept-list">
          <label className="search">
            <Search size={16} />
            <input placeholder={t.searchTerms} value={search} onChange={(e) => setSearch(e.target.value)} />
          </label>
          {isPending ? (
            <ListSkeleton />
          ) : (
            <div className="table">
              {concepts.slice(0, 180).map((concept) => (
                <button
                  className={`concept-row ${active?.id === concept.id ? 'selected' : ''}`}
                  key={concept.id}
                  onClick={() => setActive(concept)}
                >
                  <span>
                    <strong>{concept.term}</strong>
                    <small>{concept.topic?.title}</small>
                  </span>
                  <StatusPill status={concept.state.status} />
                </button>
              ))}
            </div>
          )}
        </div>
        {!panelHidden && (
          <div
            className="col-resizer resizer-panel"
            role="separator"
            aria-orientation="vertical"
            onPointerDown={(event) => startResize(event, 'panel')}
          />
        )}
        {!panelHidden &&
          (active ? (
            <>
              <div className="panel-scrim" onClick={() => setActive(null)} aria-hidden="true" />
              <aside className="concept-panel">
                <button className="panel-close" onClick={() => setActive(null)} aria-label={t.back}>
                  <X size={16} />
                </button>
                <small>{active.unit?.title}</small>
                <h2>{active.term}</h2>
                <div className="panel-status">
                  <StatusPill status={active.state.status} />
                  {savingMark && <span className="sync-dot" aria-hidden="true" />}
                  <div className="mini-status">
                    {(['proficient', 'fuzzy', 'unknown'] as const).map((status) => (
                      <button
                        key={status}
                        className={active.state.status === status ? 'active' : ''}
                        onClick={() => mark(active, status)}
                      >
                        {status === 'proficient' ? t.proficient : status === 'fuzzy' ? t.fuzzy : t.unknown}
                      </button>
                    ))}
                    <button
                      className={`star-toggle ${active.state.starred ? 'on' : ''}`}
                      onClick={() => toggleStar(active)}
                      aria-label={active.state.starred ? t.unstar : t.star}
                      title={active.state.starred ? t.unstar : t.star}
                    >
                      <Star size={15} fill={active.state.starred ? 'currentColor' : 'none'} />
                    </button>
                  </div>
                </div>
                <RichContent content={active.content} />
              </aside>
            </>
          ) : (
            <aside className="concept-panel empty">{t.selectConcept}</aside>
          ))}
      </div>
    </section>
  )
}
