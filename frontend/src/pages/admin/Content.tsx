import { useEffect, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Database, Edit3, Save, Search } from 'lucide-react'
import { api } from '../../lib/api'
import { sourceLabel } from '../../lib/format'
import { useSession } from '../../hooks/session'
import { Header, ListSkeleton, Metric, SpinnerButton } from '../../components/ui'
import type { Block, Concept, ConceptRow, ImportStatus } from '../../lib/types'

export function Content() {
  const { t } = useSession()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState<ConceptRow | null>(null)
  const [busy, setBusy] = useState(false)

  const { data: status } = useQuery({ queryKey: ['import-status'], queryFn: () => api.request<ImportStatus>('/api/import/status') })
  const { data: concepts = [], isPending } = useQuery({
    queryKey: ['concepts-admin', search],
    queryFn: () => api.request<ConceptRow[]>(`/api/concepts?search=${encodeURIComponent(search)}`),
  })

  async function runImport() {
    setBusy(true)
    try {
      await api.request('/api/import/run', { method: 'POST', body: '{}' })
      await queryClient.invalidateQueries({ queryKey: ['import-status'] })
      await queryClient.invalidateQueries({ queryKey: ['concepts-admin'] })
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
      <div className="data-layout">
        <div className="data-list">
          <div className="metrics compact">
            <Metric label={t.totalTerms} value={status?.concepts ?? 0} loading={!status} />
            <Metric label={t.ready} value={status?.readyConcepts ?? 0} loading={!status} />
          </div>
          <label className="search">
            <Search size={16} />
            <input placeholder={t.search} value={search} onChange={(e) => setSearch(e.target.value)} />
          </label>
          {isPending ? (
            <ListSkeleton />
          ) : (
            <div className="table">
              {concepts.slice(0, 250).map((concept) => (
                <button
                  className={`concept-row ${selected?.id === concept.id ? 'selected' : ''}`}
                  key={concept.id}
                  onClick={() => setSelected(concept)}
                >
                  <span>
                    <strong>{concept.term}</strong>
                    <small>{sourceLabel(concept.content?.source ?? concept.contentStatus)}</small>
                  </span>
                  <Edit3 size={16} />
                </button>
              ))}
            </div>
          )}
        </div>
        <ConceptEditor concept={selected} onSaved={(concept) => setSelected({ ...(selected ?? ({} as ConceptRow)), ...concept })} />
      </div>
    </div>
  )
}

function ConceptEditor({ concept, onSaved }: { concept: ConceptRow | null; onSaved: (concept: Concept) => void }) {
  const { t } = useSession()
  const [definition, setDefinition] = useState('')
  const [examples, setExamples] = useState('')
  const [pitfalls, setPitfalls] = useState('')
  const [notes, setNotes] = useState('')
  const [saved, setSaved] = useState(false)
  const [busy, setBusy] = useState(false)

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
    } finally {
      setBusy(false)
    }
  }

  return (
    <aside className="editor-panel">
      <small>{concept.topic?.title}</small>
      <h2>{concept.term}</h2>
      <label>
        {t.definition}
        <textarea value={definition} onChange={(e) => setDefinition(e.target.value)} />
      </label>
      <label>
        {t.examples}
        <textarea value={examples} onChange={(e) => setExamples(e.target.value)} />
      </label>
      <label>
        {t.pitfalls}
        <textarea value={pitfalls} onChange={(e) => setPitfalls(e.target.value)} />
      </label>
      <label>
        {t.notes}
        <textarea value={notes} onChange={(e) => setNotes(e.target.value)} />
      </label>
      <SpinnerButton busy={busy} onClick={save}>
        <Save size={16} /> {saved ? t.saved : t.saveContent}
      </SpinnerButton>
    </aside>
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
