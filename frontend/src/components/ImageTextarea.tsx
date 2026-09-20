import { useRef, useState } from 'react'
import { ImagePlus } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'

// Textarea for markdown bodies that may embed images: pasting or dropping an
// image uploads it to /api/admin/question-images and inserts the returned
// /files URL as inline markdown at the caret.
export function ImageTextarea({
  value,
  onChange,
  rows = 3,
  placeholder,
  ariaLabel,
}: {
  value: string
  onChange: (value: string) => void
  rows?: number
  placeholder?: string
  ariaLabel?: string
}) {
  const { t } = useSession()
  const ref = useRef<HTMLTextAreaElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState('')

  async function insertFiles(files: FileList | File[]) {
    const image = Array.from(files).find((file) => file.type.startsWith('image/'))
    if (!image) return
    setUploading(true)
    setError('')
    try {
      const form = new FormData()
      form.append('file', image)
      const result = await api.request<{ url: string }>('/api/admin/question-images', {
        method: 'POST',
        body: form,
      })
      const markdown = `![${image.name.replace(/\.[^.]+$/, '') || 'image'}](${result.url})`
      const el = ref.current
      if (!el) {
        onChange(value ? `${value}\n${markdown}` : markdown)
        return
      }
      const start = el.selectionStart ?? value.length
      const end = el.selectionEnd ?? value.length
      onChange(value.slice(0, start) + markdown + value.slice(end))
      requestAnimationFrame(() => {
        el.focus()
        const pos = start + markdown.length
        el.setSelectionRange(pos, pos)
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setUploading(false)
    }
  }

  function intercept(event: React.ClipboardEvent | React.DragEvent) {
    const files = 'clipboardData' in event ? event.clipboardData?.files : event.dataTransfer?.files
    if (!files?.length) return
    if (!Array.from(files).some((file) => file.type.startsWith('image/'))) return
    event.preventDefault()
    void insertFiles(files)
  }

  return (
    <div className="image-textarea">
      <textarea
        ref={ref}
        aria-label={ariaLabel}
        rows={rows}
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onPaste={intercept}
        onDrop={intercept}
        onDragOver={(e) => {
          if (Array.from(e.dataTransfer?.types ?? []).includes('Files')) e.preventDefault()
        }}
      />
      <div className="image-tools">
        <button type="button" className="secondary" disabled={uploading} onClick={() => inputRef.current?.click()}>
          <ImagePlus size={14} /> {uploading ? t.imageUploading : t.addImage}
        </button>
        {error && <small className="error-text">{error}</small>}
      </div>
      <input
        ref={inputRef}
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp"
        hidden
        onChange={(e) => {
          if (e.target.files?.length) void insertFiles(e.target.files)
          e.target.value = ''
        }}
      />
    </div>
  )
}
