import { useEffect, useRef, useState, type ReactNode } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { useBackendStatus, type BackendInfo } from '../hooks/useBackendStatus'
import { useSnackbar } from './SnackbarProvider'

function IconMenu() {
  return (
    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 6h18" />
      <path d="M3 12h18" />
      <path d="M3 18h18" />
    </svg>
  )
}

function IconKanban() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="7" height="18" rx="1" />
      <rect x="14" y="3" width="7" height="11" rx="1" />
    </svg>
  )
}

function IconLogistics() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1" y="7" width="14" height="10" rx="1" />
      <path d="M15 10h4l3 3v4h-7z" />
      <circle cx="6" cy="19" r="1.5" />
      <circle cx="17.5" cy="19" r="1.5" />
    </svg>
  )
}

function IconException() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z" />
      <path d="M12 9v4" />
      <path d="M12 17h.01" />
    </svg>
  )
}

function IconReports() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 3v18h18" />
      <path d="M7 15l4-5 3 3 5-6" />
    </svg>
  )
}

function IconHistory() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 12a9 9 0 1 0 3-6.7" />
      <path d="M3 4v5h5" />
      <path d="M12 7v5l3 3" />
    </svg>
  )
}

function IconAdmin() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.7 1.7 0 0 0-1.87-.34 1.7 1.7 0 0 0-1.04 1.56V21a2 2 0 0 1-4 0v-.09a1.7 1.7 0 0 0-1.04-1.56 1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.7 1.7 0 0 0 .34-1.87 1.7 1.7 0 0 0-1.56-1.04H3a2 2 0 0 1 0-4h.09a1.7 1.7 0 0 0 1.56-1.04 1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.7 1.7 0 0 0 1.87.34H9a1.7 1.7 0 0 0 1.04-1.56V3a2 2 0 0 1 4 0v.09a1.7 1.7 0 0 0 1.04 1.56 1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.7 1.7 0 0 0-.34 1.87V9a1.7 1.7 0 0 0 1.56 1.04H21a2 2 0 0 1 0 4h-.09a1.7 1.7 0 0 0-1.56 1.04z" />
    </svg>
  )
}

const NAV_ITEMS: { to: string; label: string; icon: ReactNode }[] = [
  { to: '/board', label: 'Kanban Board', icon: <IconKanban /> },
  { to: '/exceptions', label: 'Exceptions', icon: <IconException /> },
  { to: '/logistics', label: 'Logistics', icon: <IconLogistics /> },
  { to: '/reports', label: 'Reports', icon: <IconReports /> },
]

function initials(email: string | null) {
  if (!email) return '?'
  const name = email.split('@')[0]
  return name.slice(0, 2).toUpperCase()
}

function BackendStatusBadge({ status, version }: BackendInfo) {
  const showSnackbar = useSnackbar()
  const previous = useRef(status)

  useEffect(() => {
    if (previous.current === status) return
    if (previous.current === 'offline' && status === 'online') {
      showSnackbar('Backend connection restored.', 'success')
    } else if (status === 'offline') {
      showSnackbar('Lost connection to the backend. Changes may not save.', 'error')
    }
    previous.current = status
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status])

  const label = status === 'online' ? 'Online' : status === 'offline' ? 'Offline' : 'Checking…'
  const title = version ? `Backend is ${label.toLowerCase()} (${version})` : `Backend is ${label.toLowerCase()}`

  return (
    <span className={`backend-status backend-status-${status}`} title={title}>
      <span className="backend-status-dot" />
      <span className="backend-status-label">{label}</span>
    </span>
  )
}

export function Layout() {
  const { email, roles, logout } = useAuth()
  const backendInfo = useBackendStatus()
  const navigate = useNavigate()
  const location = useLocation()
  const [menuOpen, setMenuOpen] = useState(false)
  const navItems = roles.includes('admin')
    ? [
        ...NAV_ITEMS,
        { to: '/audit-log', label: 'Audit History', icon: <IconHistory /> },
        { to: '/admin', label: 'Admin', icon: <IconAdmin /> },
      ]
    : NAV_ITEMS

  useEffect(() => {
    setMenuOpen(false)
  }, [location.pathname])

  async function handleLogout() {
    await logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="app-shell">
      <header className="app-header">
        <button
          type="button"
          className="app-header-menu-btn"
          aria-label="Toggle navigation menu"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen((open) => !open)}
        >
          <IconMenu />
        </button>
        <div className="app-header-brand">
          <img className="app-header-logo" src="/AssetWatchLogo.png" alt="SBS AssetWatch" />
          <span className="app-header-title">SBS AssetWatch</span>
        </div>
        <div className="app-header-user">
          <BackendStatusBadge {...backendInfo} />
          <span className="app-header-avatar" title={email ?? undefined}>
            {initials(email)}
          </span>
          <div className="app-header-user-info">
            <span className="app-header-email">{email}</span>
            <span className="app-header-roles">{roles.join(', ') || 'no roles assigned'}</span>
          </div>
          <button type="button" className="app-header-signout" onClick={handleLogout}>
            Sign out
          </button>
        </div>
        {menuOpen && (
          <nav className="app-header-mobile-nav">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) => `app-sidebar-link${isActive ? ' app-sidebar-link-active' : ''}`}
              >
                <span className="app-sidebar-icon">{item.icon}</span>
                {item.label}
              </NavLink>
            ))}
          </nav>
        )}
      </header>

      <div className="app-body">
        <nav className="app-sidebar">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) => `app-sidebar-link${isActive ? ' app-sidebar-link-active' : ''}`}
            >
              <span className="app-sidebar-icon">{item.icon}</span>
              {item.label}
            </NavLink>
          ))}
        </nav>

        <main className="app-content">
          <Outlet />
        </main>
      </div>

      <footer className="app-footer">
        <span>SBS AssetWatch &mdash; Hardware Lifecycle Management</span>
        {backendInfo.version && <span className="app-footer-version">v{backendInfo.version}</span>}
      </footer>
    </div>
  )
}
