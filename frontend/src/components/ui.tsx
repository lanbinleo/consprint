import { useEffect, type ReactNode } from 'react'
import { Keyboard, Loader2, UserRound, X } from 'lucide-react'
import { useSession } from '../hooks/session'

export function Header({ eyebrow, title, action }: { eyebrow?: string; title: string; action?: ReactNode }) {
  return (
    <div className="page-header">
      <div>
        {eyebrow && <span>{eyebrow}</span>}
        <h1>{title}</h1>
      </div>
      {action}
    </div>
  )
}

export function Metric({ label, value, loading = false }: { label: string; value: string | number; loading?: boolean }) {
  return (
    <div className={`metric ${loading ? 'soft-loading' : ''}`}>
      <span>{label}</span>
      <strong>{loading ? '…' : typeof value === 'number' && !Number.isInteger(value) ? value.toFixed(2) : value}</strong>
    </div>
  )
}

// Default avatars: pre-generated "Notionists" style from DiceBear
// (https://dicebear.com, CC BY 4.0), stored under public/avatars/.
// One of the 24 fixed seeds is assigned deterministically by name hash.
const DEFAULT_AVATAR_COUNT = 24

function hashName(name: string) {
  let hash = 0
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) >>> 0
  return hash
}

export function Avatar({ user }: { user: { name: string; avatarDataUrl?: string } }) {
  if (user.avatarDataUrl) return <img className="avatar" src={user.avatarDataUrl} alt="" />
  if (!user.name.trim()) {
    return (
      <span className="avatar icon-fallback">
        <UserRound size={16} />
      </span>
    )
  }
  const index = (hashName(user.name) % DEFAULT_AVATAR_COUNT) + 1
  return <img className="avatar" src={`/avatars/default-${String(index).padStart(2, '0')}.svg`} alt="" />
}

export function SetupCard({ children }: { children: ReactNode }) {
  return <div className="setup-card">{children}</div>
}

// Lightweight dialog: scrim click and Escape close it, clicks inside pass
// through. Used for the dashboard calendar day details and admin editors.
export function Modal({
  title,
  onClose,
  wide,
  children,
}: {
  title: string
  onClose: () => void
  wide?: boolean
  children: ReactNode
}) {
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        className={wide ? 'modal wide' : 'modal'}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onClick={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <h3>{title}</h3>
          <button className="icon-btn" onClick={onClose} aria-label="close">
            <X size={16} />
          </button>
        </div>
        <div className="modal-body">{children}</div>
      </div>
    </div>
  )
}

export function KeyboardHint({ text }: { text: string }) {
  return (
    <div className="keyboard-hint">
      <Keyboard size={15} /> {text}
    </div>
  )
}

export function ListSkeleton({ rows = 10 }: { rows?: number }) {
  return (
    <div className="table">
      {Array.from({ length: rows }, (_, i) => (
        <div className="concept-row skeleton-row" key={i} />
      ))}
    </div>
  )
}

export function SpinnerButton({
  busy,
  children,
  className = 'primary',
  ...rest
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { busy?: boolean; className?: string }) {
  return (
    <button className={className} disabled={busy || rest.disabled} {...rest}>
      {busy ? <Loader2 className="spin" size={16} /> : null}
      {children}
    </button>
  )
}

export function StatusPill({ status }: { status: string }) {
  const { t } = useSession()
  const tone = status === 'proficient' ? 'ok' : status === 'fuzzy' ? 'warn' : status === 'unknown' ? 'bad' : ''
  const label =
    status === 'proficient' ? t.proficient : status === 'fuzzy' ? t.fuzzy : status === 'unknown' ? t.unknown : t.unmarked
  return <span className={`pill ${tone}`}>{label}</span>
}

// TablePager is the shared footer pager for client-side table pagination
// (student lists, user lists). Place it as the last child of the table
// container so its top border reads as the table's footer row.
export function TablePager({
  page,
  pageCount,
  total,
  onChange,
}: {
  page: number
  pageCount: number
  total: number
  onChange: (page: number) => void
}) {
  const { t } = useSession()
  return (
    <div className="log-pager">
      <small className="muted">{t.totalRows.replace('{n}', String(total))}</small>
      <div className="pager-controls">
        <button className="secondary" disabled={page <= 1} onClick={() => onChange(page - 1)}>
          {t.prevPage}
        </button>
        <span className="pager-info">{t.pageOf.replace('{a}', String(page)).replace('{b}', String(pageCount))}</span>
        <button className="secondary" disabled={page >= pageCount} onClick={() => onChange(page + 1)}>
          {t.nextPage}
        </button>
      </div>
    </div>
  )
}
