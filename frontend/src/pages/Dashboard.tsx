import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { BarChart3, ClipboardList, History, Layers, ListChecks, Sparkles, TrendingUp } from 'lucide-react'
import { api } from '../lib/api'
import { examCountdown, percent, shortLabel } from '../lib/format'
import { useSession } from '../hooks/session'
import { Header, Metric } from '../components/ui'
import type { DashboardAlerts, DashboardProgress, DashboardSummary, StatBucket } from '../lib/types'

export function Dashboard() {
  const { t, meta } = useSession()
  const navigate = useNavigate()
  const [now, setNow] = useState(Date.now())
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [])

  const summary = useQuery({ queryKey: ['dashboard-summary'], queryFn: () => api.request<DashboardSummary>('/api/dashboard/summary') })
  const progress = useQuery({ queryKey: ['dashboard-progress'], queryFn: () => api.request<DashboardProgress>('/api/dashboard/progress') })
  const trends = useQuery({
    queryKey: ['dashboard-trends'],
    queryFn: () => api.request<{ daily: StatBucket[]; hourly: StatBucket[] }>('/api/dashboard/trends'),
  })
  const alerts = useQuery({ queryKey: ['dashboard-alerts'], queryFn: () => api.request<DashboardAlerts>('/api/dashboard/alerts') })

  const countdown = examCountdown(meta?.examDate || undefined, now)
  const total = summary.data?.totalConcepts ?? 0
  const marked = progress.data?.markedConcepts ?? 0
  const tierBar = (value: number, tone: string) => (
    <div className="tier-bar" title={`${value}`}>
      <i className={tone} style={{ width: `${percent(value, Math.max(total, 1))}%` }} />
    </div>
  )

  return (
    <section className="page">
      <Header
        eyebrow={t.dashboard}
        title={t.cockpit}
        action={
          <div className="action-row">
            <button className="secondary" onClick={() => navigate('/practice')}>
              <ClipboardList size={16} /> {t.browsePractice}
            </button>
            <button className="primary" onClick={() => navigate('/flashcards')}>
              <Sparkles size={16} /> {t.startFlashcards}
            </button>
          </div>
        }
      />
      <div className="metrics">
        <Metric label={t.todayReviews} value={progress.data?.todayReviews ?? 0} loading={!progress.data} />
        <Metric label={t.shortTerm} value={progress.data?.shortTermReviews ?? 0} loading={!progress.data} />
        <Metric label={t.streak} value={progress.data?.streakDays ?? 0} loading={!progress.data} />
        <Metric label={t.marked} value={`${marked} / ${total}`} loading={!progress.data} />
      </div>
      <div className="dashboard-grid">
        <div className="progress-band">
          <div>
            <span>{t.coverage}</span>
            <strong>
              {t.proficient} {progress.data?.proficientConcepts ?? 0} · {t.fuzzy} {progress.data?.fuzzyConcepts ?? 0} ·{' '}
              {t.unknown} {progress.data?.unknownConcepts ?? 0}
            </strong>
          </div>
          {tierBar(progress.data?.proficientConcepts ?? 0, 'ok')}
          {tierBar(progress.data?.fuzzyConcepts ?? 0, 'warn')}
          {tierBar(progress.data?.unknownConcepts ?? 0, 'bad')}
        </div>
        {countdown && (
          <div className="countdown-card">
            <span>{t.countdown}</span>
            <strong>
              {countdown.days}d {countdown.hours}h {countdown.minutes}m {countdown.seconds}s
            </strong>
            <small>{meta?.examDate}</small>
          </div>
        )}
      </div>
      <div className="reminder-card">
        <div>
          <span>{t.weakTopics}</span>
          <strong>{alerts.data?.weakConcepts?.[0]?.term ?? t.noWeakAreas}</strong>
        </div>
        <button className="secondary" onClick={() => navigate('/flashcards')}>
          <Layers size={16} /> {t.startFlashcards}
        </button>
      </div>
      <div className="chart-grid three">
        <ChartCard title={t.dailyReviews} icon={<BarChart3 size={17} />} data={trends.data?.daily ?? []} mode="reviews" />
        <ChartCard title={t.last24h} icon={<TrendingUp size={17} />} data={trends.data?.hourly ?? []} mode="stacked" />
        <ChartCard title={t.reviewed} icon={<History size={17} />} data={trends.data?.daily ?? []} mode="cumulative" />
      </div>
      <div className="insight-grid">
        <WeakAreaCard title={t.weakUnits} icon={<ListChecks size={17} />} rows={alerts.data?.weakUnits ?? []} empty={t.noWeakAreas} />
        <WeakAreaCard title={t.weakTopics} icon={<ListChecks size={17} />} rows={alerts.data?.weakTopics ?? []} empty={t.noWeakAreas} />
      </div>
    </section>
  )
}

function ChartCard({
  title,
  icon,
  data,
  mode,
}: {
  title: string
  icon: React.ReactNode
  data: StatBucket[]
  mode: 'reviews' | 'stacked' | 'cumulative'
}) {
  const { t } = useSession()
  const values = data.map((row) => chartValue(row, mode))
  const max = Math.max(1, ...values)
  return (
    <div className="chart-card">
      <h3>
        {icon}
        {title}
      </h3>
      {data.length === 0 || values.every((value) => value === 0) ? (
        <p className="muted">{t.empty}</p>
      ) : (
        <div className={`bars ${mode}`}>
          {data.map((row) => {
            const value = chartValue(row, mode)
            return (
              <div key={row.label} title={`${row.label}: ${value}`}>
                <i style={{ height: `${value > 0 ? Math.max(8, (value / max) * 100) : 0}%` }} />
                <span>{shortLabel(row.label)}</span>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

function chartValue(row: StatBucket, mode: 'reviews' | 'stacked' | 'cumulative') {
  if (mode === 'reviews') return row.reviews
  if (mode === 'cumulative') return row.reviews
  return row.reviews
}

function WeakAreaCard({ title, icon, rows, empty }: { title: string; icon: React.ReactNode; rows: { label: string; weak: number; marked: number }[]; empty: string }) {
  return (
    <div className="insight-card">
      <h3>
        {icon}
        {title}
      </h3>
      {rows.length === 0 ? (
        <p className="muted">{empty}</p>
      ) : (
        <div className="weak-list">
          {rows.map((row) => (
            <div key={row.label}>
              <span>{row.label}</span>
              <strong>{row.weak}</strong>
              <small>
                {row.weak}/{row.marked}
              </small>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
