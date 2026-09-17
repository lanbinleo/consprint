export function percent(value: number, total: number) {
  if (total <= 0) return 0
  return Math.round((value / total) * 100)
}

export function initials(name: string) {
  return (
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase())
      .join('') || 'AP'
  )
}

export function examCountdown(target: string | undefined, now = Date.now()) {
  if (!target) return null
  const time = new Date(target).getTime()
  if (!Number.isFinite(time)) return null
  const diff = Math.max(0, time - now)
  const days = Math.floor(diff / 86_400_000)
  const hours = Math.floor((diff % 86_400_000) / 3_600_000)
  const minutes = Math.floor((diff % 3_600_000) / 60_000)
  const seconds = Math.floor((diff % 60_000) / 1000)
  return { days, hours, minutes, seconds }
}

export function formatClock(totalSeconds: number) {
  const clamped = Math.max(0, Math.floor(totalSeconds))
  const minutes = Math.floor(clamped / 60)
  const seconds = clamped % 60
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

export function shortLabel(label: string) {
  if (label.includes('-')) return label.slice(5)
  return label
}

export function sourceLabel(source?: string) {
  const clean = (source ?? '').trim()
  if (!clean || clean === 'pending') return 'Awaiting Study Content'
  if (clean === 'unit0.md') return 'Unit 0 Key Terms'
  if (clean === 'unit1.md') return 'Unit 1 Study Notes'
  if (clean === 'ai-enrichment.compact') return 'AI AP Psych Notes'
  if (clean.includes('AP Psychology Notes')) return 'AP Psychology Notes'
  if (clean === 'manual') return 'Teacher Edited'
  if (clean.startsWith('manual')) return 'Reviewed Notes'
  return clean.replace(/\.(md|txt|opml)$/i, '').replace(/[-_]/g, ' ')
}

export function formatDateTime(value: string | Date | null | undefined) {
  if (!value) return ''
  const date = typeof value === 'string' ? new Date(value) : value
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}
