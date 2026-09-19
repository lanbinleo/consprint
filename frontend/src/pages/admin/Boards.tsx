import { useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Bold, CalendarPlus, Megaphone, Pin, Plus, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { useSession } from '../../hooks/session'
import { Header, SpinnerButton } from '../../components/ui'
import { InlineMarkdown } from '../../components/InlineMarkdown'
import { MARKER_COLORS } from '../../lib/inlineMarkdown'
import type { Announcement, CalendarEvent, CalendarEventKind } from '../../lib/types'

const KIND_KEYS: Record<CalendarEventKind, string> = {
  assignment: 'kindAssignment',
  assessment: 'kindAssessment',
  quiz: 'kindQuiz',
  holiday: 'kindHoliday',
  event: 'kindEvent',
}

export function Boards() {
  const { t } = useSession()
  const queryClient = useQueryClient()
  const [editingAnnouncement, setEditingAnnouncement] = useState<Announcement | null>(null)
  const [creatingAnnouncement, setCreatingAnnouncement] = useState(false)
  const [editingEvent, setEditingEvent] = useState<CalendarEvent | null>(null)
  const [creatingEvent, setCreatingEvent] = useState(false)
  const [actionError, setActionError] = useState('')

  const announcements = useQuery({
    queryKey: ['admin-announcements'],
    queryFn: async () => (await api.request<Announcement[] | null>('/api/admin/announcements')) ?? [],
  })
  const events = useQuery({
    queryKey: ['admin-calendar'],
    queryFn: async () => (await api.request<CalendarEvent[] | null>('/api/admin/calendar')) ?? [],
  })

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['admin-announcements'] })
    void queryClient.invalidateQueries({ queryKey: ['admin-calendar'] })
    void queryClient.invalidateQueries({ queryKey: ['dashboard', 'announcements'] })
    void queryClient.invalidateQueries({ queryKey: ['dashboard', 'calendar'] })
  }

  async function patchAnnouncement(id: string, body: Record<string, unknown>) {
    setActionError('')
    try {
      await api.request(`/api/admin/announcements/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
      refresh()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  async function removeAnnouncement(announcement: Announcement) {
    if (!window.confirm(`${t.delete}: ${announcement.title}?`)) return
    setActionError('')
    try {
      await api.request(`/api/admin/announcements/${announcement.id}`, { method: 'DELETE' })
      refresh()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  async function removeEvent(event: CalendarEvent) {
    if (!window.confirm(`${t.delete}: ${event.title}?`)) return
    setActionError('')
    try {
      await api.request(`/api/admin/calendar/${event.id}`, { method: 'DELETE' })
      refresh()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  return (
    <div>
      <Header eyebrow={t.admin} title={t.boardsTitle} />
      {actionError && <div className="error">{actionError}</div>}

      <section className="boards-admin-section">
        <h3 className="section-title">
          <Megaphone size={16} /> {t.announcementsAdmin}
          <button className="primary" onClick={() => setCreatingAnnouncement(true)}>
            <Plus size={15} /> {t.newAnnouncement}
          </button>
        </h3>
        {(announcements.data ?? []).length === 0 ? (
          <div className="empty-state">{t.empty}</div>
        ) : (
          <div className="table">
            {(announcements.data ?? []).map((announcement) => (
              <div className="concept-row" key={announcement.id}>
                <button className="row-main" onClick={() => setEditingAnnouncement(announcement)}>
                  <span>
                    <strong>
                      {announcement.pinned && <Pin size={12} className="pin-inline" />} {announcement.title}
                    </strong>
                    <small>
                      {announcement.status === 'published' ? t.published : announcement.status === 'draft' ? t.draft : t.archived}
                      {announcement.authorName ? ` · ${announcement.authorName}` : ''}
                    </small>
                  </span>
                </button>
                <div className="row-actions">
                  <button className="secondary" onClick={() => void patchAnnouncement(announcement.id, { pinned: !announcement.pinned })}>
                    {announcement.pinned ? <Pin size={14} className="pin-on" /> : <Pin size={14} />} {t.pinToggle}
                  </button>
                  {announcement.status !== 'published' ? (
                    <button className="secondary" onClick={() => void patchAnnouncement(announcement.id, { status: 'published' })}>
                      {t.publish}
                    </button>
                  ) : (
                    <button className="secondary" onClick={() => void patchAnnouncement(announcement.id, { status: 'draft' })}>
                      {t.unpublish}
                    </button>
                  )}
                  <button className="secondary" onClick={() => void removeAnnouncement(announcement)}>
                    <Trash2 size={14} /> {t.delete}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      <section className="boards-admin-section">
        <h3 className="section-title">
          <CalendarPlus size={16} /> {t.calendarAdmin}
          <button className="primary" onClick={() => setCreatingEvent(true)}>
            <Plus size={15} /> {t.newEvent}
          </button>
        </h3>
        {(events.data ?? []).length === 0 ? (
          <div className="empty-state">{t.empty}</div>
        ) : (
          <div className="table">
            {(events.data ?? []).map((event) => (
              <div className="concept-row" key={event.id}>
                <button className="row-main" onClick={() => setEditingEvent(event)}>
                  <span>
                    <strong>
                      <em className={`ev-dot ev-${event.kind}`} /> {event.title}
                    </strong>
                    <small>
                      {event.date}
                      {event.endDate ? ` → ${event.endDate}` : ''} · {event.time ?? t.allDay} ·{' '}
                      {t[KIND_KEYS[event.kind] as keyof typeof t]}
                    </small>
                  </span>
                </button>
                <div className="row-actions">
                  <button className="secondary" onClick={() => void removeEvent(event)}>
                    <Trash2 size={14} /> {t.delete}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      {(creatingAnnouncement || editingAnnouncement) && (
        <AnnouncementEditor
          key={editingAnnouncement?.id ?? 'new'}
          announcement={editingAnnouncement}
          onClose={() => {
            setEditingAnnouncement(null)
            setCreatingAnnouncement(false)
          }}
          onSaved={refresh}
        />
      )}
      {(creatingEvent || editingEvent) && (
        <EventEditor
          key={editingEvent?.id ?? 'new'}
          event={editingEvent}
          onClose={() => {
            setEditingEvent(null)
            setCreatingEvent(false)
          }}
          onSaved={refresh}
        />
      )}
    </div>
  )
}

function AnnouncementEditor({
  announcement,
  onClose,
  onSaved,
}: {
  announcement: Announcement | null
  onClose: () => void
  onSaved: () => void
}) {
  const { t } = useSession()
  const [title, setTitle] = useState(announcement?.title ?? '')
  const [body, setBody] = useState(announcement?.body ?? '')
  const [pinned, setPinned] = useState(announcement?.pinned ?? false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const bodyRef = useRef<HTMLTextAreaElement>(null)

  // Wrap the current textarea selection with marker syntax; runs after the
  // state update so the selection lands inside the inserted markers.
  function wrap(before: string, after: string) {
    const el = bodyRef.current
    if (!el) return
    const start = el.selectionStart
    const end = el.selectionEnd
    const selected = el.value.slice(start, end)
    setBody(el.value.slice(0, start) + before + selected + after + el.value.slice(end))
    requestAnimationFrame(() => {
      el.focus()
      el.setSelectionRange(start + before.length, start + before.length + selected.length)
    })
  }

  async function save(status: 'draft' | 'published') {
    setBusy(true)
    setError('')
    try {
      const payload = { title, body, pinned, status }
      if (announcement) {
        await api.request(`/api/admin/announcements/${announcement.id}`, { method: 'PATCH', body: JSON.stringify(payload) })
      } else {
        await api.request('/api/admin/announcements', { method: 'POST', body: JSON.stringify(payload) })
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  const valid = title.trim() !== ''

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal wide" onClick={(event) => event.stopPropagation()}>
        <h3>{announcement ? t.editAnnouncement : t.newAnnouncement}</h3>
        <label>
          {t.announcementTitleField}
          <input value={title} onChange={(e) => setTitle(e.target.value)} />
        </label>
        <label>
          {t.announcementBody}
          <div className="marker-toolbar">
            <button
              type="button"
              className="icon-btn"
              title={t.toolbarBold}
              onClick={() => wrap('**', '**')}
            >
              <Bold size={14} />
            </button>
            {MARKER_COLORS.map((color) => (
              <button
                type="button"
                key={color}
                className={`swatch mk-${color}`}
                title={`${t.toolbarHighlight}: ${color}`}
                onClick={() => wrap(color === 'lemon' ? '==' : `==${color}|`, '==')}
              />
            ))}
          </div>
          <textarea ref={bodyRef} rows={7} value={body} onChange={(e) => setBody(e.target.value)} placeholder={t.announcementBodyHint} />
        </label>
        {body.trim() !== '' && (
          <div className="announcement-preview">
            <strong>{title || '…'}</strong>
            {body
              .split('\n')
              .filter((line) => line.trim() !== '')
              .map((line, index) => (
                <p key={index}>
                  <InlineMarkdown text={line} />
                </p>
              ))}
          </div>
        )}
        <label className="check-row">
          <input type="checkbox" checked={pinned} onChange={(e) => setPinned(e.target.checked)} />
          {t.pinToggle}
        </label>
        {error && <div className="error">{error}</div>}
        <div className="action-row">
          <button className="secondary" onClick={onClose}>
            {t.cancel}
          </button>
          <SpinnerButton className="secondary" busy={busy} disabled={!valid} onClick={() => void save('draft')}>
            {t.saveDraft}
          </SpinnerButton>
          <SpinnerButton busy={busy} disabled={!valid} onClick={() => void save('published')}>
            {t.publish}
          </SpinnerButton>
        </div>
      </div>
    </div>
  )
}

function EventEditor({ event, onClose, onSaved }: { event: CalendarEvent | null; onClose: () => void; onSaved: () => void }) {
  const { t } = useSession()
  const [title, setTitle] = useState(event?.title ?? '')
  const [date, setDate] = useState(event?.date ?? '')
  const [endDate, setEndDate] = useState(event?.endDate ?? '')
  const [time, setTime] = useState(event?.time ?? '')
  const [kind, setKind] = useState<CalendarEventKind>(event?.kind ?? 'assignment')
  const [note, setNote] = useState(event?.note ?? '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function save() {
    setBusy(true)
    setError('')
    try {
      const payload = { title, date, endDate: endDate || undefined, time: time || undefined, kind, note }
      if (event) {
        await api.request(`/api/admin/calendar/${event.id}`, { method: 'PATCH', body: JSON.stringify(payload) })
      } else {
        await api.request('/api/admin/calendar', { method: 'POST', body: JSON.stringify(payload) })
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  const valid = title.trim() !== '' && date !== ''

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(event) => event.stopPropagation()}>
        <h3>{event ? t.editEvent : t.newEvent}</h3>
        <label>
          {t.announcementTitleField}
          <input value={title} onChange={(e) => setTitle(e.target.value)} />
        </label>
        <div className="event-grid">
          <label>
            {t.eventDate}
            <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
          </label>
          <label>
            {t.eventEndDate}
            <input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
          </label>
          <label>
            {t.eventTime}
            <input type="time" value={time} onChange={(e) => setTime(e.target.value)} />
          </label>
          <label>
            {t.eventKind}
            <select value={kind} onChange={(e) => setKind(e.target.value as CalendarEventKind)}>
              {(Object.keys(KIND_KEYS) as CalendarEventKind[]).map((value) => (
                <option key={value} value={value}>
                  {t[KIND_KEYS[value] as keyof typeof t]}
                </option>
              ))}
            </select>
          </label>
        </div>
        <label>
          {t.eventNote}
          <textarea rows={3} value={note} onChange={(e) => setNote(e.target.value)} />
        </label>
        {error && <div className="error">{error}</div>}
        <div className="action-row">
          <button className="secondary" onClick={onClose}>
            {t.cancel}
          </button>
          <SpinnerButton busy={busy} disabled={!valid} onClick={() => void save()}>
            {t.save}
          </SpinnerButton>
        </div>
      </div>
    </div>
  )
}
