import { useEffect, useRef, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import {
  BookOpen,
  ChartNoAxesColumn,
  ChevronUp,
  ClipboardList,
  Gauge,
  Languages,
  LogOut,
  Menu,
  Moon,
  NotebookPen,
  PanelLeftClose,
  PanelLeftOpen,
  PenLine,
  Settings,
  ShieldCheck,
  Sun,
  X,
  Layers,
} from 'lucide-react'
import { useSession } from '../hooks/session'
import { Avatar } from './ui'
import { Logo } from './Logo'
import type { Role } from '../lib/types'

type NavItem = { to: string; icon: React.ReactNode; label: string; end: boolean }
type NavGroup = { label?: string; items: NavItem[] }

export function Shell() {
  const { user, tenant, t, lang, setLang, theme, setTheme, isStaff, logout } = useSession()
  const [collapsed, setCollapsed] = useState(false)
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const [menuPos, setMenuPos] = useState<{ left: number; bottom: number } | null>(null)
  const [tip, setTip] = useState<{ text: string; x: number; y: number } | null>(null)
  const navigate = useNavigate()
  const location = useLocation()
  const queryClient = useQueryClient()
  const dockRef = useRef<HTMLDivElement>(null)
  const chipRef = useRef<HTMLButtonElement>(null)

  const roleLabel = (role: Role | undefined) =>
    role === 'admin' ? t.roleAdmin : role === 'teacher' ? t.roleTeacher : t.roleStudent

  // Route changes always close the mobile drawer.
  useEffect(() => {
    setMobileNavOpen(false)
  }, [location.pathname])

  useEffect(() => {
    if (!mobileNavOpen) return
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') setMobileNavOpen(false)
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [mobileNavOpen])

  useEffect(() => {
    if (!userMenuOpen) return
    function onPointerDown(event: MouseEvent) {
      if (dockRef.current && !dockRef.current.contains(event.target as Node)) setUserMenuOpen(false)
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') setUserMenuOpen(false)
    }
    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [userMenuOpen])

  // The user menu is anchored with fixed coordinates instead of absolute
  // positioning inside the sidebar: the sidebar clips overflow (especially
  // when collapsed to icons), which would cut the popup off.
  function toggleUserMenu() {
    const next = !userMenuOpen
    setUserMenuOpen(next)
    if (next && chipRef.current) {
      const rect = chipRef.current.getBoundingClientRect()
      setMenuPos({ left: rect.left, bottom: window.innerHeight - rect.top + 8 })
    }
  }

  function showTip(event: React.MouseEvent<HTMLButtonElement> | React.FocusEvent<HTMLButtonElement>) {
    const rect = event.currentTarget.getBoundingClientRect()
    setTip({ text: event.currentTarget.dataset.tip ?? '', x: rect.left + rect.width / 2, y: rect.top - 8 })
  }
  const hideTip = () => setTip(null)

  const groups: NavGroup[] = [
    {
      label: t.homeGroup,
      items: [{ to: '/', icon: <Gauge size={17} />, label: t.dashboard, end: true }],
    },
    {
      label: t.knowledge,
      items: [
        { to: '/terms', icon: <BookOpen size={17} />, label: t.glossary, end: false },
        { to: '/flashcards', icon: <Layers size={17} />, label: t.flashcards, end: false },
        { to: '/notes', icon: <NotebookPen size={17} />, label: t.notes, end: false },
      ],
    },
    {
      label: t.practice,
      items: [
        { to: '/practice', icon: <ClipboardList size={17} />, label: t.practiceSets, end: true },
        { to: '/practice/writing', icon: <PenLine size={17} />, label: t.writing, end: false },
        { to: '/wrongbook', icon: <ChartNoAxesColumn size={17} />, label: t.wrongbook, end: false },
      ],
    },
  ]
  if (isStaff) {
    // One entry for the whole admin area: prefix matching keeps it
    // highlighted on every /admin/* sub-page, not just the analytics landing.
    groups.push({
      label: t.admin,
      items: [{ to: '/admin', icon: <ShieldCheck size={17} />, label: t.admin, end: false }],
    })
  }

  const quickButtons = [
    {
      key: 'collapse',
      tip: collapsed ? t.expand : t.collapse,
      icon: collapsed ? <PanelLeftOpen size={17} /> : <PanelLeftClose size={17} />,
      onClick: () => setCollapsed((value) => !value),
      desktopOnly: true,
    },
    {
      key: 'theme',
      tip: theme === 'light' ? t.dark : t.light,
      icon: theme === 'light' ? <Moon size={17} /> : <Sun size={17} />,
      onClick: () => setTheme(theme === 'light' ? 'dark' : 'light'),
    },
    {
      key: 'lang',
      // Language toggles read best in the language being switched to.
      tip: lang === 'en' ? '切换到中文' : 'Switch to English',
      icon: <Languages size={17} />,
      onClick: () => setLang(lang === 'en' ? 'zh' : 'en'),
    },
  ]

  return (
    <div className={`app-shell ${collapsed ? 'nav-collapsed' : ''} ${mobileNavOpen ? 'nav-open' : ''}`}>
      <header className="mobile-topbar">
        <button
          className="icon-btn"
          aria-label={mobileNavOpen ? t.collapse : t.expand}
          onClick={() => setMobileNavOpen((value) => !value)}
        >
          {mobileNavOpen ? <X size={19} /> : <Menu size={19} />}
        </button>
        <div className="brand">
          <div className="brand-mark">
            <Logo size={28} />
          </div>
          <div>
            <strong>Psych Hub</strong>
            <span>{tenant?.name ?? 'AP Psychology'}</span>
          </div>
        </div>
      </header>
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">
            <Logo size={36} />
          </div>
          {!collapsed && (
            <div>
              <strong>Psych Hub</strong>
              <span>{tenant?.name ?? 'AP Psychology'}</span>
            </div>
          )}
        </div>
        <nav>
          {groups.map((group, i) => (
            <div key={group.label ?? i}>
              {group.label && <div className="nav-group-label">{group.label}</div>}
              {group.items.map((link) => (
                <NavLink key={link.to} to={link.to} end={link.end} title={link.label} className={({ isActive }) => (isActive ? 'active' : '')}>
                  {link.icon}
                  {!collapsed && link.label}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
        <div className="sidebar-tools">
          <div className="sidebar-quick">
            {quickButtons.map((button) => (
              <button
                key={button.key}
                className={`icon-btn ${button.desktopOnly ? 'desktop-only' : ''}`}
                data-tip={button.tip}
                aria-label={button.tip}
                onClick={button.onClick}
                onMouseEnter={showTip}
                onFocus={showTip}
                onMouseLeave={hideTip}
                onBlur={hideTip}
              >
                {button.icon}
              </button>
            ))}
          </div>
          <div className="user-dock" ref={dockRef}>
            <button
              ref={chipRef}
              className={`user-chip ${userMenuOpen ? 'open' : ''}`}
              onClick={toggleUserMenu}
              aria-haspopup="menu"
              aria-expanded={userMenuOpen}
            >
              {user && <Avatar user={user} />}
              {!collapsed && user && (
                <span className="user-chip-text">
                  <strong>{user.name}</strong>
                  <small>{roleLabel(user.role)}</small>
                </span>
              )}
              {!collapsed && <ChevronUp size={15} className={`user-chev ${userMenuOpen ? 'open' : ''}`} />}
            </button>
            {userMenuOpen && user && menuPos && (
              <div className="user-menu" role="menu" style={{ left: menuPos.left, bottom: menuPos.bottom }}>
                <button
                  role="menuitem"
                  onClick={() => {
                    navigate('/profile')
                    setUserMenuOpen(false)
                  }}
                >
                  <Settings size={16} /> {t.settings}
                </button>
                <button
                  role="menuitem"
                  className="logout"
                  onClick={() => {
                    setUserMenuOpen(false)
                    logout()
                    queryClient.clear()
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
      <div className="nav-scrim" onClick={() => setMobileNavOpen(false)} aria-hidden="true" />
      <main className="workspace">
        <div className="view-frame">
          <Outlet />
        </div>
      </main>
      {tip && (
        <div className="floating-tip" style={{ left: tip.x, top: tip.y }} role="tooltip">
          {tip.text}
        </div>
      )}
    </div>
  )
}
