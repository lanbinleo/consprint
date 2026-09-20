import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { FilePlus2, Plus, Trash2 } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { ImageTextarea } from './ImageTextarea'
import type { Stimulus, StimulusDocument } from '../lib/types'

// Pick a shared stimulus (MCQ passage / AAQ article / EBQ sources) for a
// question, or create one inline. Creating uploads and publishes immediately
// so the parent only ever deals with a stimulus id.
export function StimulusPicker({
  value,
  onChange,
  defaultKind = 'passage',
}: {
  value: string
  onChange: (id: string) => void
  defaultKind?: Stimulus['kind']
}) {
  const { t } = useSession()
  const queryClient = useQueryClient()
  const { data: stimuli = [] } = useQuery({
    queryKey: ['admin-stimuli'],
    queryFn: async () => (await api.request<Stimulus[] | null>('/api/admin/stimuli')) ?? [],
  })
  const [creating, setCreating] = useState(false)
  const [title, setTitle] = useState('')
  const [kind, setKind] = useState<Stimulus['kind']>(defaultKind)
  const [docs, setDocs] = useState<StimulusDocument[]>([{ title: '', text: '' }])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const selected = stimuli.find((stimulus) => stimulus.id === value)

  async function create() {
    setBusy(true)
    setError('')
    try {
      const created = await api.request<Stimulus>('/api/admin/stimuli', {
        method: 'POST',
        body: JSON.stringify({ title, kind, documents: docs }),
      })
      void queryClient.invalidateQueries({ queryKey: ['admin-stimuli'] })
      onChange(created.id)
      setCreating(false)
      setTitle('')
      setKind(defaultKind)
      setDocs([{ title: '', text: '' }])
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  if (creating) {
    return (
      <div className="stimulus-editor">
        <div className="scope-grid">
          <label>
            {t.stimulusTitle}
            <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t.stimulusTitlePlaceholder} />
          </label>
          <label>
            {t.stimulusKind}
            <select value={kind} onChange={(e) => setKind(e.target.value as Stimulus['kind'])}>
              <option value="passage">{t.kindPassage}</option>
              <option value="article">{t.kindArticle}</option>
              <option value="sources">{t.kindSources}</option>
            </select>
          </label>
        </div>
        {docs.map((doc, i) => (
          <div className="doc-edit" key={i}>
            <div className="doc-head">
              <input
                placeholder={`${t.document} ${i + 1}`}
                value={doc.title}
                onChange={(e) => setDocs((rows) => rows.map((row, j) => (i === j ? { ...row, title: e.target.value } : row)))}
              />
              {docs.length > 1 && (
                <button type="button" className="icon-btn" onClick={() => setDocs((rows) => rows.filter((_, j) => j !== i))}>
                  <Trash2 size={14} />
                </button>
              )}
            </div>
            <ImageTextarea
              ariaLabel={t.stimulusDocument}
              rows={kind === 'sources' ? 5 : 8}
              value={doc.text}
              placeholder={t.stimulusDocumentHint}
              onChange={(text) => setDocs((rows) => rows.map((row, j) => (i === j ? { ...row, text } : row)))}
            />
          </div>
        ))}
        <button type="button" className="secondary" onClick={() => setDocs((rows) => [...rows, { title: '', text: '' }])}>
          <Plus size={14} /> {t.addDocument}
        </button>
        {error && <div className="error">{error}</div>}
        <div className="action-row">
          <button type="button" className="secondary" onClick={() => setCreating(false)}>
            {t.cancel}
          </button>
          <button
            type="button"
            className="primary"
            disabled={busy || !title.trim() || !docs.some((doc) => doc.text.trim())}
            onClick={create}
          >
            {t.saveStimulus}
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="stimulus-picker">
      <select value={value} onChange={(e) => onChange(e.target.value)} aria-label={t.stimulus}>
        <option value="">{t.noStimulus}</option>
        {stimuli.map((stimulus) => (
          <option key={stimulus.id} value={stimulus.id}>
            {stimulus.title} · {kindLabel(t, stimulus.kind)} ·{' '}
            {stimulus.documentCount ?? stimulus.documents?.length ?? 0} {t.docs}
          </option>
        ))}
      </select>
      {selected && selected.documents && selected.documents.length > 0 && (
        <small className="muted">{selected.documents.map((doc) => doc.title || t.untitled).join(' / ')}</small>
      )}
      <button type="button" className="secondary" onClick={() => setCreating(true)}>
        <FilePlus2 size={14} /> {t.newStimulus}
      </button>
    </div>
  )
}

export function kindLabel(
  t: { kindPassage: string; kindArticle: string; kindSources: string },
  kind: Stimulus['kind'],
) {
  if (kind === 'article') return t.kindArticle
  if (kind === 'sources') return t.kindSources
  return t.kindPassage
}
