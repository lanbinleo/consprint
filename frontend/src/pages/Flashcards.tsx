import { useEffect, useMemo, useState } from 'react'
import { useBlocker, useNavigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, ChevronRight, Play, RotateCcw, Star } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { praiseKey } from '../lib/i18n'
import { Header, KeyboardHint, Metric, SetupCard } from '../components/ui'
import { RichContent } from '../components/RichContent'
import { useUnits } from '../components/ScopePicker'
import { StatusButtons } from '../components/StatusButtons'
import { reviewQueue, usePendingReviewCount } from '../lib/reviewQueue'
import { markConceptState, useConceptRows } from '../lib/conceptStore'
import type { Concept, ConceptRow, ConceptState, ConceptStatus, Unit } from '../lib/types'

type Filter = 'unmarked' | 'review' | 'proficient' | 'all'

const FILTER_VALUES: Filter[] = ['all', 'unmarked', 'review', 'proficient']

export function Flashcards() {
  const { t } = useSession()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { data: units = [], isPending: unitsPending } = useUnits()
  // Card content hydrates from the shared concept store, so the deck request
  // itself carries only concept ids + state.
  const { rows: conceptRows, isPending: conceptsPending } = useConceptRows()
  const [picked, setPicked] = useState<Set<string>>(new Set())
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [order, setOrder] = useState<'random' | 'outline'>('random')
  const [filter, setFilter] = useState<Filter>('all')
  const [queue, setQueue] = useState<ConceptRow[]>([])
  const [index, setIndex] = useState(0)
  const [flipped, setFlipped] = useState(false)
  const [started, setStarted] = useState(false)
  const [loading, setLoading] = useState(false)
  const [tally, setTally] = useState<Record<string, number>>({})
  const [error, setError] = useState('')
  // Star toggles made this session; wins over the payload's initial state.
  const [starOverrides, setStarOverrides] = useState<Record<string, boolean>>({})
  const current = queue[index]

  const totalTopics = useMemo(() => units.reduce((sum, unit) => sum + unit.topics.length, 0), [units])
  const pendingCount = usePendingReviewCount()

  // Scope params (topics + status) drive the session fetch below.
  const scopeParams = useMemo(() => {
    const search = new URLSearchParams()
    if (picked.size > 0) search.set('topicIds', [...picked].join(','))
    if (filter === 'unmarked') search.set('status', 'unmarked')
    if (filter === 'review') search.set('status', 'fuzzy,unknown')
    if (filter === 'proficient') search.set('status', 'proficient')
    return search.toString()
  }, [picked, filter])

  // Totals come from per-topic metadata on /api/units, summed locally — no
  // count request per scope change.
  const totalAvailable = useMemo(() => {
    const allTopics = units.flatMap((unit) => unit.topics)
    const topics = picked.size > 0 ? allTopics.filter((topic) => picked.has(topic.id)) : allTopics
    const sums = topics.reduce(
      (acc, topic) => {
        const c = topic.counts
        if (c) {
          acc.total += c.total
          acc.proficient += c.proficient
          acc.fuzzy += c.fuzzy
          acc.unknown += c.unknown
        }
        return acc
      },
      { total: 0, proficient: 0, fuzzy: 0, unknown: 0 },
    )
    if (filter === 'unmarked') return sums.total - sums.proficient - sums.fuzzy - sums.unknown
    if (filter === 'review') return sums.fuzzy + sums.unknown
    if (filter === 'proficient') return sums.proficient
    return sums.total
  }, [units, picked, filter])

  // Session-size cap shared with the backend (/api/review/next serves at most 200).
  const sessionCap = 200
  const sessionInProgress = started && !!current
  const blocker = useBlocker(sessionInProgress)

  // Guard an unfinished session: confirm on SPA navigation and on reload/close.
  useEffect(() => {
    if (!sessionInProgress) return
    const handler = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', handler)
    return () => window.removeEventListener('beforeunload', handler)
  }, [sessionInProgress])

  function toggleTopic(topicId: string) {
    setPicked((current) => {
      const next = new Set(current)
      if (next.has(topicId)) next.delete(topicId)
      else next.add(topicId)
      return next
    })
  }

  function toggleUnit(unit: Unit) {
    setPicked((current) => {
      const next = new Set(current)
      const allIn = unit.topics.every((topic) => next.has(topic.id))
      for (const topic of unit.topics) {
        if (allIn) next.delete(topic.id)
        else next.add(topic.id)
      }
      return next
    })
  }

  function toggleExpand(unitId: string) {
    setExpanded((current) => {
      const next = new Set(current)
      if (next.has(unitId)) next.delete(unitId)
      else next.add(unitId)
      return next
    })
  }

  useEffect(() => {
    if (!started || !current) return
    function handleKey(event: KeyboardEvent) {
      // Holding 1/2/3 auto-repeats and would walk the whole queue in
      // seconds, queuing garbage marks; modifier combos belong to the
      // browser/OS.
      if (event.repeat || event.metaKey || event.ctrlKey || event.altKey) return
      // Enter/Space on a focused button must activate that button (e.g. the
      // three status buttons), not flip the card.
      if (event.target instanceof HTMLElement && event.target.closest('button, input, textarea, select')) {
        if (event.key === ' ' || event.key === 'Enter') return
      }
      if (event.key === ' ' || event.key === 'Enter') {
        event.preventDefault()
        setFlipped((value) => !value)
        return
      }
      if (event.key === 'ArrowLeft') {
        event.preventDefault()
        goPrevious()
        return
      }
      if (event.key === 'ArrowRight') {
        event.preventDefault()
        goNext()
        return
      }
      if (event.key === '1') respond('proficient')
      if (event.key === '2') respond('fuzzy')
      if (event.key === '3') respond('unknown')
    }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [started, current])

  // The deck endpoint returns concept ids + state in the chosen order; card
  // content is looked up in the local corpus. Ids missing from the corpus
  // (stale-cache edge, e.g. fresh import) are backfilled from the detail
  // endpoint so a session never silently drops cards.
  async function start() {
    setLoading(true)
    setError('')
    try {
      const search = new URLSearchParams({ limit: String(sessionCap), order })
      const deck = (await api.request<ConceptState[] | null>(`/api/review/next?${scopeParams}&${search.toString()}`)) ?? []
      const byId = new Map(conceptRows.map((row) => [row.id, row]))
      const resolved = new Map<string, ConceptRow>()
      const missing: ConceptState[] = []
      for (const entry of deck) {
        const row = byId.get(entry.conceptId)
        if (row) resolved.set(entry.conceptId, { ...row, state: entry })
        else missing.push(entry)
      }
      if (missing.length > 0) {
        const details = await Promise.all(
          missing.map((entry) =>
            api
              .request<{ concept: Concept; state: ConceptState }>(`/api/concepts/${entry.conceptId}`)
              .then((payload) => ({ ...payload.concept, state: entry }))
              .catch(() => null),
          ),
        )
        for (const row of details) if (row) resolved.set(row.id, row)
      }
      setQueue(deck.map((entry) => resolved.get(entry.conceptId)).filter((row) => !!row))
      setIndex(0)
      setFlipped(false)
      setTally({})
      setStarted(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setLoading(false)
    }
  }

  // Marks are queued locally and flushed in the background, so the card always
  // flips immediately; retries are handled by the queue. The shared concept
  // cache is patched too, so the glossary reflects the mark right away.
  function respond(response: ConceptStatus) {
    if (!current || !response) return
    setError('')
    reviewQueue.enqueue({ conceptId: current.id, response })
    markConceptState(queryClient, current.id, {
      status: response,
      shortTermReview: response !== 'proficient',
      reviewCount: current.state.reviewCount + 1,
    })
    setTally((counts) => ({ ...counts, [response]: (counts[response] ?? 0) + 1 }))
    setFlipped(false)
    setIndex((value) => value + 1)
  }

  function goPrevious() {
    setIndex((value) => Math.max(0, value - 1))
    setFlipped(false)
  }

  function goNext() {
    setFlipped(false)
    setIndex((value) => value + 1)
  }

  // Corner star: optimistic toggle that never flips the card underneath, and
  // keeps the shared concept cache in step.
  function toggleCardStar(concept: ConceptRow) {
    const next = !(starOverrides[concept.id] ?? concept.state.starred)
    setStarOverrides((map) => ({ ...map, [concept.id]: next }))
    api
      .request<ConceptState>(`/api/concepts/${concept.id}/star`, { method: 'PATCH', body: JSON.stringify({ starred: next }) })
      .then((state) =>
        markConceptState(queryClient, concept.id, {
          status: state.status,
          reviewCount: state.reviewCount,
          shortTermReview: state.shortTermReview,
          starred: state.starred,
        }),
      )
      .catch(() => setStarOverrides((map) => ({ ...map, [concept.id]: !next })))
  }

  function filterLabel(value: Filter) {
    switch (value) {
      case 'unmarked':
        return t.unmarked
      case 'review':
        return t.needsReview
      case 'proficient':
        return t.proficient
      default:
        return t.filterAll
    }
  }

  if (!started) {
    return (
      <section className="page">
        <Header eyebrow={t.flashcards} title={t.flashcardsTitle} />
        <SetupCard>
          <label>{t.pickTopics}</label>
          <div className="scope-picker">
            {units.map((unit) => {
              const pickedInUnit = unit.topics.filter((topic) => picked.has(topic.id)).length
              const isOpen = expanded.has(unit.id)
              return (
                <div className={`scope-unit ${isOpen ? 'open' : ''}`} key={unit.id}>
                  <div
                    className="scope-unit-head"
                    role="button"
                    tabIndex={0}
                    aria-expanded={isOpen}
                    onClick={() => toggleExpand(unit.id)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault()
                        toggleExpand(unit.id)
                      }
                    }}
                  >
                    <span className="scope-unit-toggle">
                      <ChevronRight size={15} className={`scope-chev ${isOpen ? 'open' : ''}`} />
                      <strong>{unit.title}</strong>
                    </span>
                    <span className="action-row">
                      {pickedInUnit > 0 && (
                        <small>
                          {pickedInUnit} / {unit.topics.length}
                        </small>
                      )}
                      <button
                        className="link-button"
                        onClick={(event) => {
                          event.stopPropagation()
                          toggleUnit(unit)
                        }}
                      >
                        {pickedInUnit === unit.topics.length && unit.topics.length > 0 ? t.clearAll : t.selectAll}
                      </button>
                    </span>
                  </div>
                  {isOpen && (
                    <div className="scope-topics">
                      {unit.topics.map((topic) => (
                        <label key={topic.id} className={`scope-topic ${picked.has(topic.id) ? 'picked' : ''}`}>
                          <input type="checkbox" checked={picked.has(topic.id)} onChange={() => toggleTopic(topic.id)} />
                          {topic.title}
                        </label>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
          <div className="scope-summary">
            <span>
              {picked.size === 0 ? `${t.filterAll} · ${totalTopics}` : `${picked.size} ${t.topicsSelected}`}
            </span>
            {picked.size > 0 && (
              <button className="link-button" onClick={() => setPicked(new Set())}>
                {t.clearAll}
              </button>
            )}
          </div>
          <div className="scope-grid">
            <label>
              {t.order}
              <div className="segmented">
                <button className={order === 'outline' ? 'active' : ''} onClick={() => setOrder('outline')}>
                  {t.outlineOrder}
                </button>
                <button className={order === 'random' ? 'active' : ''} onClick={() => setOrder('random')}>
                  {t.random}
                </button>
              </div>
            </label>
            <label>
              {t.filterAll}
              <div className="segmented four">
                {FILTER_VALUES.map((value) => (
                  <button key={value} className={filter === value ? 'active' : ''} onClick={() => setFilter(value)}>
                    {filterLabel(value)}
                  </button>
                ))}
              </div>
            </label>
          </div>
          <div className="session-bar">
            <div className="session-count">
              <strong>{unitsPending ? '…' : totalAvailable}</strong>
              <span>
                {t.cardsAvailable}
                {!unitsPending && totalAvailable > 200 ? ` · ${t.sessionMax}` : ''}
              </span>
            </div>
            <div className="session-controls">
              <button className="primary" onClick={() => void start()} disabled={loading || unitsPending || conceptsPending || totalAvailable === 0}>
                <Play size={16} /> {t.start}
              </button>
            </div>
          </div>
          {error && <div className="error">{error}</div>}
        </SetupCard>
      </section>
    )
  }

  if (!current) {
    return (
      <section className="page">
        <Header
          eyebrow={t.flashcards}
          title={t.sessionDone}
          action={
            <button className="secondary" onClick={() => setStarted(false)}>
              <RotateCcw size={16} /> {t.newSession}
            </button>
          }
        />
        <div className="metrics">
          <Metric label={t.reviewed} value={Object.values(tally).reduce((sum, value) => sum + value, 0)} />
          <Metric label={t.proficient} value={tally.proficient ?? 0} />
          <Metric label={t.fuzzy} value={tally.fuzzy ?? 0} />
          <Metric label={t.unknown} value={tally.unknown ?? 0} />
        </div>
        <div className="empty-state celebrate">
          <strong>{t[praiseKey()]}</strong>
          <button className="primary" onClick={() => navigate('/terms')}>
            {t.glossary}
          </button>
        </div>
      </section>
    )
  }

  return (
    <section className="page focus-page">
      <Header
        eyebrow={t.flashcards}
        title={`${index + 1} / ${queue.length}`}
        action={
          <div className="action-row">
            {pendingCount > 0 && <span className="sync-badge">{t.syncPending.replace('{n}', String(pendingCount))}</span>}
            <KeyboardHint text={t.flipHint} />
          </div>
        }
      />
      <div className={`flashcard ${flipped ? 'flipped' : ''}`} onClick={() => setFlipped((value) => !value)}>
        <div className="flashcard-inner">
          <div className="flashcard-face flashcard-front">
            <small>{current.topic?.title}</small>
            <h2>{current.term}</h2>
            <p className="muted">{t.tapToFlip}</p>
              <button
                className={`card-star ${starOverrides[current.id] ?? current.state.starred ? 'on' : ''}`}
                onClick={(event) => {
                  event.stopPropagation()
                  toggleCardStar(current)
                }}
                aria-label={t.star}
                title={t.star}
              >
                <Star size={18} fill={(starOverrides[current.id] ?? current.state.starred) ? 'currentColor' : 'none'} />
            </button>
          </div>
          <div className="flashcard-face flashcard-back">
            <small>{current.term}</small>
            <div className="flashcard-back-scroll">
              <RichContent content={current.content} />
            </div>
          </div>
        </div>
      </div>
      {error && <div className="error">{error}</div>}
      <div className="flashcard-actions">
        <button
          className="icon-btn back"
          onClick={goPrevious}
          disabled={index === 0}
          title={t.previousCard}
          aria-label={t.previousCard}
        >
          <ArrowLeft size={18} />
        </button>
        <StatusButtons status="" onMark={(status) => respond(status)} size="large" />
        {/* On the last card this becomes "finish review" and leads to the
            summary — the session must be endable without forcing a mark. */}
        <button className="secondary next-card" onClick={goNext} title={index >= queue.length - 1 ? t.finishSession : t.nextCard}>
          {index >= queue.length - 1 ? t.finishSession : t.nextCard} <ChevronRight size={18} />
        </button>
      </div>
      {blocker.state === 'blocked' && (
        <div className="modal-backdrop">
          <div className="modal leave-modal" role="dialog" aria-modal="true" onClick={(event) => event.stopPropagation()}>
            <h3>{t.leaveTitle}</h3>
            <p className="muted">{t.leaveBody}</p>
            <div className="action-row">
              <button className="secondary" onClick={() => blocker.reset()}>
                {t.keepReviewing}
              </button>
              <button className="danger" onClick={() => blocker.proceed()}>
                {t.leaveNow}
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  )
}
