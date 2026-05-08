export default function DashboardPage() {
  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1
          className="text-2xl font-bold mb-1"
          style={{ color: 'var(--text-primary)' }}
        >
          Dashboard
        </h1>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Monitor your platform metrics.
        </p>
      </div>

      {/* Placeholder chart area */}
      <div
        className="rounded-xl p-6 flex items-center justify-center"
        style={{
          background: 'var(--bg-surface)',
          border: '1px solid var(--border)',
          minHeight: 240,
        }}
      >
        <div className="text-center flex flex-col items-center gap-3">
          <svg
            width="48"
            height="48"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            style={{ color: 'var(--text-muted)' }}
          >
            <line x1="18" y1="20" x2="18" y2="10" />
            <line x1="12" y1="20" x2="12" y2="4" />
            <line x1="6" y1="20" x2="6" y2="14" />
          </svg>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Charts coming soon
          </p>
        </div>
      </div>

      {/* Placeholder table */}
      <div
        className="rounded-xl overflow-hidden"
        style={{
          background: 'var(--bg-surface)',
          border: '1px solid var(--border)',
        }}
      >
        <div
          className="px-6 py-4"
          style={{ borderBottom: '1px solid var(--border)' }}
        >
          <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
            Recent Activity
          </h2>
        </div>
        <div className="divide-y" style={{ borderColor: 'var(--border)' }}>
          {['Login event', 'Config updated', 'New user registered', 'Service restarted'].map(
            (event, i) => (
              <div
                key={i}
                className="px-6 py-3 flex items-center justify-between"
              >
                <span className="text-sm" style={{ color: 'var(--text-primary)' }}>
                  {event}
                </span>
                <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  {i + 1}m ago
                </span>
              </div>
            )
          )}
        </div>
      </div>
    </div>
  )
}
