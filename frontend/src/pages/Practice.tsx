import { Link } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { ClipboardList, Clock3, Zap } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { Header, ListSkeleton } from '../components/ui'
import type { PracticeSet } from '../lib/types'

export function Practice() {
  const { t } = useSession()
  const { data: sets = [], isPending } = useQuery({
    queryKey: ['practice-sets'],
    queryFn: () => api.request<PracticeSet[]>('/api/practice/sets'),
  })

  return (
    <section className="page">
      <Header eyebrow={t.practice} title={t.practiceTitle} />
      {isPending ? (
        <ListSkeleton rows={4} />
      ) : sets.length === 0 ? (
        <div className="empty-state">{t.noSets}</div>
      ) : (
        <div className="set-grid">
          {sets.map((set) => {
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
    </section>
  )
}

export function PracticeIcon() {
  return <ClipboardList size={16} />
}
