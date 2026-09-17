import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { api } from '../../lib/api'
import { useSession } from '../../hooks/session'
import { Header } from '../../components/ui'
import type { Role, User } from '../../lib/types'

export function Users() {
  const { t, user: me } = useSession()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const { data: users = [] } = useQuery({
    queryKey: ['admin-users', search],
    queryFn: () => api.request<User[]>(`/api/admin/users?search=${encodeURIComponent(search)}`),
  })

  async function setRole(target: User, role: Role) {
    await api.request(`/api/admin/users/${target.id}`, { method: 'PATCH', body: JSON.stringify({ role }) })
    void queryClient.invalidateQueries({ queryKey: ['admin-users'] })
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
          <span>Provider</span>
          <span />
        </div>
        {users.map((user) => (
          <div className="student-row" key={user.id}>
            <span>
              <strong>{user.name}</strong>
              <small>{user.email}</small>
            </span>
            <select value={user.role} disabled={user.id === me?.id} onChange={(e) => void setRole(user, e.target.value as Role)}>
              <option value="student">{t.roleStudent}</option>
              <option value="teacher">{t.roleTeacher}</option>
              <option value="admin">{t.roleAdmin}</option>
            </select>
            <span>{user.provider === 'entra' ? t.providerEntra : t.providerLocal}</span>
            <span />
          </div>
        ))}
      </div>
    </div>
  )
}
