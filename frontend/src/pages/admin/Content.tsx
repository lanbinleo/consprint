import { useEffect, useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Database, Edit3, Save, Search } from 'lucide-react'
import { api } from '../../lib/api'
import { sourceLabel } from '../../lib/format'
import { useSession } from '../../hooks/session'
import { Header, ListSkeleton, Metric, SpinnerButton } from '../../components/ui'
import { InlineMarkdown } from '../../components/InlineMarkdown'
import { imageOnlyBlock } from '../../lib/inlineMarkdown'
import { attachUnitTopic } from '../../lib/conceptStore'
import { useUnits } from '../../components/ScopePicker'
import type { Block, Concept, ImportStatus } from '../../lib/types'

export function Content() {
  const { t, lang } = useSession()
  const queryClient = useQueryClient()
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState<Concept | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const { data: units = [] } = useUnits()

  // Debounce: every keystroke would otherwise refetch the admin list.
  useEffect(() => {
    const timer = setTimeout(() => setSearch(searchInput.trim()), 300)
    return () => clearTimeout(timer)
  }, [searchInput])

  const { data: status } = useQuery({ queryKey: ['import-status'], queryFn: () => api.request<ImportStatus>('/api/import/status') })
  const { data: concepts = [], isPending } = useQuery({
    queryKey: ['concepts-admin', search],
    queryFn: async () => (await api.request<Concept[] | null>(`/api/concepts?search=${encodeURIComponent(search)}`)) ?? [],
  })
  // The slim list carries no unit/topic objects — join them from /api/units.
  const rows = useMemo(() => attachUnitTopic(concepts, units), [concepts, units])

  async function runImport() {
    setBusy(true)
    setError('')
    try {
      await api.request('/api/import/run', { method: 'POST', body: '{}' })
      await queryClient.invalidateQueries({ queryKey: ['import-status'] })
      await queryClient.invalidateQueries({ queryKey: ['concepts-admin'] })
      // The shared concept store picks the new corpus up via its version check.
      void queryClient.invalidateQueries({ queryKey: ['concepts'] })
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <Header
        eyebrow={t.admin}
        title={t.contentTitle}
        action={
          <SpinnerButton busy={busy} onClick={runImport}>
            <Database size={16} /> {t.reimport}
          </SpinnerButton>
        }
      />
      {error && <div className="error">{error}</div>}
      <div className="data-layout">
        <div className="data-list">
          <div className="metrics compact">
            <Metric label={t.totalTerms} value={status?.concepts ?? 0} loading={!status} />
            <Metric label={t.ready} value={status?.readyConcepts ?? 0} loading={!status} />
          </div>
          <label className="search">
            <Search size={16} />
            <input placeholder={t.search} value={searchInput} onChange={(e) => setSearchInput(e.target.value)} />
          </label>
          {isPending ? (
            <ListSkeleton />
          ) : (
            <div className="table">
              {rows.slice(0, 250).map((concept) => (
                <button
                  className={`concept-row ${selected?.id === concept.id ? 'selected' : ''}`}
                  key={concept.id}
                  onClick={() => setSelected(concept)}
                >
                  <span>
                    <strong>{concept.term}</strong>
                    <small>{sourceLabel(concept.content?.source ?? concept.contentStatus, lang)}</small>
                  </span>
                  <Edit3 size={16} />
                </button>
              ))}
            </div>
          )}
        </div>
        <ConceptEditor
          concept={selected}
          onSaved={(concept) => {
            setSelected(concept)
            // Propagate the edit to the shared concept store (version check
            // will fetch just this row as a delta).
            void queryClient.invalidateQueries({ queryKey: ['concepts'] })
          }}
        />
      </div>
    </div>
  )
}

function ConceptEditor({ concept, onSaved }: { concept: Concept | null; onSaved: (concept: Concept) => void }) {
  const { t } = useSession()
  const [definition, setDefinition] = useState('')
  const [examples, setExamples] = useState('')
  const [pitfalls, setPitfalls] = useState('')
  const [notes, setNotes] = useState('')
  const [preview, setPreview] = useState(false)
  const [saved, setSaved] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    setDefinition(blocksToText(concept?.content?.definition))
    setExamples(blocksToText(concept?.content?.examples))
    setPitfalls(blocksToText(concept?.content?.pitfalls))
    setNotes(blocksToText(concept?.content?.notes))
  }, [concept?.id, concept?.content])

  if (!concept) return <aside className="editor-panel empty">{t.pickEdit}</aside>

  async function save() {
    if (!concept) return
    setBusy(true)
    setError('')
    try {
      const updated = await api.request<Concept>(`/api/concepts/${concept.id}/content`, {
        method: 'PATCH',
        body: JSON.stringify({
          definition: textToBlocks(definition),
          examples: textToBlocks(examples),
          pitfalls: textToBlocks(pitfalls),
          notes: textToBlocks(notes),
          source: 'manual',
        }),
      })
      onSaved(updated)
      setSaved(true)
      setTimeout(() => setSaved(false), 1200)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  const fields = (
    <>
      <label>
        {t.definition}
        {preview ? <FieldPreview text={definition} /> : <textarea value={definition} onChange={(e) => setDefinition(e.target.value)} />}
      </label>
      <label>
        {t.examples}
        {preview ? <FieldPreview text={examples} /> : <textarea value={examples} onChange={(e) => setExamples(e.target.value)} />}
      </label>
      <label>
        {t.pitfalls}
        {preview ? <FieldPreview text={pitfalls} tone="warn" /> : <textarea value={pitfalls} onChange={(e) => setPitfalls(e.target.value)} />}
      </label>
      <label>
        {t.contentNotes}
        {preview ? <FieldPreview text={notes} /> : <textarea value={notes} onChange={(e) => setNotes(e.target.value)} />}
      </label>
    </>
  )

  return (
    <aside className="editor-panel">
      <small>{concept.topic?.title}</small>
      <h2>{concept.term}</h2>
      <div className="editor-toolbar">
        <div className="segmented inline">
          <button className={!preview ? 'active' : ''} onClick={() => setPreview(false)}>
            {t.editingContent}
          </button>
          <button className={preview ? 'active' : ''} onClick={() => setPreview(true)}>
            {t.previewContent}
          </button>
        </div>
      </div>
      {fields}
      {error && <div className="error">{error}</div>}
      <SpinnerButton busy={busy} onClick={save}>
        <Save size={16} /> {saved ? t.saved : t.saveContent}
      </SpinnerButton>
    </aside>
  )
}

// Renders one edited field exactly the way RichContent will show it to
// students: one paragraph per line, image-only lines become figures.
function FieldPreview({ text, tone = '' }: { text: string; tone?: string }) {
  const lines = text
    .split(/\n+/)
    .map((line) => line.trim())
    .filter(Boolean)
  if (!lines.length) return <p className="muted">—</p>
  return (
    <div className={`rich-content preview ${tone}`}>
      {lines.map((line, index) => {
        const figure = imageOnlyBlock(line)
        if (figure) {
          return (
            <figure key={index}>
              <img src={figure.src} alt={figure.alt} loading="lazy" />
            </figure>
          )
        }
        return (
          <p key={index}>
            <InlineMarkdown text={line} />
          </p>
        )
      })}
    </div>
  )
}

function blocksToText(blocks?: Block[] | null) {
  return blocks?.map((block) => block.text).join('\n') ?? ''
}

function textToBlocks(text: string) {
  return text
    .split(/\n+/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => ({ type: 'paragraph', text: line }))
}
