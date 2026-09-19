import { Link } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { Clock3, NotebookPen, PenLine, Zap } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { Header, ListSkeleton } from '../components/ui'
import type { PracticeSet } from '../lib/types'

export function Writing() {
  const { t } = useSession()
  const { data: sets = [], isPending } = useQuery({
    queryKey: ['practice-sets'],
    queryFn: async () => (await api.request<PracticeSet[] | null>('/api/practice/sets')) ?? [],
  })

  return (
    <section className="page">
      <Header eyebrow={t.practice} title={t.writingTitle} />
      <p className="muted">{t.writingHint}</p>
      {isPending ? (
        <ListSkeleton rows={4} />
      ) : sets.length === 0 ? (
        <div className="empty-state">
          <NotebookPen size={28} />
          <p className="muted">{t.noSets}</p>
        </div>
      ) : (
        <div className="set-grid">
          {sets.map((set) => (
            <Link className="set-card" key={set.id} to={`/practice/${set.id}/writing`}>
              <div className="set-head">
                <strong>{set.title}</strong>
                <span className={`pill ${set.mode === 'exam' ? 'warn' : 'ok'}`}>
                  {set.mode === 'exam' ? <Clock3 size={13} /> : <Zap size={13} />}
                  {set.mode === 'exam' ? t.examMode : t.instantMode}
                  {set.mode === 'exam' && set.timeLimitSec ? ` · ${Math.round(set.timeLimitSec / 60)} ${t.minutes}` : ''}
                </span>
              </div>
              {set.description && <p>{set.description}</p>}
              <div className="set-action">
                <span className="primary">
                  <PenLine size={15} /> {t.start}
                </span>
              </div>
            </Link>
          ))}
        </div>
      )}
    </section>
  )
}
