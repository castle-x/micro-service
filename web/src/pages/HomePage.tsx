import { useAuthStore } from '../store/auth'

export default function HomePage() {
  const user = useAuthStore((s) => s.user)

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1
          className="text-2xl font-bold mb-1"
          style={{ color: 'var(--text-primary)' }}
        >
          Welcome back{user?.name ? `, ${user.name}` : ''}
        </h1>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Here's an overview of your platform.
        </p>
      </div>

      {/* Placeholder cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {['Users', 'Services', 'Requests'].map((label, i) => (
          <div
            key={label}
            className="rounded-xl p-6 flex flex-col gap-2"
            style={{
              background: 'var(--bg-surface)',
              border: '1px solid var(--border)',
            }}
          >
            <p className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
              {label}
            </p>
            <p className="text-3xl font-bold" style={{ color: 'var(--text-primary)' }}>
              {(i + 1) * 42}
            </p>
            <p className="text-xs" style={{ color: 'var(--success)' }}>
              ↑ {(i + 1) * 3}% this week
            </p>
          </div>
        ))}
      </div>
    </div>
  )
}
