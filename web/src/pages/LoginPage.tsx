import { useState, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { getGoogleAuthUrl, getAlipayAuthUrl } from '../lib/api'

export default function LoginPage() {
  const [loadingGoogle, setLoadingGoogle] = useState(false)
  const [loadingAlipay, setLoadingAlipay] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [searchParams] = useSearchParams()

  // 读取回调带回的 error 参数
  useEffect(() => {
    const urlError = searchParams.get('error')
    if (urlError) {
      setError(decodeURIComponent(urlError))
    }
  }, [])

  const handleGoogleLogin = async () => {
    setLoadingGoogle(true)
    setError(null)
    try {
      const url = await getGoogleAuthUrl()
      window.location.href = url
    } catch {
      setError('Failed to initiate Google login. Please try again.')
      setLoadingGoogle(false)
    }
  }

  const handleAlipayLogin = async () => {
    setLoadingAlipay(true)
    setError(null)
    try {
      const url = await getAlipayAuthUrl()
      window.location.href = url
    } catch {
      setError('Failed to initiate Alipay login. Please try again.')
      setLoadingAlipay(false)
    }
  }

  const disabled = loadingGoogle || loadingAlipay

  return (
    <div className="min-h-screen flex items-center justify-center p-4" style={{ background: 'var(--bg-primary)' }}>
      <div
        className="w-full max-w-sm rounded-2xl p-8 flex flex-col items-center gap-6"
        style={{ background: 'var(--bg-surface)', border: '1px solid var(--border)', boxShadow: '0 24px 64px rgba(0,0,0,0.4)' }}
      >
        {/* Logo */}
        <div className="w-12 h-12 rounded-xl flex items-center justify-center"
          style={{ background: 'var(--accent-glow)', border: '1px solid var(--accent)' }}>
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor"
            strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ color: 'var(--accent)' }}>
            <rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" />
            <rect x="3" y="14" width="7" height="7" /><rect x="14" y="14" width="7" height="7" />
          </svg>
        </div>

        {/* Title */}
        <div className="text-center">
          <h1 className="text-2xl font-bold mb-1" style={{ color: 'var(--text-primary)' }}>Platform Admin</h1>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Sign in to continue</p>
        </div>

        {/* Error */}
        {error && (
          <div className="w-full text-sm px-4 py-3 rounded-lg"
            style={{ background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', color: 'var(--danger)' }}>
            {error}
          </div>
        )}

        {/* Login buttons */}
        <div className="w-full flex flex-col gap-3">
          {/* Google */}
          <button onClick={handleGoogleLogin} disabled={disabled}
            className="w-full flex items-center justify-center gap-3 px-4 py-2.5 rounded-lg text-sm font-medium transition-all"
            style={{ background: '#ffffff', color: '#1a1a1a', border: '1px solid #e0e0e0', opacity: disabled ? 0.7 : 1, cursor: disabled ? 'not-allowed' : 'pointer' }}
            onMouseEnter={(e) => { if (!disabled) e.currentTarget.style.boxShadow = '0 2px 8px rgba(0,0,0,0.15)' }}
            onMouseLeave={(e) => { e.currentTarget.style.boxShadow = 'none' }}
          >
            <svg width="18" height="18" viewBox="0 0 24 24">
              <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4" />
              <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
              <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05" />
              <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
            </svg>
            {loadingGoogle ? 'Redirecting...' : 'Continue with Google'}
          </button>

          {/* Alipay */}
          <button onClick={handleAlipayLogin} disabled={disabled}
            className="w-full flex items-center justify-center gap-3 px-4 py-2.5 rounded-lg text-sm font-medium transition-all"
            style={{ background: '#1677FF', color: '#ffffff', border: 'none', opacity: disabled ? 0.7 : 1, cursor: disabled ? 'not-allowed' : 'pointer' }}
            onMouseEnter={(e) => { if (!disabled) e.currentTarget.style.background = '#0e6ae0' }}
            onMouseLeave={(e) => { e.currentTarget.style.background = '#1677FF' }}
          >
            <svg width="20" height="20" viewBox="0 0 48 48" fill="none">
              <rect width="48" height="48" rx="8" fill="white"/>
              <path d="M24 6C14.06 6 6 14.06 6 24s8.06 18 18 18 18-8.06 18-18S33.94 6 24 6zm8.3 25.4c-2.1-.64-3.82-1.18-5.3-1.62 1.04-1.76 1.86-3.76 2.38-5.98h-6.2V22h7v-1.8H22.4v-2.4h-2.2v2.4H13v1.8h7.18v1.8h-5.96v1.8h10.04c-.38 1.46-.9 2.82-1.58 4.04-2.6-.64-4.66-.96-6.28-.96-3.88 0-6.4 1.72-6.4 4.34 0 2.44 2.18 4.18 5.28 4.18 3.46 0 6.46-1.94 8.2-5.14 2.34.82 5.08 1.86 8.06 3.12l.76-3.08z" fill="#1677FF"/>
              <path d="M15.18 32.82c0-1.38 1.42-2.32 3.62-2.32 1.28 0 2.88.28 5 .86-1.36 2.34-3.5 3.78-5.7 3.78-1.76 0-2.92-.92-2.92-2.32z" fill="#1677FF"/>
            </svg>
            {loadingAlipay ? 'Redirecting...' : 'Continue with Alipay'}
          </button>
        </div>

        <p className="text-xs text-center" style={{ color: 'var(--text-muted)' }}>
          支付宝登录当前使用沙箱环境
        </p>
      </div>
    </div>
  )
}
