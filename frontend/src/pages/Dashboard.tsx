import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { CalendarDays, ChevronLeft, ChevronRight, ClipboardList, History, ListChecks, Megaphone, Pin, Sparkles, Star } from 'lucide-react'
import { api } from '../lib/api'
import { examCountdown, formatDateTime } from '../lib/format'
import { useSession } from '../hooks/session'
import type { Copy } from '../lib/i18n'
import { Header, ListSkeleton, Modal } from '../components/ui'
import { InlineMarkdown } from '../components/InlineMarkdown'
import type {
  Announcement,
  CalendarEvent,
  CalendarEventKind,
  DashboardAlerts,
  DashboardProgress,
  DashboardSummary,
  RecentConcept,
  StarredConcept,
} from '../lib/types'

const KIND_LABELS: Record<CalendarEventKind, keyof Copy> = {
  assignment: 'kindAssignment',
  assessment: 'kindAssessment',
  quiz: 'kindQuiz',
  holiday: 'kindHoliday',
  event: 'kindEvent',
}

const RESPONSE_PILL = { proficient: 'ok', fuzzy: 'warn', unknown: 'bad' } as const
const RESPONSE_LABEL = { proficient: 'proficient', fuzzy: 'fuzzy', unknown: 'unknown' } as const

function isoDate(date: Date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

function shiftMonth(month: string, delta: number) {
  const [year, mon] = month.split('-').map(Number)
  const date = new Date(year, mon - 1 + delta, 1)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`
}

function eventCovers(event: CalendarEvent, iso: string) {
  return event.date <= iso && (!event.endDate || event.endDate >= iso)
}

export function Dashboard() {
  const { t, user, lang, meta } = useSession()
  const navigate = useNavigate()
  const locale = lang === 'zh' ? 'zh-CN' : 'en-US'
  const [month, setMonth] = useState(() => isoDate(new Date()).slice(0, 7))
  const [openDay, setOpenDay] = useState<string | null>(null)

  // Hierarchical keys: other pages invalidate the shared ['dashboard'] prefix
  // after marking concepts or toggling stars.
  const summary = useQuery({ queryKey: ['dashboard', 'summary'], queryFn: () => api.request<DashboardSummary>('/api/dashboard/summary') })
  const progress = useQuery({ queryKey: ['dashboard', 'progress'], queryFn: () => api.request<DashboardProgress>('/api/dashboard/progress') })
  const alerts = useQuery({ queryKey: ['dashboard', 'alerts'], queryFn: () => api.request<DashboardAlerts>('/api/dashboard/alerts') })
  const announcements = useQuery({
    queryKey: ['dashboard', 'announcements'],
    queryFn: async () => (await api.request<Announcement[] | null>('/api/announcements')) ?? [],
  })
  const starred = useQuery({
    queryKey: ['dashboard', 'starred'],
    queryFn: async () => (await api.request<StarredConcept[] | null>('/api/dashboard/starred')) ?? [],
  })
  const recent = useQuery({
    queryKey: ['dashboard', 'recent'],
    queryFn: async () => (await api.request<RecentConcept[] | null>('/api/dashboard/recent')) ?? [],
  })
  const calendar = useQuery({
    queryKey: ['dashboard', 'calendar', month],
    queryFn: async () => (await api.request<CalendarEvent[] | null>(`/api/calendar?month=${month}`)) ?? [],
  })

  const now = new Date()
  const hour = now.getHours()
  const greeting = hour < 12 ? t.goodMorning : hour < 18 ? t.goodAfternoon : t.goodEvening
  const todayLine = now.toLocaleDateString(locale, { weekday: 'long', month: 'long', day: 'numeric' })
  const quote = summary.data?.quote ?? null
  const countdown = examCountdown(meta?.examDate || undefined, Date.now())
  // Purely visual fill for the slim countdown track: progress across a
  // one-year window ending on exam day.
  const examFill = countdown ? Math.min(100, Math.max(2, Math.round(((365 - countdown.days) / 365) * 100))) : 0
  const examDateLine = meta?.examDate
    ? new Date(meta.examDate).toLocaleDateString(locale, { year: 'numeric', month: 'long', day: 'numeric' })
    : ''
  const openDayEvents = openDay ? (calendar.data ?? []).filter((event) => eventCovers(event, openDay)) : []
  const weakConcepts = alerts.data?.weakConcepts ?? []

  return (
    <section className="page">
      <Header
        eyebrow={t.dashboard}
        title={t.overview}
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
      <div className="boards-hero">
        <aside className="greet-card">
          <div className="greet-head">
            <small className="greet-date">{todayLine}</small>
            <strong>{user ? `${greeting}${lang === 'zh' ? '，' : ', '}${user.name}` : greeting}</strong>
          </div>
          <div className="greet-quote">
            <p className="daily-quote">{quote ? (lang === 'zh' ? quote.textZh : quote.textEn) : t.loading}</p>
            {quote?.source && <small className="quote-source">— {quote.source}</small>}
          </div>
          <div className="greet-stats">
            <button onClick={() => navigate('/flashcards')}>
              <strong>{progress.data?.todayReviews ?? '–'}</strong>
              <span>{t.todayReviews}</span>
            </button>
            <button onClick={() => navigate('/flashcards')}>
              <strong>{progress.data?.streakDays ?? '–'}</strong>
              <span>{t.streak}</span>
            </button>
            <button onClick={() => navigate('/flashcards')}>
              <strong>{progress.data?.shortTermReviews ?? '–'}</strong>
              <span>{t.shortTerm}</span>
            </button>
          </div>
        </aside>
        <div className="board-card announcements-card">
          <h3>
            <Megaphone size={17} /> {t.announcements}
          </h3>
          {announcements.isPending ? (
            <ListSkeleton rows={3} />
          ) : (announcements.data ?? []).length === 0 ? (
            <div className="board-empty">
              <p>{t.announcementsEmpty}</p>
            </div>
          ) : (
            <AnnouncementCarousel items={announcements.data ?? []} locale={locale} />
          )}
        </div>
      </div>
      {countdown && (
        <div className="exam-band">
          <span className="exam-label">{t.countdown}</span>
          <div className="exam-track" aria-hidden="true">
            <i style={{ width: `${examFill}%` }} />
          </div>
          <strong>
            {countdown.days} {t.daysUnit}
          </strong>
          <small>{examDateLine}</small>
        </div>
      )}
      <div className="boards-row">
        <div className="board-card calendar-card">
          <h3>
            <CalendarDays size={17} />
            <span>{monthTitle(month, locale)}</span>
            <span className="cal-nav">
              <button className="icon-btn" onClick={() => setMonth(shiftMonth(month, -1))} aria-label={t.previous}>
                <ChevronLeft size={15} />
              </button>
              <button className="icon-btn" onClick={() => setMonth(shiftMonth(month, 1))} aria-label={t.next}>
                <ChevronRight size={15} />
              </button>
            </span>
          </h3>
          <MiniMonth month={month} events={calendar.data ?? []} onOpenDay={setOpenDay} />
        </div>
        <div className="board-card starred-card">
          <h3>
            <Star size={17} /> {t.starredTitle}
          </h3>
          {starred.isPending ? (
            <ListSkeleton rows={3} />
          ) : (starred.data ?? []).length === 0 ? (
            <div className="board-empty">
              <p>{t.starredEmpty}</p>
            </div>
          ) : (
            <div className="starred-list">
              {(starred.data ?? []).map((row) => (
                <button key={row.conceptId} onClick={() => navigate(`/terms?concept=${row.conceptId}`)}>
                  <strong>{row.term}</strong>
                  <small>
                    {row.unitTitle} · {row.topicTitle}
                  </small>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
      <div className="boards-row boards-row-bottom">
        <div className="board-card recent-card">
          <h3>
            <History size={17} /> {t.recentReviews}
          </h3>
          {recent.isPending ? (
            <ListSkeleton rows={3} />
          ) : (recent.data ?? []).length === 0 ? (
            <div className="board-empty">
              <p>{t.recentEmpty}</p>
            </div>
          ) : (
            <div className="starred-list">
              {(recent.data ?? []).map((row) => (
                <button key={row.conceptId} onClick={() => navigate(`/terms?concept=${row.conceptId}`)}>
                  <span className="row-line">
                    <strong>{row.term}</strong>
                    <em className={`pill ${RESPONSE_PILL[row.response]}`}>{t[RESPONSE_LABEL[row.response]]}</em>
                  </span>
                  <small>
                    {row.unitTitle} · {row.topicTitle} · {formatDateTime(row.lastAt)}
                  </small>
                </button>
              ))}
            </div>
          )}
        </div>
        <div className="board-card tasks-card">
          <h3>
            <ListChecks size={17} /> {t.reviewTasks}
          </h3>
          {alerts.isPending ? (
            <ListSkeleton rows={3} />
          ) : weakConcepts.length === 0 ? (
            <div className="board-empty">
              <p>{t.tasksEmpty}</p>
            </div>
          ) : (
            <div className="starred-list">
              {weakConcepts.map((row) => (
                <button key={row.conceptId} onClick={() => navigate(`/terms?concept=${row.conceptId}`)}>
                  <span className="row-line">
                    <strong>{row.term}</strong>
                    <em className={`pill ${row.status === 'unknown' ? 'bad' : 'warn'}`}>
                      {row.status === 'unknown' ? t.unknown : t.fuzzy}
                    </em>
                  </span>
                  <small>
                    {row.unitTitle} · {row.topicTitle}
                  </small>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
      {openDay && (
        <Modal
          title={new Date(`${openDay}T00:00`).toLocaleDateString(locale, { month: 'long', day: 'numeric', weekday: 'long' })}
          onClose={() => setOpenDay(null)}
        >
          {openDayEvents.length === 0 ? (
            <p className="muted">{t.calendarEmpty}</p>
          ) : (
            <div className="day-events">
              {openDayEvents.map((event) => (
                <div key={event.id} className={`day-event ev-${event.kind}`}>
                  <span className="ev-kind">{t[KIND_LABELS[event.kind]]}</span>
                  <strong>{event.title}</strong>
                  <small>
                    {event.time ?? t.allDay}
                    {event.endDate ? ` · ${event.date} → ${event.endDate}` : ''}
                  </small>
                  {event.note && <p>{event.note}</p>}
                </div>
              ))}
            </div>
          )}
        </Modal>
      )}
    </section>
  )
}

function monthTitle(month: string, locale: string) {
  const [year, mon] = month.split('-').map(Number)
  return new Date(year, mon - 1, 1).toLocaleDateString(locale, { year: 'numeric', month: 'long' })
}

// Announcement carousel: one announcement at a time, auto-advances every 6s
// with a horizontal swipe, pauses while hovered, and the dots jump manually.
function AnnouncementCarousel({ items, locale }: { items: Announcement[]; locale: string }) {
  const { t } = useSession()
  const [index, setIndex] = useState(0)
  const [paused, setPaused] = useState(false)
  const count = items.length

  useEffect(() => {
    if (index >= count) setIndex(0)
  }, [index, count])

  useEffect(() => {
    if (paused || count < 2) return
    const id = setInterval(() => setIndex((i) => (i + 1) % count), 6000)
    return () => clearInterval(id)
  }, [paused, count])

  return (
    <>
      <div className="ann-carousel" onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)}>
        <div className="ann-track" style={{ transform: `translateX(-${index * 100}%)` }}>
          {items.map((announcement) => (
            <article key={announcement.id} className={`announcement ${announcement.pinned ? 'pinned' : ''}`}>
              <header>
                <h4>{announcement.title}</h4>
                <small>
                  {announcement.pinned && (
                    <span className="pin-flag">
                      <Pin size={11} /> {t.pinned}
                    </span>
                  )}
                  {announcement.authorName ? `${announcement.authorName} · ` : ''}
                  {new Date(announcement.createdAt).toLocaleDateString(locale, { month: 'short', day: 'numeric' })}
                </small>
              </header>
              {announcement.body
                .split('\n')
                .filter((line) => line.trim() !== '')
                .map((line, index) => (
                  <p key={index}>
                    <InlineMarkdown text={line} />
                  </p>
                ))}
            </article>
          ))}
        </div>
      </div>
      {count > 1 && (
        <div className="ann-dots" role="tablist">
          {items.map((announcement, i) => (
            <button
              key={announcement.id}
              className={i === index ? 'on' : ''}
              onClick={() => setIndex(i)}
              aria-label={announcement.title}
              aria-current={i === index}
            />
          ))}
        </div>
      )}
    </>
  )
}

// Mini month grid: leading/trailing blanks pad to whole weeks (Sunday first),
// each day shows up to three kind-colored dots, and days with events open the
// detail modal. Hover reveals a small tooltip listing the day's titles.
function MiniMonth({
  month,
  events,
  onOpenDay,
}: {
  month: string
  events: CalendarEvent[]
  onOpenDay: (iso: string) => void
}) {
  const { t, lang } = useSession()
  const locale = lang === 'zh' ? 'zh-CN' : 'en-US'
  const [year, mon] = month.split('-').map(Number)
  const lead = new Date(year, mon - 1, 1).getDay()
  const daysInMonth = new Date(year, mon, 0).getDate()
  const cells: (string | null)[] = Array.from({ length: lead }, () => null)
  for (let day = 1; day <= daysInMonth; day++) cells.push(`${month}-${String(day).padStart(2, '0')}`)
  while (cells.length % 7 !== 0) cells.push(null)
  // 2023-10-01 is a Sunday; walking that week yields Sun→Sat weekday labels.
  const weekdays = Array.from({ length: 7 }, (_, i) => new Date(2023, 9, 1 + i).toLocaleDateString(locale, { weekday: 'narrow' }))
  const today = isoDate(new Date())

  return (
    <div className="mini-cal">
      <div className="cal-week">
        {weekdays.map((label, index) => (
          <span key={index}>{label}</span>
        ))}
      </div>
      <div className="cal-grid">
        {cells.map((iso, index) => {
          if (!iso) return <span key={`blank-${index}`} className="cal-day blank" />
          const dayEvents = events.filter((event) => eventCovers(event, iso))
          const dots = [...new Set(dayEvents.map((event) => event.kind))].slice(0, 3)
          return (
            <button
              key={iso}
              className={`cal-day ${iso === today ? 'today' : ''} ${dayEvents.length > 0 ? 'has-events' : ''}`}
              onClick={() => dayEvents.length > 0 && onOpenDay(iso)}
            >
              <span>{Number(iso.slice(8))}</span>
              {dots.length > 0 && (
                <i className="cal-dots" aria-hidden="true">
                  {dots.map((kind) => (
                    <em key={kind} className={`ev-${kind}`} />
                  ))}
                </i>
              )}
              {dayEvents.length > 0 && (
                <span className="cal-tip">
                  {dayEvents.map((event) => (
                    <span key={event.id}>{event.title}</span>
                  ))}
                </span>
              )}
            </button>
          )
        })}
      </div>
      <small className="cal-legend">
        {(Object.keys(KIND_LABELS) as CalendarEventKind[]).map((kind) => (
          <span key={kind}>
            <em className={`ev-${kind}`} /> {t[KIND_LABELS[kind]]}
          </span>
        ))}
      </small>
    </div>
  )
}
