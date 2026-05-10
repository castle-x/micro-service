import { useEffect } from 'react'
import { Outlet } from 'react-router-dom'
import Sidebar from './Sidebar'
import Topbar from './Topbar'
import { useAuthStore } from '../../store/auth'
import { getUserMe } from '../../lib/api'

export default function AppLayout() {
  const { accessToken, user, setUser } = useAuthStore()

  useEffect(() => {
    if (accessToken && !user?.role) {
      getUserMe()
        .then((data) => setUser({
          userId: data.user_id,
          name: data.name || data.email,
          email: data.email,
          avatar: data.avatar_url || '',
          role: data.role || '',
        }))
        .catch(() => {/* token 失效由 axios 拦截器处理 */})
    }
  }, [accessToken])

  return (
    <div
      className="flex h-screen overflow-hidden"
      style={{ background: 'var(--bg-primary)' }}
    >
      <Sidebar />
      <div className="flex flex-col flex-1 min-w-0 overflow-hidden">
        <Topbar />
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
