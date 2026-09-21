import { useCallback, useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Download } from 'lucide-react'
import { api } from '../../lib/api'
import { formatDateTime } from '../../lib/format'
import { useSession } from '../../hooks/session'
import { Header, ListSkeleton, Metric, TablePager } from '../../components/ui'
import { BarsChart, ChartCard, DoughnutChart, HBarsChart, TrendChart } from '../../components/charts'
import type { Copy } from '../../lib/i18n'
import type { TelemetryActivity, TelemetryFeatures, TelemetryLogEvent, TelemetryLogPage } from '../../lib/types'

const DAY_OPTIONS = [7, 30, 90] as const
const STUDENT_PAGE_SIZE = 15

function featureLabel(name: string, t: Copy): string {
  switch (name) {
    case 'page.dashboard':
      return t.dashboardFeature
    case 'page.terms':
      return t.termsFeature
    case 'page.flashcards':
      return t.flashcardsFeature
    case 'page.notes':
      return t.notesFeature
    case 'page.practice':
      return t.practiceFeature
    case 'page.writing':
      return t.writingFeature
    case 'page.practice-runner':
      return t.practiceRunnerFeature
    case 'page.practice-history':
      return t.practiceHistoryFeature
    case 'page.wrongbook':
      return t.wrongbookFeature
    case 'page.profile':
      return t.profileFeature
    case 'note.open':
      return t.noteOpenFeature
    default:
      return name
  }
}

const EVENT_TYPES = ['login', 'heartbeat', 'page_view', 'feature'] as const

