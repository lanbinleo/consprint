import { NavLink, Outlet } from 'react-router'
import { useSession } from '../../hooks/session'

export function AdminLayout() {
  const { t, isAdmin } = useSession()
  const items = [
    { to: '/admin/analytics', label: t.adminAnalytics },
    { to: '/admin/questions', label: t.adminQuestions, end: false },
    { to: '/admin/import', label: t.adminImport },
    { to: '/admin/sets', label: t.adminSets },
    ...(isAdmin
      ? [
          { to: '/admin/users', label: t.adminUsers },
          { to: '/admin/content', label: t.adminContent },
        ]
      : []),
  ]
  return (
    <section className="page">
      <div className="admin-layout">
        <aside className="admin-nav">
          {items.map((item) => (
            <NavLink key={item.to} to={item.to} end={'end' in item && item.end ? true : undefined}>
              {item.label}
            </NavLink>
          ))}
        </aside>
        <div className="admin-body">
          <Outlet />
        </div>
      </div>
    </section>
  )
}
