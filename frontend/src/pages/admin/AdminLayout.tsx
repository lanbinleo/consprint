import { NavLink, Outlet } from 'react-router'
import { useSession } from '../../hooks/session'

export function AdminLayout() {
  const { t, isAdmin } = useSession()
  const groups = [
    {
      label: t.adminTeach,
      items: [
        { to: '/admin/analytics', label: t.adminAnalytics },
        { to: '/admin/questions', label: t.adminQuestions },
        { to: '/admin/import', label: t.adminImport },
        { to: '/admin/sets', label: t.adminSets },
        { to: '/admin/notes', label: t.adminNotes },
        { to: '/admin/boards', label: t.adminBoards },
      ],
    },
    ...(isAdmin
      ? [
          {
            label: t.adminManage,
            items: [
              { to: '/admin/activity', label: t.adminActivity },
              { to: '/admin/users', label: t.adminUsers },
              { to: '/admin/content', label: t.adminContent },
            ],
          },
        ]
      : []),
  ]
  return (
    <section className="page">
      <div className="admin-layout">
        <aside className="admin-nav">
          {groups.map((group) => (
            <div key={group.label}>
              <div className="nav-group-label">{group.label}</div>
              {group.items.map((item) => (
                <NavLink key={item.to} to={item.to}>
                  {item.label}
                </NavLink>
              ))}
            </div>
          ))}
        </aside>
        <div className="admin-body">
          <Outlet />
        </div>
      </div>
    </section>
  )
}
