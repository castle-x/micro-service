import { useState, type FormEvent } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { getGoogleAuthUrl, getAlipayAuthUrl, loginByPassword, register } from '../lib/api'
import { getErrorMessage } from '../lib/error'
import { useAuthStore } from '../store/auth'

type Mode = 'login' | 'register'

// 小眼睛图标
function EyeIcon({ open }: { open: boolean }) {
  return open ? (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  ) : (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
      <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
      <line x1="1" y1="1" x2="23" y2="23" />
    </svg>
  )
}

// 带小眼睛的密码输入框
function PasswordInput({
  placeholder,
  value,
  onChange,
  disabled,
}: {
  placeholder: string
  value: string
  onChange: (v: string) => void
  disabled: boolean
}) {
  const [show, setShow] = useState(false)
  return (
    <div className="relative w-full">
      <input
        type={show ? 'text' : 'password'}
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        required
        disabled={disabled}
        className="w-full px-3 py-2.5 pr-10 rounded-lg text-sm outline-none"
        style={{ background: 'var(--bg-primary)', border: '1px solid var(--border)', color: 'var(--text-primary)' }}
      />
      <button
        type="button"
        onClick={() => setShow((s) => !s)}
        tabIndex={-1}
        className="absolute right-3 top-1/2 -translate-y-1/2"
        style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-muted)', padding: 0, lineHeight: 0 }}
      >
        <EyeIcon open={show} />
      </button>
    </div>
  )
}

export default function LoginPage() {
  const [mode, setMode] = useState<Mode>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [name, setName] = useState('')
  const [loadingPassword, setLoadingPassword] = useState(false)
  const [loadingGoogle, setLoadingGoogle] = useState(false)
  const [loadingAlipay, setLoadingAlipay] = useState(false)
  const [searchParams] = useSearchParams()
  const [error, setError] = useState<string | null>(() => {
    const urlError = searchParams.get('error')
    return urlError ? decodeURIComponent(urlError) : null
  })
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)

  const handlePasswordSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    if (mode === 'register' && password !== confirm) {
      setError('Passwords do not match')
      return
    }
    setLoadingPassword(true)
    try {
      const data = mode === 'login'
        ? await loginByPassword(email, password)
        : await register(email, password, name || undefined)
      setAuth({ accessToken: data.access_token, refreshToken: data.refresh_token, userId: data.user_id })
      navigate('/dashboard')
    } catch (err: unknown) {
      setError(getErrorMessage(err))
    } finally {
      setLoadingPassword(false)
    }
  }

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

  const switchMode = (m: Mode) => {
    setMode(m)
    setError(null)
    setPassword('')
    setConfirm('')
  }

  const disabled = loadingPassword || loadingGoogle || loadingAlipay

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
          <h1 className="text-2xl font-bold mb-1" style={{ color: 'var(--text-primary)' }}>Platform</h1>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            {mode === 'login' ? 'Sign in to continue' : 'Create your account'}
          </p>
        </div>

        {/* Error */}
        {error && (
          <div className="w-full text-sm px-4 py-3 rounded-lg"
            style={{ background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', color: 'var(--danger)' }}>
            {error}
          </div>
        )}

        {/* Form */}
        <form className="w-full flex flex-col gap-3" onSubmit={handlePasswordSubmit}>
          {mode === 'register' && (
            <input
              type="text"
              placeholder="Name (optional)"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={disabled}
              className="w-full px-3 py-2.5 rounded-lg text-sm outline-none"
              style={{ background: 'var(--bg-primary)', border: '1px solid var(--border)', color: 'var(--text-primary)' }}
            />
          )}
          <input
            type="email"
            placeholder="Email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            disabled={disabled}
            className="w-full px-3 py-2.5 rounded-lg text-sm outline-none"
            style={{ background: 'var(--bg-primary)', border: '1px solid var(--border)', color: 'var(--text-primary)' }}
          />
          <PasswordInput
            placeholder="Password"
            value={password}
            onChange={setPassword}
            disabled={disabled}
          />
          {mode === 'register' && (
            <PasswordInput
              placeholder="Confirm password"
              value={confirm}
              onChange={setConfirm}
              disabled={disabled}
            />
          )}
          <button
            type="submit"
            disabled={disabled}
            className="w-full py-2.5 rounded-lg text-sm font-medium transition-all"
            style={{ background: 'var(--accent)', color: '#fff', opacity: disabled ? 0.7 : 1, cursor: disabled ? 'not-allowed' : 'pointer' }}
          >
            {loadingPassword ? '...' : mode === 'login' ? 'Sign in' : 'Create account'}
          </button>
        </form>

        {/* Mode toggle */}
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          {mode === 'login' ? "Don't have an account? " : 'Already have an account? '}
          <button
            onClick={() => switchMode(mode === 'login' ? 'register' : 'login')}
            className="font-medium"
            style={{ color: 'var(--accent)', background: 'none', border: 'none', cursor: 'pointer' }}
          >
            {mode === 'login' ? 'Register' : 'Sign in'}
          </button>
        </p>

        {/* Divider */}
        <div className="w-full flex items-center gap-3">
          <div className="flex-1 h-px" style={{ background: 'var(--border)' }} />
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>or</span>
          <div className="flex-1 h-px" style={{ background: 'var(--border)' }} />
        </div>

        {/* OAuth buttons */}
        <div className="w-full flex flex-col gap-3">
          <button onClick={handleGoogleLogin} disabled={disabled}
            className="w-full flex items-center justify-center gap-3 px-4 py-2.5 rounded-lg text-sm font-medium transition-all"
            style={{ background: '#ffffff', color: '#1a1a1a', border: '1px solid #e0e0e0', opacity: disabled ? 0.7 : 1, cursor: disabled ? 'not-allowed' : 'pointer' }}
          >
            <svg width="18" height="18" viewBox="0 0 24 24">
              <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4" />
              <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
              <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05" />
              <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
            </svg>
            {loadingGoogle ? 'Redirecting...' : 'Continue with Google'}
          </button>

          <button onClick={handleAlipayLogin} disabled={disabled}
            className="w-full flex items-center justify-center gap-3 px-4 py-2.5 rounded-lg text-sm font-medium transition-all"
            style={{ background: '#1677FF', color: '#ffffff', border: 'none', opacity: disabled ? 0.7 : 1, cursor: disabled ? 'not-allowed' : 'pointer' }}
          >
            <svg width="20" height="20" viewBox="0 0 100 100" fill="none">
              <rect width="100" height="100" rx="16" fill="white"/>
              <text x="50" y="72" textAnchor="middle" fontSize="62" fontWeight="bold" fill="#1677FF" fontFamily="Arial, sans-serif">支</text>
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
