import { useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Download, Upload } from 'lucide-react'
import { api } from '../../lib/api'
import { useSession } from '../../hooks/session'
import { Header } from '../../components/ui'
import type { ImportPreview } from '../../lib/types'

export function Import() {
  const { t } = useSession()
  const queryClient = useQueryClient()
  const fileInput = useRef<HTMLInputElement>(null)
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState('')

  async function upload(file: File) {
    setBusy(true)
    setError('')
    setResult('')
    try {
      const form = new FormData()
      form.append('file', file)
      const payload = await api.request<ImportPreview>('/api/admin/questions/import/preview', { method: 'POST', body: form })
      setPreview(payload)
      setSelected(new Set((payload.items ?? []).map((_, index) => index)))
    } catch (err) {
      setError(err instanceof Error ? err.message : t.importFailed)
    } finally {
      setBusy(false)
    }
  }

  async function commit(all: boolean) {
    if (!preview?.importId) return
    setBusy(true)
    try {
      const include = all ? undefined : Array.from(selected)
      const payload = await api.request<{ created: number }>('/api/admin/questions/import/commit', {
        method: 'POST',
        body: JSON.stringify({ importId: preview.importId, include }),
      })
      setResult(`${payload.created} ${t.importDone}`)
      setPreview(null)
      void queryClient.invalidateQueries({ queryKey: ['admin-questions'] })
    } catch (err) {
      setError(err instanceof Error ? err.message : t.importFailed)
    } finally {
      setBusy(false)
    }
  }

  function toggle(index: number) {
    setSelected((current) => {
      const next = new Set(current)
      if (next.has(index)) next.delete(index)
      else next.add(index)
      return next
    })
  }

  return (
    <div>
      <Header
        eyebrow={t.admin}
        title={t.importTitle}
        action={
          <button
            className="secondary"
            onClick={() => void api.download('/api/admin/questions/import/template', 'question-import-template.csv')}
          >
            <Download size={16} /> {t.importTemplate}
          </button>
        }
      />
      <p className="muted">{t.importHint}</p>
      <div className="upload-zone" onClick={() => fileInput.current?.click()}>
        <Upload size={22} />
        <span>{t.uploadFile}</span>
        <input
          ref={fileInput}
          type="file"
          accept=".csv,.json"
          hidden
          onChange={(e) => {
            const file = e.target.files?.[0]
            if (file) void upload(file)
            e.target.value = ''
          }}
        />
      </div>
      {error && <div className="error">{error}</div>}
      {result && <div className="success-note">{result}</div>}
      {preview && (
        <div className="import-preview">
          <div className="import-summary">
            <strong>
              {t.previewResult}: {preview.valid} {t.validRows} · {(preview.errors ?? []).length} {t.invalidRows} ({preview.channel.toUpperCase()})
            </strong>
            <div className="action-row">
              <button className="primary" disabled={busy || selected.size === 0} onClick={() => commit(false)}>
                {t.importSelected} ({selected.size})
              </button>
              <button className="secondary" disabled={busy || preview.valid === 0} onClick={() => commit(true)}>
                {t.importAll}
              </button>
            </div>
          </div>
          {(preview.errors ?? []).length > 0 && (
            <div className="import-errors">
              <strong>{t.rowErrors}</strong>
              {(preview.errors ?? []).map((issue) => (
                <p key={issue.index} className="error">
                  {t.row} {issue.index + 1}: {issue.message}
                </p>
              ))}
            </div>
          )}
          <div className="table">
            {(preview.items ?? []).map((item, index) => (
              <label className="concept-row import-row" key={index}>
                <input type="checkbox" checked={selected.has(index)} onChange={() => toggle(index)} />
                <span>
                  <strong>{item.stem.slice(0, 140)}</strong>
                  <small>
                    {item.type === 'mcq' ? t.mcq : item.format === 'aaq' ? t.formatAaq : item.format === 'ebq' ? t.formatEbg : `${t.subjective} · FRQ`}
                    {item.stimulus ? ` · ${t.stimulus}: ${item.stimulus.title}` : item.stimulusId ? ` · ${t.stimulus}: ✓` : ''} ·{' '}
                    {item.answerKey ?? '—'} · {t.tags}: {(item.tags ?? []).join('; ')}
                  </small>
                </span>
              </label>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
