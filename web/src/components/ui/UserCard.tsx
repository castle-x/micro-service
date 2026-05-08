import { useState, useRef, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../../store/auth'
import { postLogout } from '../../lib/api'

interface UserCardProps {
  collapsed: boolean
}

export default function UserCard({ collapsed }: UserCardProps) {
  const { user, refreshToken, logout } = useAuthStore()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  const handleLogout = async () => {
    try {
      if (refreshToken) {
        await postLogout(refreshToken)
      }
    } catch {
      // ignore
    }
    logout()
    navigate('/login')
  }

  if (!user) return null

  return (
    <div ref={ref} className="relative">
      {/* Popover menu */}
      {open && (
        <div
          className="absolute bottom-full left-0 mb-2 w-56 rounded-lg border shadow-xl z-50"
          style={{
            background: 'var(--bg-surface)',
            borderColor: 'var(--border)',
            boxShadow: '0 8px 32px rgba(0,0,0,0.4)',
          }}
        >
          {/* User info header */}
          <div className="px-4 py-3" style={{ borderBottom: '1px solid var(--border)' }}>
            <p className="text-sm font-semibold truncate" style={{ color: 'var(--text-primary)' }}>
              {user.name}
            </p>
            <p className="text-xs truncate" style={{ color: 'var(--text-muted)' }}>
              {user.email}
            </p>
          </div>
          {/* Actions */}
          <div className="py-1">
            <button
              onClick={handleLogout}
              className="w-full text-left flex items-center gap-2 px-4 py-2 text-sm transition-colors"
              style={{ color: 'var(--danger)' }}
              onMouseEnter={(e) => (e.currentTarget.style.background = 'var(--bg-hover)')}
              onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
            >
              <span>🚪</span>
              <span>Logout</span>
            </button>
          </div>
        </div>
      )}

      {/* Card trigger */}
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-3 px-3 py-2 rounded-lg transition-colors"
        style={{ background: open ? 'var(--bg-hover)' : 'transparent' }}
        onMouseEnter={(e) => (e.currentTarget.style.background = 'var(--bg-hover)')}
        onMouseLeave={(e) =>
          !open && (e.currentTarget.style.background = 'transparent')
        }
      >
        {user.avatar ? (
          <img
            src={user.avatar}
            alt={user.name}
            className="rounded-full flex-shrink-0"
            style={{ width: 32, height: 32 }}
            referrerPolicy="no-referrer"
          />
        ) : (
          <div
            className="rounded-full flex-shrink-0 flex items-center justify-center text-sm font-bold"
            style={{
              width: 32,
              height: 32,
              background: 'var(--accent)',
              color: '#fff',
            }}
          >
            {user.name?.[0]?.toUpperCase() ?? 'U'}
          </div>
        )}
        {!collapsed && (
          <div className="flex-1 min-w-0 text-left">
            <p
              className="text-sm font-semibold truncate"
              style={{ color: 'var(--text-primary)' }}
            >
              {user.name}
            </p>
            <p className="text-xs truncate" style={{ color: 'var(--text-muted)' }}>
              {user.email}
            </p>
          </div>
        )}
      </button>
    </div>
  )
}