export function Activity() {
  const { t, isAdmin } = useSession()
  const [days, setDays] = useState<number>(30)
  const [studentPage, setStudentPage] = useState(1)

  const { data: activity } = useQuery({
    queryKey: ['admin-telemetry', 'activity', days],
    queryFn: () => api.request<TelemetryActivity>(`/api/admin/telemetry/activity?days=${days}`),
  })
  const { data: features } = useQuery({
    queryKey: ['admin-telemetry', 'features', days],
    queryFn: () => api.request<TelemetryFeatures>(`/api/admin/telemetry/features?days=${days}`),
  })

  const summary = activity?.summary
  const daily = activity?.daily ?? []
  const hours = (activity?.hours ?? []).map((row) => ({ hour: `${String(row.hour).padStart(2, '0')}:00`, events: row.events }))
  const students = activity?.students ?? []
  const studentPageCount = Math.max(1, Math.ceil(students.length / STUDENT_PAGE_SIZE))
  const safeStudentPage = Math.min(studentPage, studentPageCount)
  const pagedStudents = students.slice((safeStudentPage - 1) * STUDENT_PAGE_SIZE, safeStudentPage * STUDENT_PAGE_SIZE)
  const featureNames = features?.names ?? []
  const featureRows = features?.rows ?? []
  const hasFeatureData = featureRows.some((row) => featureNames.some((name) => Number(row[name] ?? 0) > 0))
  const providerColors = useProviderColors()
  // Zero-count providers (e.g. Entra never used) only add dead legend chips.
  const providers = (activity?.providers ?? []).filter((row) => row.count > 0).map((row, i) => ({
    name: row.provider === 'entra' ? t.providerEntra : row.provider === 'local' ? t.providerLocal : row.provider,
    value: row.count,
    color: providerColors[i % providerColors.length],
  }))
  const providerTotal = providers.reduce((sum, row) => sum + row.value, 0)

  return (
    <div>
      <Header
        eyebrow={t.admin}
        title={t.activityTitle}
        action={
          <div className="day-range-picker" role="group" aria-label={t.lastDays.replace('{n}', String(days))}>
            {DAY_OPTIONS.map((option) => (
              <button key={option} className={option === days ? 'active' : ''} onClick={() => setDays(option)}>
                {t.lastDays.replace('{n}', String(option))}
              </button>
            ))}
          </div>
        }
      />
      <div className="metrics">
        <Metric label={t.todayLogins} value={summary?.todayLogins ?? 0} loading={!activity} />
        <Metric label={t.todayActive} value={summary?.todayActive ?? 0} loading={!activity} />
        <Metric label={t.active7d} value={summary?.active7d ?? 0} loading={!activity} />
        <Metric label={t.inactive7d} value={summary?.inactive7d ?? 0} loading={!activity} />
      </div>
      <ChartCard title={t.loginTrend} subtitle={t.loginTrendSub} height={236}>
        {daily.some((row) => row.logins > 0 || row.dau > 0) ? (
          <TrendChart
            data={daily}
            xKey="date"
            series={[
              { key: 'logins', label: t.todayLogins, color: 'accent' },
              { key: 'dau', label: t.todayActive, color: 'green' },
            ]}
          />
        ) : (
          <p className="muted">{t.empty}</p>
        )}
      </ChartCard>
      <div className="dashboard-grid">
        <ChartCard title={t.activeHours} subtitle={t.activeHoursSub}>
          {hours.some((row) => row.events > 0) ? (
            <BarsChart
              data={hours}
              xKey="hour"
              series={[{ key: 'events', label: t.activeHours, color: 'accent' }]}
              tickFormatter={(value) => value.slice(0, 2)}
            />
          ) : (
            <p className="muted">{t.empty}</p>
          )}
        </ChartCard>
        <ChartCard title={t.loginProviders}>
          {providers.length > 0 ? (
            <DoughnutChart data={providers} centerValue={providerTotal} centerLabel={t.loginCount} />
          ) : (
            <p className="muted">{t.empty}</p>
          )}
        </ChartCard>
      </div>
      <ChartCard title={t.featureUsage} subtitle={t.featureUsageSub} height={260}>
        {hasFeatureData ? (
          <BarsChart
            data={featureRows}
            xKey="date"
            stacked
            series={featureNames.map((name, i) => ({
              key: name,
              label: featureLabel(name, t),
              color: i,
            }))}
          />
        ) : (
          <p className="muted">{t.empty}</p>
        )}
      </ChartCard>
      <div className="dashboard-grid">
        <ChartCard title={t.featureTotals} height={Math.max(200, featureNames.length * 40 + 60)}>
          {featureNames.length > 0 ? (
            <HBarsChart
              data={features?.totals.map((row) => ({ name: featureLabel(row.name, t), count: row.count })) ?? []}
              labelKey="name"
              valueKey="count"
              valueName={t.usageCount}
            />
          ) : (
            <p className="muted">{t.empty}</p>
          )}
        </ChartCard>
        <ChartCard title={t.notesRanking} height={Math.max(200, (features?.notes.length ?? 0) * 40 + 60)}>
          {(features?.notes ?? []).length > 0 ? (
            <HBarsChart
              data={(features?.notes ?? []).map((row) => ({ title: row.title || row.resourceId, opens: row.opens }))}
              labelKey="title"
              valueKey="opens"
              color="green"
              valueName={t.usageCount}
            />
          ) : (
            <p className="muted">{t.notesRankingEmpty}</p>
          )}
        </ChartCard>
      </div>
      <h3 className="section-title">
        {t.studentActivity} ({activity?.students.length ?? 0})
      </h3>
      <div className="activity-table">
        <div className="activity-row head">
          <span>{t.student}</span>
          <span>{t.recentLogin}</span>
          <span>{t.recentSeen}</span>
          <span>{t.loginCount}</span>
          <span>{t.activeDays}</span>
          <span>{t.inactiveDays}</span>
        </div>
        {pagedStudents.map((student) => (
          <div className="activity-row" key={student.id}>
            <span>
              <strong>{student.name}</strong>
              <small>{student.email}</small>
            </span>
            <span>{student.lastLoginAt ? formatDateTime(student.lastLoginAt) : '—'}</span>
            <span>{student.lastSeenAt ? formatDateTime(student.lastSeenAt) : '—'}</span>
            <span>{student.logins}</span>
            <span>{student.activeDays}</span>
            <span>{student.inactiveDays < 0 ? '—' : student.inactiveDays}</span>
          </div>
        ))}
        {activity && activity.students.length === 0 && <p className="muted">{t.empty}</p>}
        {activity && activity.students.length > STUDENT_PAGE_SIZE && (
          <TablePager page={safeStudentPage} pageCount={studentPageCount} total={activity.students.length} onChange={setStudentPage} />
        )}
      </div>
      {isAdmin && <RawLog students={activity?.students ?? []} />}
    </div>
  )
}

// Provider swatches cycle a fixed palette: the doughnut draws with concrete
// colors, and useChartTheme already re-reads on theme changes.
function useProviderColors(): string[] {
  const { theme } = useSession()
  return useMemo(
    () => (theme === 'dark' ? ['#c98ad9', '#4cc36a', '#ff8a3d'] : ['#883d92', '#1aae39', '#dd5b00']),
    [theme],
  )
}

