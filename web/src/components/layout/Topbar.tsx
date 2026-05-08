import { useLocation } from 'react-router-dom'
import ThemeToggle from '../ui/ThemeToggle'

function getBreadcrumb(pathname: string): string[] {
  if (pathname === '/') return ['Home']
  return pathname
    .split('/')
    .filter(Boolean)
    .map((seg) => seg.charAt(0).toUpperCase() + seg.slice(1))
}

export default function Topbar() {
  const location = useLocation()
  const crumbs = getBreadcrumb(location.pathname)

  return (
    <header
      className="flex items-center justify-between h-14 px-6 flex-shrink-0"
      style={{
        background: 'var(--bg-secondary)',
        borderBottom: '1px solid var(--border)',
      }}
    >
      {/* Breadcrumb */}
      <nav className="flex items-center gap-2 text-sm">
        {crumbs.map((crumb, i) => (
          <span key={i} className="flex items-center gap-2">
            {i > 0 && (
              <svg
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                style={{ color: 'var(--text-muted)' }}
              >
                <polyline points="9 18 15 12 9 6" />
              </svg>
            )}
            <span
              style={{
                color: i === crumbs.length - 1 ? 'var(--text-primary)' : 'var(--text-muted)',
                fontWeight: i === crumbs.length - 1 ? 500 : 400,
              }}
            >
              {crumb}
            </span>
          </span>
        ))}
      </nav>

      {/* Right side */}
      <div className="flex items-center gap-2">
        <ThemeToggle />
      </div>
    </header>
  )
}
