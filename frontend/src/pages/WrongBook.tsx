import { useState } from 'react'
import { Link } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import { formatDateTime } from '../lib/format'
import { useSession } from '../hooks/session'
import { Header, ListSkeleton } from '../components/ui'
import { praiseKey } from '../lib/i18n'
import { useUnits } from '../components/ScopePicker'
import type { WrongEntry } from '../lib/types'

export function WrongBook() {
  const { t } = useSession()
  const { data: units = [] } = useUnits()
  const [unitFilter, setUnitFilter] = useState('')
  const params = new URLSearchParams()
  if (unitFilter) params.set('unitId', unitFilter)
  const { data, isPending } = useQuery({
    queryKey: ['wrongbook', unitFilter],
    queryFn: async () => (await api.request<WrongEntry[] | null>(`/api/practice/wrongbook?${params.toString()}`)) ?? [],
  })
  const entries = data ?? []
  const praise = t[praiseKey()]

  return (
    <section className="page">
      <Header
        eyebrow={t.wrongbook}
        title={t.wrongbookTitle}
        action={
          <select value={unitFilter} onChange={(e) => setUnitFilter(e.target.value)}>
            <option value="">{t.allUnits}</option>
            {units.map((unit) => (
              <option key={unit.id} value={unit.id}>
                {unit.title}
              </option>
            ))}
          </select>
        }
      />
      {isPending ? (
        <ListSkeleton rows={5} />
      ) : entries.length === 0 ? (
        <div className="empty-state celebrate">
          <strong>{t.wrongbookEmpty}</strong>
          <span className="muted">{praise}</span>
        </div>
      ) : (
        <div className="table">
          {entries.map((entry) => (
            <div className="wrong-row" key={entry.question.id}>
              <div className="wrong-main">
                <div className="wrong-meta">
                  <span className={`pill ${entry.reason === 'wrong' ? 'bad' : 'warn'}`}>
                    {entry.reason === 'wrong' ? t.wrongReason : t.weakReason}
                  </span>
                  {entry.wrongCount > 1 && (
                    <small>
                      {entry.wrongCount} {t.wrongTimes}
                    </small>
                  )}
                  <small>{formatDateTime(entry.lastAt)}</small>
                  {(entry.question.tags ?? []).map((tag) => (
                    <span className="pill" key={tag.id}>
                      {tag.name}
                    </span>
                  ))}
                </div>
                <strong>{entry.question.stem}</strong>
                {entry.question.type === 'mcq' && (
                  <div className="wrong-detail">
                    {(entry.question.choices ?? []).map((choice) => (
                      <p key={choice.key} className={choice.key === entry.question.answerKey ? 'key-answer' : ''}>
                        <strong>{choice.key}.</strong> {choice.text}
                        {choice.key === entry.question.answerKey ? ' ✓' : ''}
                      </p>
                    ))}
                    {entry.question.explanation && <p className="muted">{entry.question.explanation}</p>}
                  </div>
                )}
                {entry.question.type === 'subjective' && (
                  <div className="wrong-detail">
                    {(entry.question.parts ?? []).map((part) => (
                      <p key={part.label}>
                        <strong>{part.label}.</strong> {part.referenceAnswer}
                      </p>
                    ))}
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      <div className="runner-footer">
        <Link className="secondary" to="/practice">
          {t.practiceAgain}
        </Link>
      </div>
    </section>
  )
}
