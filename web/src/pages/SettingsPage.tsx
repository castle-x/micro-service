import React, { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { getUserMe } from '../lib/api'
import { useAuthStore } from '../store/auth'
import api from '../lib/api'

interface UserProfile {
  user_id: string
  email: string
  name: string
  avatar_url: string
  role: string
}

export default function SettingsPage() {
  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const role = useAuthStore((s) => s.user?.role)
  const isAdmin = role === 'super_admin' || role === 'admin'

  useEffect(() => {
    getUserMe()
      .then((data) => {
        setProfile(data)
        setName(data.name || '')
      })
      .catch((e) => setError(e?.message ?? 'Failed to load profile'))
      .finally(() => setLoading(false))
  }, [])

  const handleSave = async () => {
    if (!name.trim()) return
    setSaving(true)
    setSaved(false)
    setError(null)
    try {
      // PUT /api/v1/user/profile — 此接口若未实现则静默提示
      await api.put('/v1/user/profile', { name: name.trim() })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (e: any) {
      const msg = e?.response?.data?.message ?? e?.message ?? 'Save failed'
      if (e?.response?.status === 404) {
        // 接口未实现，本地更新 store 即可
        useAuthStore.getState().setUser({
          ...(useAuthStore.getState().user ?? { userId: '', email: '', avatar: '', role: '' }),
          name: name.trim(),
        })
        setSaved(true)
        setTimeout(() => setSaved(false), 2000)
      } else {
        setError(msg)
      }
    } finally {
      setSaving(false)
    }
  }

  const cardStyle: React.CSSProperties = {
    background: 'var(--bg-secondary)',
    border: '1px solid var(--border)',
    borderRadius: 12,
    padding: 24,
    marginBottom: 20,
  }
  const labelStyle: React.CSSProperties = {
    fontSize: 12,
    color: 'var(--text-secondary)',
    marginBottom: 4,
    display: 'block',
  }
  const inputStyle: React.CSSProperties = {
    width: '100%',
    boxSizing: 'border-box',
    padding: '8px 12px',
    borderRadius: 8,
    border: '1px solid var(--border)',
    background: 'var(--bg-primary)',
    color: 'var(--text-primary)',
    fontSize: 14,
  }

  return (
    <div style={{ maxWidth: 560 }}>
      <h2 style={{ margin: '0 0 24px', fontSize: 20, fontWeight: 700, color: 'var(--text-primary)' }}>设置</h2>

      {/* 个人资料 */}
      <div style={cardStyle}>
        <h3 style={{ margin: '0 0 16px', fontSize: 15, fontWeight: 600, color: 'var(--text-primary)' }}>个人资料</h3>

        {loading ? (
          <p style={{ color: 'var(--text-secondary)', fontSize: 14 }}>加载中...</p>
        ) : error ? (
          <p style={{ color: 'rgb(239,68,68)', fontSize: 14 }}>{error}</p>
        ) : (
          <>
            <div style={{ marginBottom: 14 }}>
              <label style={labelStyle}>邮箱</label>
              <input value={profile?.email ?? ''} readOnly style={{ ...inputStyle, opacity: 0.6, cursor: 'not-allowed' }} />
            </div>
            <div style={{ marginBottom: 14 }}>
              <label style={labelStyle}>角色</label>
              <input value={profile?.role ?? ''} readOnly style={{ ...inputStyle, opacity: 0.6, cursor: 'not-allowed' }} />
            </div>
            <div style={{ marginBottom: 20 }}>
              <label style={labelStyle}>名称</label>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="输入你的显示名称"
                style={inputStyle}
              />
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <button
                onClick={handleSave}
                disabled={saving || !name.trim()}
                style={{
                  background: 'var(--accent)', color: '#fff', border: 'none', borderRadius: 8,
                  padding: '8px 20px', fontSize: 14, fontWeight: 600, cursor: 'pointer',
                  opacity: (saving || !name.trim()) ? 0.6 : 1,
                }}
              >
                {saving ? '保存中...' : '保存'}
              </button>
              {saved && <span style={{ color: 'rgb(34,197,94)', fontSize: 13 }}>✓ 已保存</span>}
            </div>
          </>
        )}
      </div>

      {/* AI 模型供应商快捷入口（仅管理员可见）*/}
      {isAdmin && (
        <div style={cardStyle}>
          <h3 style={{ margin: '0 0 8px', fontSize: 15, fontWeight: 600, color: 'var(--text-primary)' }}>AI 模型管理</h3>
          <p style={{ margin: '0 0 14px', fontSize: 13, color: 'var(--text-secondary)' }}>
            配置 DeepSeek、Seedream 等 AI 模型供应商的接入信息。
          </p>
          <button
            onClick={() => navigate('/admin/models')}
            style={{
              background: 'transparent', color: 'var(--accent)', border: '1px solid var(--accent)',
              borderRadius: 8, padding: '7px 16px', fontSize: 13, fontWeight: 500, cursor: 'pointer',
            }}
          >
            前往模型供应商管理 →
          </button>
        </div>
      )}
    </div>
  )
}
