import { useState } from 'react'
import { Link } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { Clock3, History, NotebookPen, Search, Zap } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { Header, ListSkeleton } from '../components/ui'
import { useUnits } from '../components/ScopePicker'
import type { PracticeSet } from '../lib/types'

type SetRow = PracticeSet & { unitIds?: string[]; formats?: string[] }

export function Practice() {
  const { t } = useSession()
  const { data: units = [] } = useUnits()
  const [search, setSearch] = useState('')
  const [unitId, setUnitId] = useState('')
  const [format, setFormat] = useState('')
  const [status, setStatus] = useState('')
  const { data: sets = [], isPending } = useQuery({
    queryKey: ['practice-sets'],
    queryFn: async () => (await api.request<SetRow[] | null>('/api/practice/sets')) ?? [],
  })

  const filtered = sets.filter((set) => {
    if (search.trim() && !set.title.toLowerCase().includes(search.trim().toLowerCase())) return false
    if (unitId && !(set.unitIds ?? []).includes(unitId)) return false
    if (format && !(set.formats ?? []).includes(format)) return false
    if (status === 'unfinished' && !(set.attempts ?? []).some((attempt) => !attempt.finishedAt)) return false
    if (status === 'done' && !(set.attempts ?? []).some((attempt) => attempt.finishedAt)) return false
    return true
  })

  return (
    <section className="page">
      <Header
        eyebrow={t.practice}
        title={t.practiceTitle}
        action={
          <Link className="secondary" to="/practice/history">
            <History size={16} /> {t.historyTitle}
          </Link>
        }
      />
      {isPending ? (
        <ListSkeleton rows={4} />
      ) : (
        <>
          {sets.length > 0 && (
            <div className="filter-bar">
              <label className="search">
                <Search size={16} />
                <input placeholder={t.search} value={search} onChange={(e) => setSearch(e.target.value)} />
              </label>
              <select value={unitId} onChange={(e) => setUnitId(e.target.value)}>
                <option value="">{t.allUnits}</option>
                {units.map((unit) => (
                  <option key={unit.id} value={unit.id}>
                    {unit.title}
                  </option>
                ))}
              </select>
              <select value={format} onChange={(e) => setFormat(e.target.value)}>
                <option value="">{t.allFormats}</option>
                <option value="mcq">{t.mcq}</option>
                <option value="frq">FRQ</option>
                <option value="aaq">AAQ</option>
                <option value="ebq">EBQ</option>
              </select>
              <select value={status} onChange={(e) => setStatus(e.target.value)}>
                <option value="">{t.allStatuses}</option>
                <option value="unfinished">{t.inProgress}</option>
                <option value="done">{t.completed}</option>
              </select>
            </div>
          )}
          {filtered.length === 0 ? (
            <div className="empty-state">
              <NotebookPen size={28} />
              <p className="muted">{sets.length === 0 ? t.noSets : t.noSetsMatch}</p>
            </div>
          ) : (
            <div className="set-grid">
              {filtered.map((set) => {
                const attempts = set.attempts ?? []
                const hasUnfinished = attempts.some((attempt) => !attempt.finishedAt)
                return (
                  <Link className="set-card" key={set.id} to={`/practice/${set.id}`}>
                    <div className="set-head">
                      <strong>{set.title}</strong>
                      <span className={`pill ${set.mode === 'exam' ? 'warn' : 'ok'}`}>
                        {set.mode === 'exam' ? <Clock3 size={13} /> : <Zap size={13} />}
                        {set.mode === 'exam' ? t.examMode : t.instantMode}
                        {set.mode === 'exam' && set.timeLimitSec ? ` · ${Math.round(set.timeLimitSec / 60)} ${t.minutes}` : ''}
                      </span>
                    </div>
                    {set.description && <p>{set.description}</p>}
                    {(set.unitIds ?? []).length > 0 && (
                      <div className="chip-row">
                        {(set.unitIds ?? []).slice(0, 4).map((id) => {
                          const unit = units.find((candidate) => candidate.id === id)
                          return unit ? (
                            <span className="chip" key={id}>
                              {unit.title}
                            </span>
                          ) : null
                        })}
                        {(set.formats ?? []).includes('aaq') && <span className="chip">AAQ</span>}
                        {(set.formats ?? []).includes('ebq') && <span className="chip">EBQ</span>}
                      </div>
                    )}
                    <div className="set-meta">
                      <span>
                        {set.questionCount} {t.questions}
                      </span>
                      {attempts.length > 0 && (
                        <span>
                          {attempts.length} {t.attemptsCount}
                        </span>
                      )}
                      {set.bestScore != null && set.bestScore >= 0 && attempts[0]?.totalMcq ? (
                        <span>
                          {t.best}: {set.bestScore}/{attempts[0].totalMcq}
                        </span>
                      ) : null}
                    </div>
                    <div className="set-action">
                      <span className="primary">{hasUnfinished ? t.resume : attempts.length > 0 ? t.retake : t.startPractice}</span>
                    </div>
                  </Link>
                )
              })}
            </div>
          )}
        </>
      )}
    </section>
  )
}
