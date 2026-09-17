import { useState } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router'
import {
  BookOpen,
  ChartNoAxesColumn,
  ClipboardList,
  Gauge,
  Languages,
  LogOut,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  ShieldCheck,
  Sun,
  Layers,
} from 'lucide-react'
import { useSession } from '../hooks/session'
import { Avatar } from './ui'

export function Shell() {
  const { user, tenant, t, lang, setLang, theme, setTheme, isStaff, logout } = useSession()
  const [collapsed, setCollapsed] = useState(false)
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const navigate = useNavigate()

  const links = [
    { to: '/', icon: <Gauge size={17} />, label: t.dashboard, end: true },
    { to: '/learn', icon: <BookOpen size={17} />, label: t.learn, end: false },
    { to: '/flashcards', icon: <Layers size={17} />, label: t.flashcards, end: false },
    { to: '/practice', icon: <ClipboardList size={17} />, label: t.practice, end: false },
    { to: '/wrongbook', icon: <ChartNoAxesColumn size={17} />, label: t.wrongbook, end: false },
  ]

  return (
    <div className={`app-shell ${collapsed ? 'nav-collapsed' : ''}`}>
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">AP</div>
          {!collapsed && (
            <div>
              <strong>Psych Hub</strong>
              <span>{tenant?.name ?? 'AP Psychology'}</span>
            </div>
          )}
        </div>
        <button className="icon-line" onClick={() => setCollapsed((value) => !value)} title={t.collapse}>
          {collapsed ? <PanelLeftOpen size={17} /> : <PanelLeftClose size={17} />}
          {!collapsed && t.collapse}
        </button>
        <nav>
          {links.map((link) => (
            <NavLink key={link.to} to={link.to} end={link.end} title={link.label}>
              {link.icon}
              {!collapsed && link.label}
            </NavLink>
          ))}
          {isStaff && (
            <NavLink to="/admin/analytics" title={t.admin}>
              <ShieldCheck size={17} />
              {!collapsed && t.admin}
            </NavLink>
          )}
        </nav>
        <div className="sidebar-tools">
          <button className="icon-line" onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')} title={t.theme}>
            {theme === 'light' ? <Moon size={16} /> : <Sun size={16} />}
            {!collapsed && (theme === 'light' ? t.dark : t.light)}
          </button>
          <button className="icon-line" onClick={() => setLang(lang === 'en' ? 'zh' : 'en')} title={t.language}>
            <Languages size={16} />
            {!collapsed && (lang === 'en' ? '中文' : 'English')}
          </button>
          <div className="user-dock">
            <button className="user-chip" onClick={() => setUserMenuOpen((value) => !value)} title={t.settings}>
              {user && <Avatar user={user} />}
              {!collapsed && user && (
                <span>
                  <strong>{user.name}</strong>
                  <small>{user.role}</small>
                </span>
              )}
            </button>
            {userMenuOpen && (
              <div className="user-menu">
                <button
                  onClick={() => {
                    navigate('/profile')
                    setUserMenuOpen(false)
                  }}
                >
                  <Settings size={16} /> {t.settings}
                </button>
                <button
                  onClick={() => {
                    logout()
                    navigate('/login')
                  }}
                >
                  <LogOut size={16} /> {t.logout}
                </button>
              </div>
            )}
          </div>
        </div>
      </aside>
      <main className="workspace">
        <div className="view-frame">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