const LOG_PAGE_SIZE = 20

function RawLog({ students }: { students: TelemetryActivity['students'] }) {
  const { t } = useSession()
  const [userId, setUserId] = useState('')
  const [type, setType] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [page, setPage] = useState(1)

  // Any filter change restarts from the first page.
  useEffect(() => {
    setPage(1)
  }, [userId, type, from, to])

  const filterParams = useCallback(() => {
    const params = new URLSearchParams()
    if (userId) params.set('userId', userId)
    if (type) params.set('type', type)
    if (from) params.set('from', from)
    if (to) params.set('to', to)
    return params
  }, [userId, type, from, to])

  const { data: logPage, isFetching } = useQuery({
    queryKey: ['admin-telemetry', 'log', userId, type, from, to, page],
    queryFn: () => {
      const params = filterParams()
      params.set('page', String(page))
      params.set('limit', String(LOG_PAGE_SIZE))
      return api.request<TelemetryLogPage>(`/api/admin/telemetry/log?${params.toString()}`)
    },
    placeholderData: (previous) => previous,
  })

  const total = logPage?.total ?? 0
  const pageCount = Math.max(1, Math.ceil(total / LOG_PAGE_SIZE))

  function exportCsv() {
    void api.download(`/api/admin/telemetry/log?format=csv&${filterParams().toString()}`, 'activity-log.csv')
  }

  function describe(event: TelemetryLogEvent) {
    if (event.meta && typeof event.meta === 'object') {
      const email = event.meta.email
      const reason = event.meta.reason
      if (typeof reason === 'string') return reason
      if (typeof email === 'string') return email
    }
    return event.path
  }

  return (
    <>
      <h3 className="section-title">{t.rawLog}</h3>
      <div className="log-filters">
        <select value={userId} onChange={(e) => setUserId(e.target.value)} aria-label={t.filterUser}>
          <option value="">{t.allUsers}</option>
          {students.map((student) => (
            <option key={student.id} value={student.id}>
              {student.name}
            </option>
          ))}
        </select>
        <select value={type} onChange={(e) => setType(e.target.value)} aria-label={t.filterType}>
          <option value="">{t.allTypes}</option>
          {EVENT_TYPES.map((value) => (
            <option key={value} value={value}>
              {value}
            </option>
          ))}
        </select>
        <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} aria-label={t.filterType} />
        <span className="muted">→</span>
        <input type="date" value={to} onChange={(e) => setTo(e.target.value)} />
        <button className="secondary" onClick={exportCsv}>
          <Download size={15} /> {t.exportCsv}
        </button>
      </div>
      <div className="log-table">
        <div className="log-row head">
          <span>{t.date}</span>
          <span>{t.filterType}</span>
          <span>{t.student}</span>
          <span>{t.detail}</span>
        </div>
        {(logPage?.events ?? []).map((event) => (
          <div className="log-row" key={event.id}>
            <span>{formatDateTime(event.createdAt)}</span>
            <span>
              <span className={`pill ${event.type === 'login' ? 'accent' : ''}`}>{event.type}</span>
            </span>
            <span>
              {event.userName ? (
                <>
                  <strong>{event.userName}</strong>
                  <small>{event.userEmail}</small>
                </>
              ) : (
                <small className="muted">—</small>
              )}
            </span>
            <span className="log-detail">
              {featureLabel(event.name, t)}
              {describe(event) !== event.name && <small>{describe(event)}</small>}
            </span>
          </div>
        ))}
        {logPage && logPage.events.length === 0 && !isFetching && <p className="muted">{t.logEmpty}</p>}
        {isFetching && !logPage && <ListSkeleton />}
        <div className="log-pager">
          <small className="muted">{t.totalRows.replace('{n}', String(total))}</small>
          <div className="pager-controls">
            <button className="secondary" disabled={page <= 1 || isFetching} onClick={() => setPage((value) => Math.max(1, value - 1))}>
              {t.prevPage}
            </button>
            <span className="pager-info">{t.pageOf.replace('{a}', String(page)).replace('{b}', String(pageCount))}</span>
            <button className="secondary" disabled={page >= pageCount || isFetching} onClick={() => setPage((value) => Math.min(pageCount, value + 1))}>
              {t.nextPage}
            </button>
          </div>
        </div>
      </div>
    </>
  )
}
