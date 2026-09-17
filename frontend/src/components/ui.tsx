import type { ReactNode } from 'react'
import { Keyboard, Loader2, User } from 'lucide-react'
import { initials } from '../lib/format'

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

export function Avatar({ user }: { user: { name: string; avatarDataUrl?: string } }) {
  if (user.avatarDataUrl) return <img className="avatar" src={user.avatarDataUrl} alt="" />
  return (
    <span className="avatar">
      <User size={17} />
      {initials(user.name)}
    </span>
  )
}

export function SetupCard({ children }: { children: ReactNode }) {
  return <div className="setup-card">{children}</div>
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
  const tone = status === 'proficient' ? 'ok' : status === 'fuzzy' ? 'warn' : status === 'unknown' ? 'bad' : ''
  return <span className={`pill ${tone}`}>{status || 'unmarked'}</span>
}
