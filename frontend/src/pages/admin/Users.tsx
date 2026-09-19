import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, PenLine, Search, X } from 'lucide-react'
import { api } from '../../lib/api'
import { useSession } from '../../hooks/session'
import { Header } from '../../components/ui'
import type { Role, User } from '../../lib/types'

export function Users() {
  const { t, user: me } = useSession()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const { data } = useQuery({
    queryKey: ['admin-users', search],
    queryFn: async () => (await api.request<User[] | null>(`/api/admin/users?search=${encodeURIComponent(search)}`)) ?? [],
  })
  const users = data ?? []
  const [actionError, setActionError] = useState('')
  const [renamingId, setRenamingId] = useState<string | null>(null)
  const [renameValue, setRenameValue] = useState('')

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['admin-users'] })
  }

  async function setRole(target: User, role: Role) {
    setActionError('')
    try {
      await api.request(`/api/admin/users/${target.id}`, { method: 'PATCH', body: JSON.stringify({ role }) })
      refresh()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t.errorGeneric)
      refresh()
    }
  }

  async function saveName(target: User) {
    const name = renameValue.trim()
    if (!name) return
    setActionError('')
    try {
      await api.request(`/api/admin/users/${target.id}`, { method: 'PATCH', body: JSON.stringify({ name }) })
      setRenamingId(null)
      refresh()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  function startRename(target: User) {
    setRenamingId(target.id)
    setRenameValue(target.name)
  }

  return (
    <div>
      <Header eyebrow={t.admin} title={t.usersTitle} />
      <label className="search">
        <Search size={16} />
        <input placeholder={t.search} value={search} onChange={(e) => setSearch(e.target.value)} />
      </label>
      <div className="student-table">
        <div className="student-row head">
          <span>{t.student}</span>
          <span>{t.role}</span>
          <span>{t.provider}</span>
          <span />
        </div>
        {users.map((user) => (
          <div className="student-row" key={user.id}>
            <span className="student-name">
              {renamingId === user.id ? (
                <>
                  <input
                    autoFocus
                    value={renameValue}
                    onChange={(e) => setRenameValue(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') void saveName(user)
                      if (e.key === 'Escape') setRenamingId(null)
                    }}
                  />
                  <button className="icon-line" title={t.save} onClick={() => void saveName(user)}>
                    <Check size={15} />
                  </button>
                  <button className="icon-line" title={t.cancel} onClick={() => setRenamingId(null)}>
                    <X size={15} />
                  </button>
                </>
              ) : (
                <>
                  <strong>{user.name}</strong>
                  <small>{user.email}</small>
                </>
              )}
            </span>
            <select value={user.role} disabled={user.id === me?.id} onChange={(e) => void setRole(user, e.target.value as Role)}>
              <option value="student">{t.roleStudent}</option>
              <option value="teacher">{t.roleTeacher}</option>
              <option value="admin">{t.roleAdmin}</option>
            </select>
            <span>{user.provider === 'entra' ? t.providerEntra : t.providerLocal}</span>
            <span className="row-actions">
              <button className="icon-line" title={t.renameUser} onClick={() => startRename(user)}>
                <PenLine size={15} />
              </button>
            </span>
          </div>
        ))}
      </div>
      {actionError && <div className="error" style={{ marginTop: 12 }}>{actionError}</div>}
    </div>
  )
}
