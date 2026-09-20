import { useLayoutEffect, useRef } from 'react'

// Textarea that grows with its content (AAQ/EBQ per-part answers). Height is
// driven from scrollHeight after every value change; an optional maxRows
// keeps a runaway paste from filling the screen.
export function AutoGrowTextarea({
  value,
  onChange,
  onBlur,
  minRows = 3,
  maxRows,
  placeholder,
  ariaLabel,
  readOnly,
}: {
  value: string
  onChange: (value: string) => void
  onBlur?: () => void
  minRows?: number
  maxRows?: number
  placeholder?: string
  ariaLabel?: string
  readOnly?: boolean
}) {
  const ref = useRef<HTMLTextAreaElement>(null)

  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    el.style.height = 'auto'
    let height = el.scrollHeight
    if (maxRows) {
      const lineHeight = Number.parseFloat(getComputedStyle(el).lineHeight) || 22
      const max = minRows > maxRows ? minRows * lineHeight : maxRows * lineHeight
      height = Math.min(height, max)
    }
    el.style.height = `${Math.max(height, minRows * 24)}px`
  }, [value, minRows, maxRows])

  return (
    <textarea
      ref={ref}
      aria-label={ariaLabel}
      rows={minRows}
      value={value}
      readOnly={readOnly}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      onBlur={onBlur}
    />
  )
}
