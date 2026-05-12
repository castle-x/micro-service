import React, { useState, useEffect, useCallback } from 'react'
import { adminListPermissions, adminCreatePermission, type AdminPermission } from '../../lib/api'
import { getErrorMessage } from '../../lib/error'

export default function PermissionsPage() {
  const [perms, setPerms] = useState<AdminPermission[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [createModal, setCreateModal] = useState(false)
  const [form, setForm] = useState({ code: '', displayName: '', description: '' })
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setPerms(await adminListPermissions())
    } catch (e: unknown) {
      setError(getErrorMessage(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    queueMicrotask(() => { void load() })
  }, [load])

  const save = async () => {
    setSaving(true)
    try {
      await adminCreatePermission(form.code, form.displayName, form.description)
      setCreateModal(false)
      setForm({ code: '', displayName: '', description: '' })
      void load()
    } catch (e: unknown) {
      alert(getErrorMessage(e))
    } finally {
      setSaving(false)
    }
  }

  const systemPerms = perms.filter(p => p.IsSystem)
  const customPerms = perms.filter(p => !p.IsSystem)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h1 style={{ fontSize: 22, fontWeight: 700, color: 'var(--text-primary)', margin: 0 }}>权限管理</h1>
          <p style={{ color: 'var(--text-secondary)', fontSize: 13, margin: '4px 0 0' }}>
            {systemPerms.length} 个内置权限，{customPerms.length} 个自定义权限
          </p>
        </div>
        <button onClick={() => setCreateModal(true)} style={primaryBtn}>+ 新建权限</button>
      </div>

      {error && (
        <div style={{ padding: '10px 14px', borderRadius: 8, background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', color: 'var(--danger)', fontSize: 13 }}>
          {error}
        </div>
      )}

      {loading ? (
        <div style={{ textAlign: 'center', padding: 40, color: 'var(--text-muted)' }}>加载中...</div>
      ) : (
        <>
          <Section title="内置权限" items={systemPerms} />
          {customPerms.length > 0 && <Section title="自定义权限" items={customPerms} />}
        </>
      )}

      {createModal && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100 }}
          onClick={e => { if (e.target === e.currentTarget) setCreateModal(false) }}>
          <div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border)', borderRadius: 12, padding: 24, width: 440, maxWidth: '90vw' }}>
            <h2 style={{ fontSize: 15, fontWeight: 600, color: 'var(--text-primary)', marginBottom: 16 }}>新建权限</h2>

            <div style={{ marginBottom: 12 }}>
              <label style={labelStyle}>权限 Code（格式：resource:action，如 report:view）</label>
              <input value={form.code} onChange={e => setForm({ ...form, code: e.target.value })}
                style={inputStyle} placeholder="report:view" />
            </div>
            <div style={{ marginBottom: 12 }}>
              <label style={labelStyle}>展示名称</label>
              <input value={form.displayName} onChange={e => setForm({ ...form, displayName: e.target.value })}
                style={inputStyle} placeholder="查看报表" />
            </div>
            <div style={{ marginBottom: 20 }}>
              <label style={labelStyle}>描述（可选）</label>
              <input value={form.description} onChange={e => setForm({ ...form, description: e.target.value })}
                style={inputStyle} placeholder="允许查看数据报表" />
            </div>

            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
              <button onClick={() => setCreateModal(false)} style={outlineBtn}>取消</button>
              <button onClick={save} disabled={saving || !form.code || !form.displayName} style={{ ...primaryBtn, opacity: (!form.code || !form.displayName) ? 0.6 : 1 }}>
                {saving ? '创建中...' : '创建'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function Section({ title, items }: { title: string; items: AdminPermission[] }) {
  return (
    <div>
      <h2 style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 10 }}>{title}</h2>
      <div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border)', borderRadius: 12, overflow: 'hidden' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--border)' }}>
              {['Code', '展示名', '描述', '类型'].map(h => (
                <th key={h} style={{ padding: '10px 16px', textAlign: 'left', color: 'var(--text-secondary)', fontWeight: 500 }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {items.map(p => (
              <tr key={p.Code} style={{ borderBottom: '1px solid var(--border)' }}
                onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-hover)')}
                onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}>
                <td style={{ padding: '10px 16px', fontFamily: 'monospace', color: 'rgb(167,139,250)' }}>{p.Code}</td>
                <td style={{ padding: '10px 16px', color: 'var(--text-primary)' }}>{p.DisplayName}</td>
                <td style={{ padding: '10px 16px', color: 'var(--text-muted)' }}>{p.Description || '-'}</td>
                <td style={{ padding: '10px 16px' }}>
                  {p.IsSystem ? (
                    <span style={{ fontSize: 11, color: 'rgb(251,191,36)', background: 'rgba(251,191,36,0.1)', padding: '2px 7px', borderRadius: 4 }}>内置</span>
                  ) : (
                    <span style={{ fontSize: 11, color: 'rgb(52,211,153)', background: 'rgba(52,211,153,0.1)', padding: '2px 7px', borderRadius: 4 }}>自定义</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

const primaryBtn: React.CSSProperties = {
  padding: '7px 16px', borderRadius: 8, border: 'none',
  background: 'var(--accent)', color: '#fff', cursor: 'pointer', fontSize: 13, fontWeight: 500,
}
const outlineBtn: React.CSSProperties = {
  padding: '7px 16px', borderRadius: 8,
  border: '1px solid var(--border)', background: 'transparent',
  color: 'var(--text-secondary)', cursor: 'pointer', fontSize: 13,
}
const labelStyle: React.CSSProperties = {
  display: 'block', fontSize: 12, color: 'var(--text-secondary)', marginBottom: 4
}
const inputStyle: React.CSSProperties = {
  width: '100%', padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border)',
  background: 'var(--bg-primary)', color: 'var(--text-primary)', fontSize: 13, boxSizing: 'border-box',
}
