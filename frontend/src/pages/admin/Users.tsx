import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { KeyRound, PenLine, RotateCcw, Search, X } from 'lucide-react'
import { api } from '../../lib/api'
import { useSession } from '../../hooks/session'
import { Avatar, Header, SpinnerButton, TablePager } from '../../components/ui'
import { formatDateTime } from '../../lib/format'
import type { Role, User } from '../../lib/types'

const USER_PAGE_SIZE = 15

export function Users() {
  const { t, user: me } = useSession()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const { data } = useQuery({
    queryKey: ['admin-users', search],
    queryFn: async () => (await api.request<User[] | null>(`/api/admin/users?search=${encodeURIComponent(search)}`)) ?? [],
  })
  const users = data ?? []
  const [page, setPage] = useState(1)
  const [actionError, setActionError] = useState('')
  const [editing, setEditing] = useState<User | null>(null)
  const pageCount = Math.max(1, Math.ceil(users.length / USER_PAGE_SIZE))
  const safePage = Math.min(page, pageCount)
  const pagedUsers = users.slice((safePage - 1) * USER_PAGE_SIZE, safePage * USER_PAGE_SIZE)

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['admin-users'] })
  }

  function roleLabel(role: string) {
    if (role === 'admin') return t.roleAdmin
    if (role === 'teacher') return t.roleTeacher
    return t.roleStudent
  }

  return (
    <div>
      <Header eyebrow={t.admin} title={t.usersTitle} />
      <label className="search">
        <Search size={16} />
        <input placeholder={t.search} value={search} onChange={(e) => { setSearch(e.target.value); setPage(1) }} />
      </label>
      <div className="student-table user-table">
        <div className="student-row head">
          <span>{t.student}</span>
          <span>{t.role}</span>
          <span>{t.provider}</span>
          <span>{t.recentLogin}</span>
          <span>{t.recentSeen}</span>
          <span />
        </div>
        {pagedUsers.map((user) => (
          <div className="student-row" key={user.id}>
            <span className="student-name">
              <Avatar user={user} />
              <span className="student-id">
                <strong>{user.name}</strong>
                <small>{user.email}</small>
              </span>
            </span>
            <span>{roleLabel(user.role)}</span>
            <span>{user.provider === 'entra' ? t.providerEntra : t.providerLocal}</span>
            <span>{user.lastLoginAt ? formatDateTime(user.lastLoginAt) : '—'}</span>
            <span>{user.lastSeenAt ? formatDateTime(user.lastSeenAt) : '—'}</span>
            <span className="row-actions">
              <button className="icon-line" title={t.editUser} onClick={() => setEditing(user)}>
                <PenLine size={15} />
              </button>
            </span>
          </div>
        ))}
        {users.length > USER_PAGE_SIZE && (
          <TablePager page={safePage} pageCount={pageCount} total={users.length} onChange={setPage} />
        )}
      </div>
      {actionError && <div className="error" style={{ marginTop: 12 }}>{actionError}</div>}
      {editing && (
        <UserEditModal
          user={editing}
          selfEdit={editing.id === me?.id}
          onClose={() => setEditing(null)}
          refresh={refresh}
          onError={setActionError}
        />
      )}
    </div>
  )
}

// Editing lives in a modal: identity fields up top, support actions (avatar
// and password reset) below the fold of the same dialog.
function UserEditModal({
  user,
  selfEdit,
  onClose,
  refresh,
  onError,
}: {
  user: User
  selfEdit: boolean
  onClose: () => void
  refresh: () => void
  onError: (message: string) => void
}) {
  const { t } = useSession()
  const [name, setName] = useState(user.name)
  const [role, setRole] = useState<Role>(user.role)
  const [avatarDataUrl, setAvatarDataUrl] = useState(user.avatarDataUrl)
  const [newPassword, setNewPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [saved, setSaved] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')

  async function save() {
    setBusy(true)
    setError('')
    try {
      await api.request<User>(`/api/admin/users/${user.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ name, role }),
      })
      refresh()
      setSaved(true)
      setTimeout(() => setSaved(false), 1200)
      onError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  async function resetAvatar() {
    setError('')
    try {
      await api.request(`/api/admin/users/${user.id}`, { method: 'PATCH', body: JSON.stringify({ avatarReset: true }) })
      setAvatarDataUrl(undefined)
      refresh()
      setNotice(t.avatarResetDone)
      setTimeout(() => setNotice(''), 1500)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  async function resetPassword() {
    setError('')
    if (newPassword.length < 8) {
      setError(t.passwordTooShort)
      return
    }
    try {
      await api.request(`/api/admin/users/${user.id}/password`, {
        method: 'PATCH',
        body: JSON.stringify({ newPassword }),
      })
      setNewPassword('')
      setNotice(t.passwordResetDone)
      setTimeout(() => setNotice(''), 1500)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" role="dialog" aria-modal="true" onClick={(event) => event.stopPropagation()}>
        <div className="modal-head">
          <h3>{t.editUser}</h3>
          <button className="icon-line" title={t.cancel} onClick={onClose}>
            <X size={16} />
          </button>
        </div>
        <div className="user-edit-head">
          <Avatar user={{ name: user.name, avatarDataUrl }} />
          <div>
            <strong>{user.name}</strong>
            <small>{user.email}</small>
          </div>
        </div>
        <label>
          {t.name}
          <input value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <label>
          {t.role}
          <select value={role} disabled={selfEdit} onChange={(e) => setRole(e.target.value as Role)} className={selfEdit ? 'disabled-field' : ''}>
            <option value="student">{t.roleStudent}</option>
            <option value="teacher">{t.roleTeacher}</option>
            <option value="admin">{t.roleAdmin}</option>
          </select>
          {selfEdit && <small className="field-hint">{t.cannotEditSelfRole}</small>}
        </label>
        {error && <div className="error">{error}</div>}
        <SpinnerButton className="primary" busy={busy} onClick={() => void save()}>
          {saved ? t.saved : t.save}
        </SpinnerButton>

        <div className="modal-section">
          <button className="secondary" onClick={() => void resetAvatar()}>
            <RotateCcw size={16} /> {t.resetAvatar}
          </button>
        </div>
        <div className="modal-section">
          <label>
            {t.newPassword}
            <input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder={t.passwordResetPlaceholder}
              autoComplete="new-password"
            />
          </label>
          <button className="secondary" disabled={!newPassword} onClick={() => void resetPassword()}>
            <KeyRound size={16} /> {t.resetPassword}
          </button>
        </div>
        {notice && <div className="success-note">{notice}</div>}
      </div>
    </div>
  )
}
