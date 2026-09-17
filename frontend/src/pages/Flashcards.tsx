import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { Check, Play, RotateCcw } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { Header, KeyboardHint, Metric, SetupCard } from '../components/ui'
import { RichContent } from '../components/RichContent'
import { ScopePicker } from '../components/ScopePicker'
import { StatusButtons } from '../components/StatusButtons'
import type { Concept, ConceptStatus } from '../lib/types'

type Filter = 'unmarked' | 'marked' | 'proficient' | 'fuzzy' | 'unknown' | 'all'

function filterLabel(t: ReturnType<typeof useSession>['t'], value: Filter) {
  switch (value) {
    case 'unmarked':
      return t.unmarked
    case 'marked':
      return t.marked
    case 'proficient':
      return t.proficient
    case 'fuzzy':
      return t.fuzzy
    case 'unknown':
      return t.unknown
    default:
      return t.filterAll
  }
}

export function Flashcards() {
  const { t } = useSession()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [unitId, setUnitId] = useState('')
  const [topicId, setTopicId] = useState('')
  const [limit, setLimit] = useState(30)
  const [order, setOrder] = useState<'random' | 'outline'>('random')
  const [filter, setFilter] = useState<Filter>('unmarked')
  const [queue, setQueue] = useState<Concept[]>([])
  const [index, setIndex] = useState(0)
  const [flipped, setFlipped] = useState(false)
  const [started, setStarted] = useState(false)
  const [loading, setLoading] = useState(false)
  const [tally, setTally] = useState<Record<string, number>>({})
  const current = queue[index]

  const params = useMemo(() => {
    const search = new URLSearchParams({ limit: String(Math.min(200, Math.max(5, limit + 40))), order })
    if (topicId) search.set('topicId', topicId)
    else if (unitId) search.set('unitId', unitId)
    if (filter !== 'all') search.set('status', filter)
    return search.toString()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [unitId, topicId, limit, order, filter])

  useEffect(() => {
    if (!started || !current) return
    function handleKey(event: KeyboardEvent) {
      if (event.key === ' ' || event.key === 'Enter') {
        event.preventDefault()
        setFlipped((value) => !value)
        return
      }
      if (event.key === '1') void respond('proficient')
      if (event.key === '2') void respond('fuzzy')
      if (event.key === '3') void respond('unknown')
    }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [started, current])

  async function start() {
    setLoading(true)
    try {
      const rows = await api.request<Concept[]>(`/api/review/next?${params}`)
      setQueue(rows.slice(0, limit))
      setIndex(0)
      setFlipped(false)
      setTally({})
      setStarted(true)
    } finally {
      setLoading(false)
    }
  }

  async function respond(response: ConceptStatus) {
    if (!current || !response) return
    await api.request('/api/review/events', {
      method: 'POST',
      body: JSON.stringify({ conceptId: current.id, response }),
    })
    setTally((counts) => ({ ...counts, [response]: (counts[response] ?? 0) + 1 }))
    setFlipped(false)
    setIndex((value) => value + 1)
    void queryClient.invalidateQueries({ queryKey: ['dashboard'] })
  }

  if (!started) {
    return (
      <section className="page">
        <Header eyebrow={t.flashcards} title={t.flashcardsTitle} />
        <SetupCard>
          <ScopePicker unitId={unitId} topicId={topicId} setUnitId={setUnitId} setTopicId={setTopicId} />
          <div className="scope-grid">
            <label>
              {t.wordsPerSession}
              <input type="number" min={5} max={200} value={limit} onChange={(e) => setLimit(Number(e.target.value))} />
            </label>
            <label>
              {t.topic}
              <div className="segmented inline">
                <button className={order === 'random' ? 'active' : ''} onClick={() => setOrder('random')}>
                  {t.random}
                </button>
                <button className={order === 'outline' ? 'active' : ''} onClick={() => setOrder('outline')}>
                  {t.outlineOrder}
                </button>
              </div>
            </label>
          </div>
          <div className="segmented wrap">
            {(['unmarked', 'marked', 'proficient', 'fuzzy', 'unknown', 'all'] as Filter[]).map((value) => (
              <button key={value} className={filter === value ? 'active' : ''} onClick={() => setFilter(value)}>
                {filterLabel(t, value)}
              </button>
            ))}
          </div>
          <button className="primary" onClick={start} disabled={loading}>
            <Play size={16} /> {t.start}
          </button>
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
        <div className="empty-state">
          <Check size={28} />
          <button className="primary" onClick={() => navigate('/learn')}>
            {t.learn}
          </button>
        </div>
      </section>
    )
  }

  return (
    <section className="page focus-page">
      <Header eyebrow={t.flashcards} title={`${index + 1} / ${queue.length}`} action={<KeyboardHint text={t.flipHint} />} />
      <div className={`flashcard ${flipped ? 'flipped' : ''}`} onClick={() => setFlipped((value) => !value)}>
        <div className="flashcard-inner">
          <div className="flashcard-face flashcard-front">
            <small>{current.topic?.title}</small>
            <h2>{current.term}</h2>
            <p className="muted">{t.tapToFlip}</p>
          </div>
          <div className="flashcard-face flashcard-back">
            <small>{current.term}</small>
            <div className="flashcard-back-scroll">
              <RichContent content={current.content} />
            </div>
          </div>
        </div>
      </div>
      <StatusButtons status="" onMark={(status) => respond(status)} size="large" />
    </section>
  )
}
