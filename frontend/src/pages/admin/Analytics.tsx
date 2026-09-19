import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { api } from '../../lib/api'
import { formatDateTime, percent } from '../../lib/format'
import { useSession } from '../../hooks/session'
import { Header, Metric } from '../../components/ui'
import type { AnalyticsOverview, AnalyticsUserDetail } from '../../lib/types'

export function Analytics() {
  const { t } = useSession()
  const [studentId, setStudentId] = useState('')
  const { data: overview } = useQuery({
    queryKey: ['analytics-overview'],
    queryFn: () => api.request<AnalyticsOverview>('/api/admin/analytics/overview'),
  })

  if (studentId) {
    return <StudentDetail userId={studentId} onBack={() => setStudentId('')} />
  }

  const flashcardStatus = overview?.flashcard ?? []
  const tierCount = (status: string) => flashcardStatus.find((row) => row.status === status)?.count ?? 0
  const totalMarked = flashcardStatus.reduce((sum, row) => (row.status ? sum + row.count : sum), 0)
  const accuracyByUnit = overview?.accuracyByUnit ?? []
  const students = overview?.students ?? []

  return (
    <div>
      <Header eyebrow={t.admin} title={t.analyticsTitle} />
      <div className="metrics">
        <Metric label={t.activeUsers} value={overview?.activeUsers7d ?? 0} loading={!overview} />
        <Metric label={t.totalAttempts} value={overview?.practice.attempts ?? 0} loading={!overview} />
        <Metric label={t.answeredQuestions} value={overview?.practice.answered ?? 0} loading={!overview} />
        <Metric label={t.classAccuracy} value={`${percent(overview?.practice.correct ?? 0, overview?.practice.answered ?? 0)}%`} loading={!overview} />
      </div>
      <div className="dashboard-grid">
        <div className="chart-card">
          <h3>{t.accuracyByUnit}</h3>
          {accuracyByUnit.length === 0 ? (
            <p className="muted">{t.empty}</p>
          ) : (
            <div className="accuracy-list">
              {accuracyByUnit.map((row) => (
                <div key={row.id || row.label}>
                  <span>{row.label}</span>
                  <div className="accuracy-bar">
                    <i style={{ width: `${Math.round(row.accuracy * 100)}%` }} />
                  </div>
                  <small>
                    {row.correct}/{row.answered}
                  </small>
                </div>
              ))}
            </div>
          )}
        </div>
        <div className="chart-card">
          <h3>{t.flashcardDistribution}</h3>
          <div className="tier-legend">
            <span className="pill ok">
              {t.proficient} {tierCount('proficient')}
            </span>
            <span className="pill warn">
              {t.fuzzy} {tierCount('fuzzy')}
            </span>
            <span className="pill bad">
              {t.unknown} {tierCount('unknown')}
            </span>
            <span className="pill">
              {t.unmarked} {tierCount('')}
            </span>
            <small className="muted">
              {totalMarked} {t.marked}
            </small>
          </div>
          <h3 style={{ marginTop: 18 }}>{t.weakestConcepts}</h3>
          <div className="weak-list">
            {(overview?.weakConcepts ?? []).map((row) => (
              <div key={row.term}>
                <span>{row.term}</span>
                <strong>{row.weak}</strong>
                <small>{row.unit.replace(/^Unit \d+: /, '')}</small>
              </div>
            ))}
            {(overview?.weakConcepts ?? []).length === 0 && <p className="muted">{t.empty}</p>}
          </div>
        </div>
      </div>
      <h3 className="section-title">
        {t.studentsTitle} ({students.length})
      </h3>
      <div className="student-table">
        <div className="student-row head">
          <span>{t.student}</span>
          <span>{t.practice}</span>
          <span>{t.accuracy}</span>
          <span>{t.markedConcepts}</span>
          <span />
        </div>
        {students.map((student) => (
          <div className="student-row" key={student.id}>
            <span>
              <strong>{student.name}</strong>
              <small>{student.email}</small>
            </span>
            <span>{student.attempts}</span>
            <span>{student.answered > 0 ? `${Math.round(student.accuracy * 100)}%` : '—'}</span>
            <span>{student.marked}</span>
            <button className="secondary" onClick={() => setStudentId(student.id)}>
              {t.viewStudent}
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}

function StudentDetail({ userId, onBack }: { userId: string; onBack: () => void }) {
  const { t } = useSession()
  const { data, isError } = useQuery({
    queryKey: ['analytics-user', userId],
    queryFn: () => api.request<AnalyticsUserDetail>(`/api/admin/analytics/users/${userId}`),
  })
  if (isError) {
    return (
      <div>
        <button className="secondary" onClick={onBack}>
          <ArrowLeft size={16} /> {t.backToOverview}
        </button>
        <div className="error" style={{ marginTop: 12 }}>{t.errorGeneric}</div>
      </div>
    )
  }
  if (!data) return <p className="muted">{t.loading}…</p>
  const flashcard = data.flashcard ?? []
  const accuracyByUnit = data.accuracyByUnit ?? []
  const attempts = data.attempts ?? []
  const tierCount = (status: string) => flashcard.find((row) => row.status === status)?.count ?? 0
  return (
    <div>
      <Header
        eyebrow={t.adminAnalytics}
        title={data.user.name}
        action={
          <button className="secondary" onClick={onBack}>
            <ArrowLeft size={16} /> {t.backToOverview}
          </button>
        }
      />
      <div className="metrics">
        <Metric label={t.streakDays} value={data.streakDays} />
        <Metric label={t.wrongAnswers} value={data.wrongAnswers} />
        <Metric label={t.proficient} value={tierCount('proficient')} />
        <Metric label={t.fuzzy} value={tierCount('fuzzy')} />
      </div>
      <div className="dashboard-grid">
        <div className="chart-card">
          <h3>{t.accuracyByUnit}</h3>
          {accuracyByUnit.length === 0 ? (
            <p className="muted">{t.empty}</p>
          ) : (
            <div className="accuracy-list">
              {accuracyByUnit.map((row) => (
                <div key={row.id || row.label}>
                  <span>{row.label}</span>
                  <div className="accuracy-bar">
                    <i style={{ width: `${Math.round(row.accuracy * 100)}%` }} />
                  </div>
                  <small>
                    {row.correct}/{row.answered}
                  </small>
                </div>
              ))}
            </div>
          )}
        </div>
        <div className="chart-card">
          <h3>{t.practice}</h3>
          <div className="attempt-list">
            {attempts.map((attempt) => (
              <div key={attempt.id}>
                <span>{attempt.setTitle}</span>
                <small>{formatDateTime(attempt.startedAt)}</small>
                <strong>
                  {attempt.score ?? '—'}/{attempt.totalMcq || '—'}
                </strong>
              </div>
            ))}
            {attempts.length === 0 && <p className="muted">{t.empty}</p>}
          </div>
        </div>
      </div>
    </div>
  )
}
