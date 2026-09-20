import { Link } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Clock3, History, Zap } from 'lucide-react'
import { api } from '../lib/api'
import { formatDateTime } from '../lib/format'
import { useSession } from '../hooks/session'
import { Header, ListSkeleton } from '../components/ui'
import type { PracticeAttempt } from '../lib/types'

// 做题记录: every attempt of the signed-in student, newest first. Finished
// attempts open a read-only review; unfinished ones jump back into the
// runner to resume.
type AttemptRow = PracticeAttempt & {
  setTitle: string
  questionCount: number
  answeredCount: number
}

export function PracticeHistory() {
  const { t } = useSession()
  const { data: attempts = [], isPending } = useQuery({
    queryKey: ['practice-attempts'],
    queryFn: async () => (await api.request<AttemptRow[] | null>('/api/practice/attempts?limit=100')) ?? [],
  })

  return (
    <section className="page">
      <Header
        eyebrow={t.practice}
        title={t.historyTitle}
        action={
          <Link className="secondary" to="/practice">
            <ArrowLeft size={16} /> {t.practice}
          </Link>
        }
      />
      {isPending ? (
        <ListSkeleton rows={5} />
      ) : attempts.length === 0 ? (
        <div className="empty-state">
          <History size={28} />
          <p className="muted">{t.noHistory}</p>
        </div>
      ) : (
        <div className="table">
          {attempts.map((attempt) => {
            const finished = !!attempt.finishedAt
            const target = finished ? `/practice/history/${attempt.id}` : `/practice/${attempt.setId}`
            return (
              <div className="concept-row" key={attempt.id}>
                <Link className="row-main" to={target}>
                  <span>
                    <strong>{attempt.setTitle || '—'}</strong>
                    <small>
                      {formatDateTime(attempt.startedAt)} ·{' '}
                      {attempt.mode === 'exam' ? (
                        <>
                          <Clock3 size={12} /> {t.examMode}
                        </>
                      ) : (
                        <>
                          <Zap size={12} /> {t.instantMode}
                        </>
                      )}{' '}
                      · {attempt.answeredCount}/{attempt.questionCount} {t.questions}
                      {finished && attempt.score != null && attempt.totalMcq > 0
                        ? ` · ${t.score} ${attempt.score}/${attempt.totalMcq}`
                        : ''}
                    </small>
                  </span>
                </Link>
                <div className="row-actions">
                  <span className={`pill ${finished ? 'ok' : 'warn'}`}>{finished ? t.completed : t.inProgress}</span>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}
