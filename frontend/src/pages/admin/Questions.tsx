import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Search } from 'lucide-react'
import { api } from '../../lib/api'
import { useSession } from '../../hooks/session'
import { Header, ListSkeleton, Modal } from '../../components/ui'
import { useUnits } from '../../components/ScopePicker'
import { StimulusPicker } from '../../components/StimulusPicker'
import { ConceptPicker } from '../../components/ConceptPicker'
import { ImageTextarea } from '../../components/ImageTextarea'
import type { Question, QuestionDraft } from '../../lib/types'

type PartEdit = { label: string; prompt: string; referenceAnswer: string; rubric: string; points: string }

// Official College Board scaffolds: AAQ = 6 parts (F is worth 2), EBQ =
// claim / two pieces of evidence / two explanations.
const AAQ_SCAFFOLD: PartEdit[] = ['A', 'B', 'C', 'D', 'E', 'F'].map((label, i) => ({
  label,
  prompt: '',
  referenceAnswer: '',
  rubric: '',
  points: i === 5 ? '2' : '1',
}))

const EBQ_SCAFFOLD: PartEdit[] = ['A', 'B', 'C', 'D', 'E'].map((label) => ({
  label,
  prompt: '',
  referenceAnswer: '',
  rubric: '',
  points: '',
}))

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
  const { data, isPending } = useQuery({
    queryKey: ['admin-questions', type, status, unitId, search],
    queryFn: async () => (await api.request<Question[] | null>(`/api/admin/questions?${params.toString()}`)) ?? [],
  })
  const questions = data ?? []
  const [actionError, setActionError] = useState('')

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin-questions'] })

  async function patchStatus(id: string, status: 'published' | 'archived') {
    setActionError('')
    try {
      await api.request(`/api/admin/questions/${id}`, { method: 'PATCH', body: JSON.stringify({ status }) })
      void refresh()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

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
      {actionError && <div className="error" style={{ marginTop: 12 }}>{actionError}</div>}
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
                    {questionLabel(t, question)} ·{' '}
                    {question.status === 'published' ? t.published : question.status === 'draft' ? t.draft : t.archived} ·{' '}
                    {(question.tags ?? []).map((tag) => tag.name).join(', ')}
                  </small>
                </span>
              </button>
              <div className="row-actions">
                {question.status !== 'published' && (
                  <button className="secondary" onClick={() => void patchStatus(question.id, 'published')}>
                    {t.publish}
                  </button>
                )}
                {question.status === 'published' && (
                  <button className="secondary" onClick={() => void patchStatus(question.id, 'archived')}>
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

export function questionLabel(
  t: { mcq: string; subjective: string; formatFrq: string; formatAaq: string; formatEbg: string; formatSet: string },
  question: Pick<Question, 'type' | 'format' | 'stimulusId'>,
) {
  if (question.type === 'mcq') {
    return question.stimulusId ? `${t.mcq} · ${t.formatSet}` : t.mcq
  }
  if (question.format === 'aaq') return t.formatAaq
  if (question.format === 'ebq') return t.formatEbg
  return `${t.subjective} · ${t.formatFrq}`
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
  const { data: tagOptions = [] } = useQuery({
    queryKey: ['admin-tags'],
    queryFn: async () => (await api.request<{ name: string; questions: number }[] | null>('/api/admin/tags')) ?? [],
  })
  const [type, setType] = useState<'mcq' | 'subjective'>(question?.type ?? 'mcq')
  const [format, setFormat] = useState<'' | 'frq' | 'aaq' | 'ebq'>(
    question?.format || (question?.type === 'subjective' ? 'frq' : ''),
  )
  const [stimulusId, setStimulusId] = useState(question?.stimulusId ?? '')
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
  const [parts, setParts] = useState<PartEdit[]>(
    question?.parts?.length
      ? question.parts.map((part) => ({
          label: part.label,
          prompt: part.prompt,
          referenceAnswer: part.referenceAnswer ?? '',
          rubric: (part.rubric ?? []).join('\n'),
          points: part.points != null ? String(part.points) : '',
        }))
      : [{ label: 'A', prompt: '', referenceAnswer: '', rubric: '', points: '' }],
  )
  const [unitRef, setUnitRef] = useState(question?.unitId ?? '')
  const [topicRef, setTopicRef] = useState(question?.topicId ?? '')
  const [conceptIds, setConceptIds] = useState<string[]>([])
  const [tags, setTags] = useState((question?.tags ?? []).map((tag) => tag.name).join('; '))
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const perPart = type === 'subjective' && (format === 'aaq' || format === 'ebq')
  const unitTopics = units.find((unit) => unit.id === unitRef)?.topics ?? []
  const stimulusRequired = perPart

  function switchType(next: 'mcq' | 'subjective') {
    setType(next)
    if (next === 'mcq') {
      setFormat('')
    } else if (!format) {
      setFormat('frq')
    }
  }

  function switchFormat(next: '' | 'frq' | 'aaq' | 'ebq') {
    setFormat(next)
    // Switching to a College Board layout re-seeds the official scaffold
    // when the parts are still the untouched single default row.
    const untouched = parts.length === 1 && !parts[0].prompt && !parts[0].referenceAnswer && !parts[0].rubric
    if (next === 'aaq' && untouched) setParts(AAQ_SCAFFOLD.map((part) => ({ ...part })))
    if (next === 'ebq' && untouched) setParts(EBQ_SCAFFOLD.map((part) => ({ ...part })))
  }

  function buildDraft(): QuestionDraft {
    const draft: QuestionDraft = {
      type,
      format: type === 'subjective' ? format : '',
      stimulusId,
      stem,
      unit: unitRef,
      topic: topicRef,
      concepts: conceptIds,
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
        points: part.points.trim() ? Number(part.points) || null : null,
      }))
    }
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
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal wide title={question ? t.editQuestion : t.newQuestion} onClose={onClose}>
      <div className="segmented inline">
        <button className={type === 'mcq' ? 'active' : ''} onClick={() => switchType('mcq')}>
          {t.mcq}
        </button>
        <button className={type === 'subjective' ? 'active' : ''} onClick={() => switchType('subjective')}>
          {t.subjective}
        </button>
      </div>
      {type === 'subjective' && (
        <div className="segmented inline">
          {(['frq', 'aaq', 'ebq'] as const).map((option) => (
            <button key={option} className={format === option ? 'active' : ''} onClick={() => switchFormat(option)}>
              {option === 'frq' ? t.formatFrq : option === 'aaq' ? t.formatAaq : t.formatEbg}
            </button>
          ))}
        </div>
      )}
      <label>
        {t.stem}
        <ImageTextarea ariaLabel={t.stem} rows={3} value={stem} onChange={setStem} />
      </label>
      <label>
        {t.materials} <small className="muted">{t.materialsHint}</small>
        <ImageTextarea ariaLabel={t.materials} rows={3} value={materials} onChange={setMaterials} />
      </label>
      <label>
        {t.stimulus}
        {stimulusRequired ? <small className="muted"> · {t.required}</small> : null}
        <StimulusPicker
          value={stimulusId}
          onChange={setStimulusId}
          defaultKind={format === 'aaq' ? 'article' : format === 'ebq' ? 'sources' : 'passage'}
        />
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
                <input
                  className="points-input"
                  type="number"
                  min={0}
                  max={10}
                  placeholder={t.points}
                  value={part.points}
                  onChange={(e) => setParts((rows) => rows.map((row, j) => (i === j ? { ...row, points: e.target.value } : row)))}
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
            onClick={() => setParts((rows) => [...rows, { label: String.fromCharCode(65 + rows.length), prompt: '', referenceAnswer: '', rubric: '', points: '' }])}
          >
            {t.addPart}
          </button>
        </>
      )}
      <div className="scope-grid">
        <label>
          {t.unit}
          <select
            value={unitRef}
            onChange={(e) => {
              setUnitRef(e.target.value)
              setTopicRef('')
            }}
          >
            <option value="">{t.all}</option>
            {units.map((unit) => (
              <option key={unit.id} value={unit.id}>
                {unit.title}
              </option>
            ))}
          </select>
        </label>
        <label>
          {t.topic}
          <select value={topicRef} onChange={(e) => setTopicRef(e.target.value)} disabled={!unitRef}>
            <option value="">{t.all}</option>
            {unitTopics.map((topic) => (
              <option key={topic.id} value={topic.id}>
                {topic.title}
              </option>
            ))}
          </select>
        </label>
      </div>
      <label>
        {t.concepts}
        <ConceptPicker initial={question?.concepts ?? []} onChange={setConceptIds} />
      </label>
      <label>
        {t.tags}
        <input
          list="question-tag-options"
          placeholder="unit-1; high-yield"
          value={tags}
          onChange={(e) => setTags(e.target.value)}
        />
        <datalist id="question-tag-options">
          {tagOptions.map((tag) => (
            <option key={tag.name} value={tag.name} />
          ))}
        </datalist>
      </label>
      {error && <div className="error">{error}</div>}
      <div className="action-row">
        <button className="secondary" onClick={onClose}>
          {t.cancel}
        </button>
        <button className="primary" disabled={busy || !stem.trim() || (stimulusRequired && !stimulusId)} onClick={save}>
          {t.save}
        </button>
      </div>
    </Modal>
  )
}
