import { useSession } from '../hooks/session'
import type { ConceptStatus } from '../lib/types'

// Three-tier self-assessment buttons (proficient / fuzzy / unknown).
export function StatusButtons({
  status,
  onMark,
  size = 'normal',
}: {
  status: ConceptStatus
  onMark: (status: ConceptStatus) => void
  size?: 'normal' | 'large'
}) {
  const { t } = useSession()
  const options: { value: Exclude<ConceptStatus, ''>; label: string; key: string; tone: string }[] = [
    { value: 'proficient', label: t.proficient, key: '1', tone: 'success' },
    { value: 'fuzzy', label: t.fuzzy, key: '2', tone: 'warn' },
    { value: 'unknown', label: t.unknown, key: '3', tone: 'danger' },
  ]
  return (
    <div className={`status-buttons ${size}`} onClick={(event) => event.stopPropagation()}>
      {options.map((option) => (
        <button
          key={option.value}
          className={`${option.tone} ${status === option.value ? 'active' : ''}`}
          onClick={() => onMark(option.value)}
          title={`${option.label} (${option.key})`}
        >
          {option.label}
        </button>
      ))}
    </div>
  )
}
