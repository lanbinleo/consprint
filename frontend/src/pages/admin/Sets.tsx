import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, ChevronUp, Plus } from 'lucide-react'
import { api } from '../../lib/api'
import { useSession } from '../../hooks/session'
import { Header } from '../../components/ui'
import { useUnits } from '../../components/ScopePicker'
import type { Question } from '../../lib/types'

export function Sets() {
  const { t } = useSession()
  const queryClient = useQueryClient()
  const [editingId, setEditingId] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)

  const { data } = useQuery({
    queryKey: ['admin-sets'],
    queryFn: async () =>
      (await api.request<(import('../../lib/types').PracticeSet & { questionCount: number })[] | null>('/api/admin/sets')) ?? [],
  })
  const sets = data ?? []
  const [actionError, setActionError] = useState('')

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin-sets'] })

  async function patchStatus(id: string, status: 'published' | 'archived') {
    setActionError('')
    try {
      await api.request(`/api/admin/sets/${id}`, { method: 'PATCH', body: JSON.stringify({ status }) })
      void refresh()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  return (
    <div>
      <Header
        eyebrow={t.admin}
        title={t.setsTitle}
        action={
          <button className="primary" onClick={() => setCreating(true)}>
            <Plus size={16} /> {t.newSet}
          </button>
        }
      />
      {sets.length === 0 ? (
        <div className="empty-state">{t.noSetsYet}</div>
      ) : (
        <div className="table">
          {sets.map((set) => (
            <div className="concept-row" key={set.id}>
              <button className="row-main" onClick={() => setEditingId(editingId === set.id ? null : set.id)}>
                <span>
                  <strong>{set.title}</strong>
                  <small>
                    {set.mode === 'exam' ? t.examMode : t.instantMode} · {set.questionCount} {t.questions} · {set.status}
                  </small>
                </span>
              </button>
              <div className="row-actions">
                {set.status !== 'published' && (
                  <button className="secondary" onClick={() => void patchStatus(set.id, 'published')}>
                    {t.publish}
                  </button>
                )}
                {set.status === 'published' && (
                  <button className="secondary" onClick={() => void patchStatus(set.id, 'archived')}>
                    {t.unpublish}
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      {(editingId || creating) && (
        <SetEditor
          setId={creating ? null : editingId}
          onClose={() => {
            setEditingId(null)
            setCreating(false)
          }}
          onSaved={refresh}
        />
      )}
      {actionError && <div className="error" style={{ marginTop: 12 }}>{actionError}</div>}
    </div>
  )
}

type SetDetail = {
  set: { id: string; title: string; description: string; mode: 'instant' | 'exam'; timeLimitSec?: number | null; status: string }
  questions: Question[]
}

function SetEditor({ setId, onClose, onSaved }: { setId: string | null; onClose: () => void; onSaved: () => void }) {
  const { t } = useSession()
  const queryClient = useQueryClient()
  const { data: units = [] } = useUnits()
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [mode, setMode] = useState<'instant' | 'exam'>('instant')
  const [timeLimitMin, setTimeLimitMin] = useState(20)
  const [items, setItems] = useState<Question[]>([])
  const [loadedSetId, setLoadedSetId] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  // Load existing set detail once.
  const detailQuery = useQuery({
    queryKey: ['admin-set-detail', setId],
    queryFn: () => api.request<SetDetail>(`/api/admin/sets/${setId}`),
    enabled: !!setId,
  })
  if (setId && detailQuery.data && loadedSetId !== setId) {
    setLoadedSetId(setId)
    setTitle(detailQuery.data.set.title)
    setDescription(detailQuery.data.set.description)
    setMode(detailQuery.data.set.mode)
    setTimeLimitMin(Math.max(1, Math.round((detailQuery.data.set.timeLimitSec ?? 1200) / 60)))
    setItems(detailQuery.data.questions ?? [])
  }

  // Picker filters
  const [filterTag, setFilterTag] = useState('')
  const [filterUnit, setFilterUnit] = useState('')
  const [filterType, setFilterType] = useState('')
  const [picked, setPicked] = useState<Set<string>>(new Set())

  const pickerParams = new URLSearchParams({ limit: '200', status: 'published' })
  if (filterTag) pickerParams.set('tags', filterTag)
  if (filterUnit) pickerParams.set('unitId', filterUnit)
  if (filterType) pickerParams.set('type', filterType)
  const { data: candidateData } = useQuery({
    queryKey: ['admin-questions', 'picker', filterTag, filterUnit, filterType],
    queryFn: async () =>
      (await api.request<Question[] | null>(`/api/admin/questions?${pickerParams.toString()}`)) ?? [],
  })
  const candidates = candidateData ?? []
  const { data: tagData } = useQuery({
    queryKey: ['admin-tags'],
    queryFn: async () => (await api.request<{ name: string; questions: number }[] | null>('/api/admin/tags')) ?? [],
  })
  const allTags = tagData ?? []

  const itemIDs = new Set(items.map((question) => question.id))

  async function save(status: 'draft' | 'published') {
    setBusy(true)
    setError('')
    try {
      const body = {
        title,
        description,
        mode,
        timeLimitSec: mode === 'exam' ? timeLimitMin * 60 : null,
        status,
        questionIds: items.map((question) => question.id),
      }
      if (setId) {
        await api.request(`/api/admin/sets/${setId}`, { method: 'PATCH', body: JSON.stringify(body) })
      } else {
        await api.request('/api/admin/sets', { method: 'POST', body: JSON.stringify(body) })
      }
      onSaved()
      onClose()
      void queryClient.invalidateQueries({ queryKey: ['admin-sets'] })
      void queryClient.invalidateQueries({ queryKey: ['admin-set-detail'] })
      void queryClient.invalidateQueries({ queryKey: ['practice-sets'] })
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal wide" onClick={(event) => event.stopPropagation()}>
        <h3>{setId ? t.editSet : t.newSet}</h3>
        <label>
          {t.setTitle}
          <input value={title} onChange={(e) => setTitle(e.target.value)} />
        </label>
        <label>
          {t.description}
          <input value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <div className="scope-grid">
          <label>
            {t.mode}
            <div className="segmented inline">
              <button className={mode === 'instant' ? 'active' : ''} onClick={() => setMode('instant')}>
                {t.instantMode}
              </button>
              <button className={mode === 'exam' ? 'active' : ''} onClick={() => setMode('exam')}>
                {t.examMode}
              </button>
            </div>
          </label>
          {mode === 'exam' && (
            <label>
              {t.timeLimit} ({t.minutes})
              <input type="number" min={1} value={timeLimitMin} onChange={(e) => setTimeLimitMin(Number(e.target.value))} />
            </label>
          )}
        </div>
        <div className="set-items">
          <strong>
            {t.selectedQuestions} ({items.length})
          </strong>
          {items.map((question, index) => (
            <div className="set-item-row" key={question.id}>
              <span>
                {index + 1}. {question.stem.slice(0, 90)}
                {question.stem.length > 90 ? '…' : ''}
              </span>
              <div className="row-actions">
                <button
                  className="icon-line"
                  title={t.moveUp}
                  onClick={() => setItems((rows) => swap(rows, index, index - 1))}
                  disabled={index === 0}
                >
                  <ChevronUp size={15} />
                </button>
                <button
                  className="icon-line"
                  title={t.moveDown}
                  onClick={() => setItems((rows) => swap(rows, index, index + 1))}
                  disabled={index === items.length - 1}
                >
                  <ChevronDown size={15} />
                </button>
                <button className="secondary" onClick={() => setItems((rows) => rows.filter((row) => row.id !== question.id))}>
                  {t.delete}
                </button>
              </div>
            </div>
          ))}
        </div>
        <div className="set-picker">
          <strong>{t.pickQuestions}</strong>
          <div className="filter-bar">
            <input placeholder={t.tags} value={filterTag} onChange={(e) => setFilterTag(e.target.value)} list="tag-options" />
            <datalist id="tag-options">
              {allTags.map((tag) => (
                <option key={tag.name} value={tag.name} />
              ))}
            </datalist>
            <select value={filterUnit} onChange={(e) => setFilterUnit(e.target.value)}>
              <option value="">{t.allUnits}</option>
              {units.map((unit) => (
                <option key={unit.id} value={unit.id}>
                  {unit.title}
                </option>
              ))}
            </select>
            <select value={filterType} onChange={(e) => setFilterType(e.target.value)}>
              <option value="">{t.all}</option>
              <option value="mcq">{t.mcq}</option>
              <option value="subjective">{t.subjective}</option>
            </select>
          </div>
          <div className="candidate-list">
            {candidates
              .filter((question) => !itemIDs.has(question.id))
              .map((question) => (
                <label className="concept-row" key={question.id}>
                  <input
                    type="checkbox"
                    checked={picked.has(question.id)}
                    onChange={() =>
                      setPicked((current) => {
                        const next = new Set(current)
                        if (next.has(question.id)) next.delete(question.id)
                        else next.add(question.id)
                        return next
                      })
                    }
                  />
                  <span>
                    <strong>{question.stem.slice(0, 110)}</strong>
                    <small>
                      {question.type} · {(question.tags ?? []).map((tag) => tag.name).join(', ')}
                    </small>
                  </span>
                </label>
              ))}
          </div>
          <button
            className="secondary"
            disabled={picked.size === 0}
            onClick={() => {
              const additions = candidates.filter((question) => picked.has(question.id))
              setItems((rows) => [...rows, ...additions])
              setPicked(new Set())
            }}
          >
            {t.addSelected} ({picked.size})
          </button>
        </div>
        {error && <div className="error">{error}</div>}
        <div className="action-row">
          <button className="secondary" onClick={onClose}>
            {t.cancel}
          </button>
          <button className="secondary" disabled={busy || !title.trim() || items.length === 0} onClick={() => void save('draft')}>
            {t.saveDraft}
          </button>
          <button className="primary" disabled={busy || !title.trim() || items.length === 0} onClick={() => void save('published')}>
            {t.publishSet}
          </button>
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
