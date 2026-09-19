import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, ChevronUp, Plus, Upload } from 'lucide-react'
import { api } from '../../lib/api'
import { useSession } from '../../hooks/session'
import { Header, SpinnerButton } from '../../components/ui'
import type { NoteResource } from '../../lib/types'

type ItemDraft = { label: string; url: string; kind: string }

const KIND_KEYS: Record<string, 'kindEmbed' | 'kindPdf' | 'kindImage' | 'kindLink'> = {
  embed: 'kindEmbed',
  pdf: 'kindPdf',
  image: 'kindImage',
  link: 'kindLink',
}

export function NoteResources() {
  const { t } = useSession()
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState<NoteResource | null>(null)
  const [creating, setCreating] = useState(false)
  const [actionError, setActionError] = useState('')

  const { data = [] } = useQuery({
    queryKey: ['admin-note-resources'],
    queryFn: async () => (await api.request<NoteResource[] | null>('/api/admin/note-resources')) ?? [],
  })
  const tabs = data

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['admin-note-resources'] })
    void queryClient.invalidateQueries({ queryKey: ['note-resources'] })
  }

  async function patch(id: string, body: Record<string, unknown>) {
    setActionError('')
    try {
      await api.request(`/api/admin/note-resources/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
      refresh()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  async function remove(resource: NoteResource) {
    if (!window.confirm(`${t.delete}: ${resource.title}?`)) return
    setActionError('')
    try {
      await api.request(`/api/admin/note-resources/${resource.id}`, { method: 'DELETE' })
      refresh()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  async function move(index: number, delta: number) {
    const target = index + delta
    if (target < 0 || target >= tabs.length) return
    const a = tabs[index]
    const b = tabs[target]
    setActionError('')
    try {
      await api.request(`/api/admin/note-resources/${a.id}`, { method: 'PATCH', body: JSON.stringify({ position: b.position }) })
      await api.request(`/api/admin/note-resources/${b.id}`, { method: 'PATCH', body: JSON.stringify({ position: a.position }) })
      refresh()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  return (
    <div>
      <Header
        eyebrow={t.admin}
        title={t.notesResourcesTitle}
        action={
          <button className="primary" onClick={() => setCreating(true)}>
            <Plus size={16} /> {t.newTab}
          </button>
        }
      />
      {tabs.length === 0 ? (
        <div className="empty-state">{t.noTabsYet}</div>
      ) : (
        <div className="table">
          {tabs.map((resource, index) => (
            <div className="concept-row" key={resource.id}>
              <button className="row-main" onClick={() => setEditing(resource)}>
                <span>
                  <strong>{resource.title}</strong>
                  <small>
                    {resource.items.length} {t.tabItemCount} ·{' '}
                    {resource.status === 'published' ? t.published : resource.status === 'draft' ? t.draft : t.archived}
                  </small>
                </span>
              </button>
              <div className="row-actions">
                <button className="icon-line" title={t.moveUp} disabled={index === 0} onClick={() => void move(index, -1)}>
                  <ChevronUp size={15} />
                </button>
                <button
                  className="icon-line"
                  title={t.moveDown}
                  disabled={index === tabs.length - 1}
                  onClick={() => void move(index, 1)}
                >
                  <ChevronDown size={15} />
                </button>
                {resource.status !== 'published' ? (
                  <button className="secondary" onClick={() => void patch(resource.id, { status: 'published' })}>
                    {t.publish}
                  </button>
                ) : (
                  <button className="secondary" onClick={() => void patch(resource.id, { status: 'draft' })}>
                    {t.unpublish}
                  </button>
                )}
                <button className="secondary" onClick={() => void remove(resource)}>
                  {t.delete}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
      {(creating || editing) && (
        <TabEditor
          key={editing?.id ?? 'new'}
          resource={editing}
          onClose={() => {
            setEditing(null)
            setCreating(false)
          }}
          onSaved={refresh}
        />
      )}
      {actionError && (
        <div className="error" style={{ marginTop: 12 }}>
          {actionError}
        </div>
      )}
    </div>
  )
}

function TabEditor({ resource, onClose, onSaved }: { resource: NoteResource | null; onClose: () => void; onSaved: () => void }) {
  const { t } = useSession()
  const [title, setTitle] = useState(resource?.title ?? '')
  const [description, setDescription] = useState(resource?.description ?? '')
  const [items, setItems] = useState<ItemDraft[]>(
    resource?.items.map((item) => ({ label: item.label, url: item.url, kind: item.kind })) ?? [],
  )
  const [uploading, setUploading] = useState(-1)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function upload(index: number, file: File) {
    setUploading(index)
    setError('')
    try {
      const form = new FormData()
      form.append('file', file)
      const stored = await api.request<{ url: string }>('/api/admin/note-resources/upload', { method: 'POST', body: form })
      setItems((rows) => rows.map((row, i) => (i === index ? { ...row, url: stored.url } : row)))
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setUploading(-1)
    }
  }

  async function save(status: 'draft' | 'published') {
    setBusy(true)
    setError('')
    try {
      const body = { title, description, status, items }
      if (resource) {
        await api.request(`/api/admin/note-resources/${resource.id}`, { method: 'PATCH', body: JSON.stringify(body) })
      } else {
        await api.request('/api/admin/note-resources', { method: 'POST', body: JSON.stringify(body) })
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  const valid = title.trim() !== '' && items.every((item) => item.label.trim() !== '' && item.url.trim() !== '')

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal wide" onClick={(event) => event.stopPropagation()}>
        <h3>{resource ? t.editTab : t.newTab}</h3>
        <label>
          {t.setTitle}
          <input value={title} onChange={(e) => setTitle(e.target.value)} />
        </label>
        <label>
          {t.description}
          <input value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <div className="set-items">
          <strong>
            {t.tabItems} ({items.length})
          </strong>
          {items.map((item, index) => (
            <div className="note-item-edit" key={index}>
              <div className="note-item-fields">
                <input placeholder={t.itemLabel} value={item.label} onChange={(e) => setItems((rows) => rows.map((row, i) => (i === index ? { ...row, label: e.target.value } : row)))} />
                <select value={item.kind} onChange={(e) => setItems((rows) => rows.map((row, i) => (i === index ? { ...row, kind: e.target.value } : row)))}>
                  {Object.entries(KIND_KEYS).map(([kind, key]) => (
                    <option key={kind} value={kind}>
                      {t[key]}
                    </option>
                  ))}
                </select>
                <input placeholder={t.itemUrl} value={item.url} onChange={(e) => setItems((rows) => rows.map((row, i) => (i === index ? { ...row, url: e.target.value } : row)))} />
              </div>
              <div className="row-actions">
                <label className={`secondary upload-label ${uploading === index ? 'busy' : ''}`}>
                  <Upload size={15} />
                  <span>{uploading === index ? '…' : t.uploadResource}</span>
                  <input
                    type="file"
                    accept="application/pdf,image/png,image/jpeg,image/gif,image/webp"
                    hidden
                    onChange={(e) => {
                      const file = e.target.files?.[0]
                      e.target.value = ''
                      if (file) void upload(index, file)
                    }}
                  />
                </label>
                <button className="icon-line" title={t.moveUp} disabled={index === 0} onClick={() => setItems((rows) => swap(rows, index, index - 1))}>
                  <ChevronUp size={15} />
                </button>
                <button className="icon-line" title={t.moveDown} disabled={index === items.length - 1} onClick={() => setItems((rows) => swap(rows, index, index + 1))}>
                  <ChevronDown size={15} />
                </button>
                <button className="secondary" onClick={() => setItems((rows) => rows.filter((_, i) => i !== index))}>
                  {t.delete}
                </button>
              </div>
            </div>
          ))}
          <button
            className="secondary"
            onClick={() => setItems((rows) => [...rows, { label: '', url: '', kind: 'embed' }])}
          >
            <Plus size={15} /> {t.addItem}
          </button>
        </div>
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

function swap<T>(rows: T[], from: number, to: number): T[] {
  if (to < 0 || to >= rows.length) return rows
  const next = [...rows]
  const [moved] = next.splice(from, 1)
  next.splice(to, 0, moved)
  return next
}
