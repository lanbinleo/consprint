import { useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, ArrowLeft, Eye, Layers, Plus, Search, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { useSession } from '../../hooks/session'
import { Modal } from '../../components/ui'
import { useUnits } from '../../components/ScopePicker'
import { InlineMarkdown, MarkdownText } from '../../components/InlineMarkdown'
import { kindLabel } from '../../components/StimulusPicker'
import { questionLabel } from './Questions'
import { clusterRanges } from '../../lib/questionGrouping'
import type { PracticeSet, Question, Stimulus } from '../../lib/types'

// Full-page assembly editor: metadata + coverage on the left, the ordered
// paper (grouped into stimulus clusters) in the middle, the question bank
// browser on the right.

type SetDetail = {
  set: PracticeSet
  items: { id: string; questionId: string; position: number }[]
  questions: Question[]
  stimuli?: Stimulus[]
}

export function SetEditor() {
  const routeSetId = useParams<{ setId: string }>().setId
  const isNew = routeSetId === 'new'
  const navigate = useNavigate()
  const { t } = useSession()
  const queryClient = useQueryClient()
  const { data: unitList = [] } = useUnits()

  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [mode, setMode] = useState<'instant' | 'exam'>('instant')
  const [timeLimitMin, setTimeLimitMin] = useState(20)
  const [items, setItems] = useState<Question[]>([])
  const [dirty, setDirty] = useState(false)
  const [loadedSetId, setLoadedSetId] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [previewing, setPreviewing] = useState<Question | null>(null)

  const detailQuery = useQuery({
    queryKey: ['admin-set-detail', routeSetId],
    queryFn: () => api.request<SetDetail>(`/api/admin/sets/${routeSetId}`),
    enabled: !isNew,
  })
  if (!isNew && detailQuery.data && loadedSetId !== routeSetId) {
    setLoadedSetId(routeSetId ?? null)
    setTitle(detailQuery.data.set.title)
    setDescription(detailQuery.data.set.description ?? '')
    setMode(detailQuery.data.set.mode)
    setTimeLimitMin(Math.max(1, Math.round((detailQuery.data.set.timeLimitSec ?? 1200) / 60)))
    // The questions query has no ORDER BY; the items array is the authority.
    const byID = new Map(detailQuery.data.questions.map((question) => [question.id, question]))
    setItems(detailQuery.data.items.map((item) => byID.get(item.questionId)).filter((q): q is Question => !!q))
  }

  // All stimuli power the cluster headers (documents included, small table).
  const { data: stimuliAll = [] } = useQuery({
    queryKey: ['admin-stimuli'],
    queryFn: async () => (await api.request<Stimulus[] | null>('/api/admin/stimuli')) ?? [],
  })
  const stimulusByID = useMemo(() => new Map(stimuliAll.map((stimulus) => [stimulus.id, stimulus])), [stimuliAll])

  // ---- bank browser filters ----
  const [search, setSearch] = useState('')
  const [filterUnit, setFilterUnit] = useState('')
  const [filterTopic, setFilterTopic] = useState('')
  const [filterType, setFilterType] = useState('')
  const [filterFormat, setFilterFormat] = useState('')
  const [filterTag, setFilterTag] = useState('')
  const pickerParams = new URLSearchParams({ limit: '200', status: 'published' })
  if (search.trim()) pickerParams.set('search', search.trim())
  if (filterUnit) pickerParams.set('unitId', filterUnit)
  if (filterTopic) pickerParams.set('topicId', filterTopic)
  if (filterType) pickerParams.set('type', filterType)
  if (filterFormat) pickerParams.set('format', filterFormat)
  if (filterTag.trim()) pickerParams.set('tags', filterTag.trim())
  const { data: candidateData } = useQuery({
    queryKey: ['admin-questions', 'picker', search, filterUnit, filterTopic, filterType, filterFormat, filterTag],
    queryFn: async () => (await api.request<Question[] | null>(`/api/admin/questions?${pickerParams.toString()}`)) ?? [],
  })
  const candidates = candidateData ?? []
  const { data: tagData } = useQuery({
    queryKey: ['admin-tags'],
    queryFn: async () => (await api.request<{ name: string; questions: number }[] | null>('/api/admin/tags')) ?? [],
  })

  const itemIDs = useMemo(() => new Set(items.map((question) => question.id)), [items])
  const topicOptions = unitList.find((unit) => unit.id === filterUnit)?.topics ?? []

  // ---- coverage rollup (client-side; the questions carry the links) ----
  const coverage = useMemo(() => {
    const units = new Map<string, number>()
    const topics = new Map<string, number>()
    const concepts = new Map<string, { term: string; count: number }>()
    let unlinked = 0
    for (const item of items) {
      const linked = (item.unitId ? 1 : 0) + (item.topicId ? 1 : 0) + (item.concepts?.length ? 1 : 0)
      if (linked === 0) unlinked++
      if (item.unitId) units.set(item.unitId, (units.get(item.unitId) ?? 0) + 1)
      if (item.topicId) topics.set(item.topicId, (topics.get(item.topicId) ?? 0) + 1)
      for (const concept of item.concepts ?? []) {
        const entry = concepts.get(concept.id) ?? { term: concept.term, count: 0 }
        entry.count++
        concepts.set(concept.id, entry)
      }
    }
    const unitTitle = (id: string) => unitList.find((unit) => unit.id === id)?.title ?? id
    const topicTitle = (id: string) => {
      for (const unit of unitList) {
        const topic = unit.topics.find((candidate) => candidate.id === id)
        if (topic) return topic.title
      }
      return id
    }
    const maxUnit = Math.max(1, ...units.values())
    return {
      unlinked,
      maxUnit,
      units: [...units.entries()].sort((a, b) => b[1] - a[1]).map(([id, count]) => ({ id, title: unitTitle(id), count })),
      topics: [...topics.entries()].sort((a, b) => b[1] - a[1]).slice(0, 8).map(([id, count]) => ({ id, title: topicTitle(id), count })),
      concepts: [...concepts.values()].sort((a, b) => b.count - a.count).slice(0, 24),
    }
  }, [items, unitList])

  // ---- ordering helpers ----
  const [dragIndex, setDragIndex] = useState<number | null>(null)
  const ranges = clusterRanges(items)

  function mutate(next: Question[]) {
    setItems(next)
    setDirty(true)
  }

  function swap(from: number, to: number) {
    if (to < 0 || to >= items.length || from === to) return
    const next = [...items]
    const [moved] = next.splice(from, 1)
    next.splice(to, 0, moved)
    mutate(next)
  }

  // Move one contiguous cluster (by its range) up or down as a unit.
  function moveBlock(start: number, length: number, delta: number) {
    const target = delta < 0 ? start - 1 : start + length
    if (target < 0 || target >= items.length) return
    const next = [...items]
    const block = next.splice(start, length)
    const insertAt = delta < 0 ? target : target - length + 1
    next.splice(insertAt, 0, ...block)
    mutate(next)
  }

  function addQuestion(question: Question) {
    if (itemIDs.has(question.id)) return
    mutate([...items, question])
  }

  async function addGroup(question: Question) {
    if (!question.stimulusId) return
    try {
      const siblings = await api.request<Question[] | null>(
        `/api/admin/questions?stimulusId=${question.stimulusId}&status=published&limit=100`,
      )
      const existing = new Set(items.map((item) => item.id))
      const additions = (siblings ?? []).filter((sibling) => !existing.has(sibling.id))
      if (additions.length === 0) return
      mutate([...items, ...additions])
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  function goBack() {
    if (dirty && !window.confirm(t.unsavedChanges)) return
    navigate('/admin/sets')
  }

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
      if (isNew) {
        const created = await api.request<PracticeSet>('/api/admin/sets', { method: 'POST', body: JSON.stringify(body) })
        setDirty(false)
        navigate(`/admin/sets/${created.id}`)
      } else {
        await api.request(`/api/admin/sets/${routeSetId}`, { method: 'PATCH', body: JSON.stringify(body) })
        setDirty(false)
      }
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
    <div className="set-editor-page">
      <div className="set-editor-topbar">
        <button className="secondary" onClick={goBack}>
          <ArrowLeft size={15} /> {t.setsTitle}
        </button>
        <strong>{isNew ? t.newSet : title || t.editSet}</strong>
        <div className="row-actions">
          <button className="secondary" disabled={busy || !title.trim() || items.length === 0} onClick={() => void save('draft')}>
            {t.saveDraft}
          </button>
          <button className="primary" disabled={busy || !title.trim() || items.length === 0} onClick={() => void save('published')}>
            {t.publishSet}
          </button>
        </div>
      </div>
      {error && <div className="error">{error}</div>}
      <div className="set-editor-layout">
        <aside className="set-meta">
          <label>
            {t.setTitle}
            <input value={title} onChange={(e) => { setTitle(e.target.value); setDirty(true) }} />
          </label>
          <label>
            {t.description}
            <input value={description} onChange={(e) => { setDescription(e.target.value); setDirty(true) }} />
          </label>
          <label>
            {t.mode}
            <div className="segmented inline">
              <button className={mode === 'instant' ? 'active' : ''} onClick={() => { setMode('instant'); setDirty(true) }}>
                {t.instantMode}
              </button>
              <button className={mode === 'exam' ? 'active' : ''} onClick={() => { setMode('exam'); setDirty(true) }}>
                {t.examMode}
              </button>
            </div>
          </label>
          {mode === 'exam' && (
            <label>
              {t.timeLimit} ({t.minutes})
              <input
                type="number"
                min={1}
                value={timeLimitMin}
                onChange={(e) => { setTimeLimitMin(Number(e.target.value)); setDirty(true) }}
              />
            </label>
          )}
          <div className="coverage-panel">
            <strong>{t.coverageTitle}</strong>
            {coverage.units.map((unit) => (
              <div className="coverage-bar" key={unit.id}>
                <span>{unit.title}</span>
                <div className="bar">
                  <div style={{ width: `${(unit.count / coverage.maxUnit) * 100}%` }} />
                </div>
                <em>{unit.count}</em>
              </div>
            ))}
            {coverage.units.length === 0 && <small className="muted">{t.noCoverageYet}</small>}
            {coverage.topics.length > 0 && (
              <div className="coverage-topics">
                {coverage.topics.map((topic) => (
                  <small key={topic.id}>
                    {topic.title} · {topic.count}
                  </small>
                ))}
              </div>
            )}
            {coverage.concepts.length > 0 && (
              <div className="chip-row">
                {coverage.concepts.map((concept) => (
                  <span className="chip" key={concept.term}>
                    {concept.term} ×{concept.count}
                  </span>
                ))}
              </div>
            )}
            {coverage.unlinked > 0 && (
              <small className="coverage-warning">
                ⚠ {coverage.unlinked} {t.unlinkedQuestions}
              </small>
            )}
          </div>
        </aside>

        <section className="set-paper">
          <strong>
            {t.selectedQuestions} ({items.length})
          </strong>
          {items.length === 0 && <div className="empty-state">{t.emptyPaper}</div>}
          {ranges.map((range) => {
            const stimulus = range.stimulusId ? stimulusByID.get(range.stimulusId) : undefined
            return (
              <div className={`paper-block ${range.stimulusId ? 'has-stimulus' : ''}`} key={`${range.stimulusId ?? 'solo'}-${range.start}`}>
                {range.stimulusId && (
                  <div className="block-head">
                    <Layers size={14} />
                    <span>
                      {stimulus?.title ?? t.stimulusMissing} · {stimulus ? kindLabel(t, stimulus.kind) : ''} · {range.length} {t.questions}
                    </span>
                    <div className="row-actions">
                      <button className="icon-line" title={t.moveUp} disabled={range.start === 0} onClick={() => moveBlock(range.start, range.length, -1)}>
                        <ArrowUp size={14} />
                      </button>
                      <button
                        className="icon-line"
                        title={t.moveDown}
                        disabled={range.start + range.length >= items.length}
                        onClick={() => moveBlock(range.start, range.length, 1)}
                      >
                        <ArrowDown size={14} />
                      </button>
                    </div>
                  </div>
                )}
                {items.slice(range.start, range.start + range.length).map((question, offset) => {
                  const index = range.start + offset
                  return (
                    <div
                      className="set-item-row"
                      key={question.id}
                      draggable
                      onDragStart={() => setDragIndex(index)}
                      onDragEnd={() => setDragIndex(null)}
                      onDragOver={(event) => event.preventDefault()}
                      onDrop={() => {
                        if (dragIndex != null && dragIndex !== index) swap(dragIndex, index)
                        setDragIndex(null)
                      }}
                    >
                      <span className="drag-handle" aria-hidden>
                        ⠿
                      </span>
                      <button className="row-main" onClick={() => setPreviewing(question)}>
                        <span>
                          <strong>
                            {index + 1}. {question.stem.slice(0, 90)}
                            {question.stem.length > 90 ? '…' : ''}
                          </strong>
                          <small>{questionLabel(t, question)}</small>
                        </span>
                      </button>
                      <div className="row-actions">
                        <button className="icon-line" title={t.moveUp} disabled={index === 0} onClick={() => swap(index, index - 1)}>
                          <ArrowUp size={14} />
                        </button>
                        <button
                          className="icon-line"
                          title={t.moveDown}
                          disabled={index === items.length - 1}
                          onClick={() => swap(index, index + 1)}
                        >
                          <ArrowDown size={14} />
                        </button>
                        <button
                          className="icon-line"
                          title={t.delete}
                          onClick={() => mutate(items.filter((_, i) => i !== index))}
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </div>
                  )
                })}
              </div>
            )
          })}
        </section>

        <aside className="set-picker">
          <strong>{t.pickQuestions}</strong>
          <label className="search">
            <Search size={16} />
            <input placeholder={t.searchStems} value={search} onChange={(e) => setSearch(e.target.value)} />
          </label>
          <div className="picker-filters">
            <select value={filterUnit} onChange={(e) => { setFilterUnit(e.target.value); setFilterTopic('') }}>
              <option value="">{t.allUnits}</option>
              {unitList.map((unit) => (
                <option key={unit.id} value={unit.id}>
                  {unit.title}
                </option>
              ))}
            </select>
            <select value={filterTopic} onChange={(e) => setFilterTopic(e.target.value)} disabled={!filterUnit}>
              <option value="">{t.allTopics}</option>
              {topicOptions.map((topic) => (
                <option key={topic.id} value={topic.id}>
                  {topic.title}
                </option>
              ))}
            </select>
            <select value={filterType} onChange={(e) => setFilterType(e.target.value)}>
              <option value="">{t.all}</option>
              <option value="mcq">{t.mcq}</option>
              <option value="subjective">{t.subjective}</option>
            </select>
            <select value={filterFormat} onChange={(e) => setFilterFormat(e.target.value)}>
              <option value="">{t.allFormats}</option>
              <option value="frq">FRQ</option>
              <option value="aaq">AAQ</option>
              <option value="ebq">EBQ</option>
            </select>
            <input list="set-picker-tags" placeholder={t.tags} value={filterTag} onChange={(e) => setFilterTag(e.target.value)} />
            <datalist id="set-picker-tags">
              {(tagData ?? []).map((tag) => (
                <option key={tag.name} value={tag.name} />
              ))}
            </datalist>
          </div>
          <div className="candidate-list">
            {candidates
              .filter((question) => !itemIDs.has(question.id))
              .map((question) => (
                <div className="candidate-row" key={question.id}>
                  <button className="row-main" onClick={() => setPreviewing(question)}>
                    <span>
                      <strong>{question.stem.slice(0, 110)}</strong>
                      <small>
                        {questionLabel(t, question)} · {(question.tags ?? []).map((tag) => tag.name).join(', ')}
                      </small>
                    </span>
                  </button>
                  <div className="row-actions">
                    {question.stimulusId && (
                      <button className="secondary" title={t.addGroup} onClick={() => void addGroup(question)}>
                        <Layers size={14} />
                      </button>
                    )}
                    <button className="secondary" title={t.preview} onClick={() => setPreviewing(question)}>
                      <Eye size={14} />
                    </button>
                    <button className="secondary" title={t.add} onClick={() => addQuestion(question)}>
                      <Plus size={14} />
                    </button>
                  </div>
                </div>
              ))}
            {candidates.filter((question) => !itemIDs.has(question.id)).length === 0 && (
              <small className="muted">{t.noCandidates}</small>
            )}
          </div>
        </aside>
      </div>
      {previewing && <QuestionPreview question={previewing} stimulus={previewing.stimulusId ? stimulusByID.get(previewing.stimulusId) : undefined} onAdd={itemIDs.has(previewing.id) ? undefined : () => { addQuestion(previewing); setPreviewing(null) }} onClose={() => setPreviewing(null)} />}
    </div>
  )
}

// Read-only full-question preview used by the bank browser.
function QuestionPreview({
  question,
  stimulus,
  onAdd,
  onClose,
}: {
  question: Question
  stimulus?: Stimulus
  onAdd?: () => void
  onClose: () => void
}) {
  const { t } = useSession()
  return (
    <Modal wide title={questionLabel(t, question)} onClose={onClose}>
      <div className="question-preview">
        {stimulus && (
          <div className="preview-stimulus">
            <strong>
              {stimulus.title} · {kindLabel(t, stimulus.kind)}
            </strong>
            {(stimulus.documents ?? []).map((doc, i) => (
              <details key={i} open={i === 0}>
                <summary>{doc.title || `${t.document} ${i + 1}`}</summary>
                <MarkdownText text={doc.text} />
              </details>
            ))}
          </div>
        )}
        <div className="preview-stem">
          <MarkdownText text={question.stem} />
        </div>
        {(question.materials ?? []).map((material, i) => (
          <div className="material-block" key={i}>
            {material.title && <strong>{material.title}</strong>}
            <MarkdownText text={material.text} />
          </div>
        ))}
        {question.type === 'mcq' ? (
          <div className="preview-choices">
            {(question.choices ?? []).map((choice) => (
              <div className={`preview-choice ${choice.key === question.answerKey ? 'correct' : ''}`} key={choice.key}>
                <span className="choice-key">{choice.key}</span>
                <span>
                  <InlineMarkdown text={choice.text} />
                </span>
              </div>
            ))}
            {question.explanation && (
              <p className="muted">
                <strong>{t.explanation}:</strong> {question.explanation}
              </p>
            )}
          </div>
        ) : (
          <div className="preview-parts">
            {(question.parts ?? []).map((part) => (
              <div className="preview-part" key={part.label}>
                <strong>
                  ({part.label})
                  {part.points != null ? ` · ${part.points} ${t.points}` : ''}
                </strong>
                <p>{part.prompt}</p>
                {part.referenceAnswer && (
                  <p className="muted">
                    <strong>{t.reference}:</strong> {part.referenceAnswer}
                  </p>
                )}
                {(part.rubric ?? []).length > 0 && (
                  <ul className="muted">
                    {(part.rubric ?? []).map((point, i) => (
                      <li key={i}>{point}</li>
                    ))}
                  </ul>
                )}
              </div>
            ))}
          </div>
        )}
        <div className="action-row">
          <button className="secondary" onClick={onClose}>
            {t.close}
          </button>
          {onAdd && (
            <button className="primary" onClick={onAdd}>
              <Plus size={14} /> {t.add}
            </button>
          )}
        </div>
      </div>
    </Modal>
  )
}
