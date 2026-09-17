import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Search } from 'lucide-react'
import { api } from '../../lib/api'
import { useSession } from '../../hooks/session'
import { Header, ListSkeleton } from '../../components/ui'
import { useUnits } from '../../components/ScopePicker'
import type { Question, QuestionDraft } from '../../lib/types'

export function Questions() {
  const { t } = useSession()
  const queryClient = useQueryClient()
  const { data: units = [] } = useUnits()
  const [search, setSearch] = useState('')
  const [type, setType] = useState('')
  const [status, setStatus] = useState('')
  const [unitId, setUnitId] = useState('')
  const [editing, setEditing] = useState<Question | 'new' | null>(null)

  const params = new URLSearchParams()
  if (type) params.set('type', type)
  if (status) params.set('status', status)
  if (unitId) params.set('unitId', unitId)
  if (search) params.set('search', search)
  const { data: questions = [], isPending } = useQuery({
    queryKey: ['admin-questions', type, status, unitId, search],
    queryFn: () => api.request<Question[]>(`/api/admin/questions?${params.toString()}`),
  })

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin-questions'] })

  return (
    <div>
      <Header
        eyebrow={t.admin}
        title={t.adminQuestions}
        action={
          <button className="primary" onClick={() => setEditing('new')}>
            <Plus size={16} /> {t.newQuestion}
          </button>
        }
      />
      <div className="filter-bar">
        <label className="search">
          <Search size={16} />
          <input placeholder={t.search} value={search} onChange={(e) => setSearch(e.target.value)} />
        </label>
        <select value={type} onChange={(e) => setType(e.target.value)}>
          <option value="">{t.type}: {t.all}</option>
          <option value="mcq">{t.mcq}</option>
          <option value="subjective">{t.subjective}</option>
        </select>
        <select value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">{t.status}: {t.all}</option>
          <option value="draft">{t.draft}</option>
          <option value="published">{t.published}</option>
        </select>
        <select value={unitId} onChange={(e) => setUnitId(e.target.value)}>
          <option value="">{t.allUnits}</option>
          {units.map((unit) => (
            <option key={unit.id} value={unit.id}>
              {unit.title}
            </option>
          ))}
        </select>
      </div>
      {isPending ? (
        <ListSkeleton />
      ) : questions.length === 0 ? (
        <div className="empty-state">{t.noQuestions}</div>
      ) : (
        <div className="table">
          {questions.map((question) => (
            <div className="concept-row" key={question.id}>
              <button className="row-main" onClick={() => setEditing(question)}>
                <span>
                  <strong>{question.stem.slice(0, 140)}</strong>
                  <small>
                    {question.type === 'mcq' ? t.mcq : t.subjective} · {question.status} ·{' '}
                    {question.tags.map((tag) => tag.name).join(', ')}
                  </small>
                </span>
              </button>
              <div className="row-actions">
                {question.status !== 'published' && (
                  <button
                    className="secondary"
                    onClick={async () => {
                      await api.request(`/api/admin/questions/${question.id}`, {
                        method: 'PATCH',
                        body: JSON.stringify({ status: 'published' }),
                      })
                      void refresh()
                    }}
                  >
                    {t.publish}
                  </button>
                )}
                {question.status === 'published' && (
                  <button
                    className="secondary"
                    onClick={async () => {
                      await api.request(`/api/admin/questions/${question.id}`, {
                        method: 'PATCH',
                        body: JSON.stringify({ status: 'archived' }),
                      })
                      void refresh()
                    }}
                  >
                    {t.unpublish}
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      {editing && <QuestionEditor question={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSaved={refresh} />}
    </div>
  )
}

function QuestionEditor({
  question,
  onClose,
  onSaved,
}: {
  question: Question | null
  onClose: () => void
  onSaved: () => void
}) {
  const { t } = useSession()
  const { data: units = [] } = useUnits()
  const [type, setType] = useState<'mcq' | 'subjective'>(question?.type ?? 'mcq')
  const [stem, setStem] = useState(question?.stem ?? '')
  const [choices, setChoices] = useState(
    question?.choices?.length ? question.choices : [
      { key: 'A', text: '' },
      { key: 'B', text: '' },
    ],
  )
  const [answerKey, setAnswerKey] = useState(question?.answerKey ?? 'A')
  const [explanation, setExplanation] = useState(question?.explanation ?? '')
  const [materials, setMaterials] = useState(question?.materials?.map((m) => `${m.title} :: ${m.text}`).join('\n') ?? '')
  const [parts, setParts] = useState(
    question?.parts?.length
      ? question.parts.map((part) => ({ label: part.label, prompt: part.prompt, referenceAnswer: part.referenceAnswer ?? '', rubric: (part.rubric ?? []).join('\n') }))
      : [{ label: 'A', prompt: '', referenceAnswer: '', rubric: '' }],
  )
  const [unitRef, setUnitRef] = useState(question?.unitId ?? '')
  const [tags, setTags] = useState(question?.tags.map((tag) => tag.name).join('; ') ?? '')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  function buildDraft(): QuestionDraft {
    const draft: QuestionDraft = {
      type,
      stem,
      tags: tags.split(/[;,]/).map((tag) => tag.trim()).filter(Boolean),
    }
    if (materials.trim()) {
      draft.materials = materials
        .split(/\n+/)
        .map((line) => line.trim())
        .filter(Boolean)
        .map((line) => {
          const [title, text] = line.split('::')
          return title && text ? { title: title.trim(), text: text.trim() } : { title: '', text: line }
        })
    }
    if (type === 'mcq') {
      draft.choices = choices.filter((choice) => choice.text.trim())
      draft.answerKey = answerKey
      draft.explanation = explanation
    } else {
      draft.parts = parts.map((part) => ({
        label: part.label,
        prompt: part.prompt || stem,
        referenceAnswer: part.referenceAnswer,
        rubric: part.rubric.split('\n').map((point) => point.trim()).filter(Boolean),
      }))
    }
    if (unitRef) draft.unit = unitRef
    return draft
  }

  async function save() {
    setBusy(true)
    setError('')
    try {
      if (question) {
        await api.request(`/api/admin/questions/${question.id}`, { method: 'PATCH', body: JSON.stringify(buildDraft()) })
      } else {
        await api.request('/api/admin/questions', { method: 'POST', body: JSON.stringify(buildDraft()) })
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal wide" onClick={(event) => event.stopPropagation()}>
        <h3>{question ? t.editQuestion : t.newQuestion}</h3>
        <div className="segmented inline">
          <button className={type === 'mcq' ? 'active' : ''} onClick={() => setType('mcq')}>
            {t.mcq}
          </button>
          <button className={type === 'subjective' ? 'active' : ''} onClick={() => setType('subjective')}>
            {t.subjective}
          </button>
        </div>
        <label>
          {t.stem}
          <textarea rows={3} value={stem} onChange={(e) => setStem(e.target.value)} />
        </label>
        <label>
          {t.materials} <small className="muted">Title :: text（one per line）</small>
          <textarea rows={3} value={materials} onChange={(e) => setMaterials(e.target.value)} />
        </label>
        {type === 'mcq' ? (
          <>
            {choices.map((choice, i) => (
              <div className="choice-edit" key={i}>
                <input
                  className="choice-key-input"
                  value={choice.key}
                  onChange={(e) => setChoices((rows) => rows.map((row, j) => (i === j ? { ...row, key: e.target.value.toUpperCase() } : row)))}
                />
                <input
                  placeholder={`${t.choices} ${choice.key}`}
                  value={choice.text}
                  onChange={(e) => setChoices((rows) => rows.map((row, j) => (i === j ? { ...row, text: e.target.value } : row)))}
                />
              </div>
            ))}
            <button className="secondary" onClick={() => setChoices((rows) => [...rows, { key: String.fromCharCode(65 + rows.length), text: '' }])}>
              {t.addChoice}
            </button>
            <label>
              {t.answerKey}
              <select value={answerKey} onChange={(e) => setAnswerKey(e.target.value)}>
                {choices.map((choice) => (
                  <option key={choice.key} value={choice.key}>
                    {choice.key}
                  </option>
                ))}
              </select>
            </label>
            <label>
              {t.explanation}
              <textarea rows={2} value={explanation} onChange={(e) => setExplanation(e.target.value)} />
            </label>
          </>
        ) : (
          <>
            {parts.map((part, i) => (
              <div className="part-edit" key={i}>
                <div className="part-head">
                  <input
                    className="choice-key-input"
                    value={part.label}
                    onChange={(e) => setParts((rows) => rows.map((row, j) => (i === j ? { ...row, label: e.target.value } : row)))}
                  />
                  <input
                    placeholder={t.partPrompt}
                    value={part.prompt}
                    onChange={(e) => setParts((rows) => rows.map((row, j) => (i === j ? { ...row, prompt: e.target.value } : row)))}
                  />
                </div>
                <textarea
                  placeholder={t.reference}
                  rows={2}
                  value={part.referenceAnswer}
                  onChange={(e) => setParts((rows) => rows.map((row, j) => (i === j ? { ...row, referenceAnswer: e.target.value } : row)))}
                />
                <textarea
                  placeholder={t.rubricPoints}
                  rows={2}
                  value={part.rubric}
                  onChange={(e) => setParts((rows) => rows.map((row, j) => (i === j ? { ...row, rubric: e.target.value } : row)))}
                />
              </div>
            ))}
            <button
              className="secondary"
              onClick={() => setParts((rows) => [...rows, { label: String.fromCharCode(65 + rows.length), prompt: '', referenceAnswer: '', rubric: '' }])}
            >
              {t.addPart}
            </button>
          </>
        )}
        <div className="scope-grid">
          <label>
            {t.unit}
            <select value={unitRef} onChange={(e) => setUnitRef(e.target.value)}>
              <option value="">{t.all}</option>
              {units.map((unit) => (
                <option key={unit.id} value={unit.id}>
                  {unit.title}
                </option>
              ))}
            </select>
          </label>
          <label>
            {t.tags}
            <input placeholder="unit-1; high-yield" value={tags} onChange={(e) => setTags(e.target.value)} />
          </label>
        </div>
        {error && <div className="error">{error}</div>}
        <div className="action-row">
          <button className="secondary" onClick={onClose}>
            {t.cancel}
          </button>
          <button className="primary" disabled={busy || !stem.trim()} onClick={save}>
            {t.save}
          </button>
        </div>
      </div>
    </div>
  )
}
