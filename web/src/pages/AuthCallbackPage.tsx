import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { getUserMe } from '../lib/api'

export default function AuthCallbackPage() {
  const navigate = useNavigate()
  const { setAuth, setUser } = useAuthStore()

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const accessToken = params.get('access_token')
    const refreshToken = params.get('refresh_token')
    const userId = params.get('user_id')

    if (!accessToken || !refreshToken || !userId) {
      navigate('/login')
      return
    }

    setAuth({ accessToken, refreshToken, userId })

    getUserMe()
      .then((data) => {
        setUser({
          userId: data.user_id,
          name: data.name,
          email: data.email,
          avatar: data.avatar_url,
        })
        navigate('/')
      })
      .catch(() => {
        navigate('/')
      })
  }, [])

  return (
    <div
      className="min-h-screen flex items-center justify-center"
      style={{ background: 'var(--bg-primary)' }}
    >
      <div className="flex flex-col items-center gap-4">
        <div
          className="w-10 h-10 rounded-full border-2 border-t-transparent animate-spin"
          style={{ borderColor: 'var(--accent)', borderTopColor: 'transparent' }}
        />
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Signing you in...
        </p>
      </div>
    </div>
  )
}
