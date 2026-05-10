import React, { useState, useEffect, useCallback } from 'react'
import {
  adminListRoles, adminListPermissions, adminCreateRole, adminUpdateRole, adminDeleteRole,
  type AdminRole, type AdminPermission
} from '../../lib/api'

export default function RolesPage() {
  const [roles, setRoles] = useState<AdminRole[]>([])
  const [perms, setPerms] = useState<AdminPermission[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // edit modal
  const [editModal, setEditModal] = useState<{
    mode: 'create' | 'edit'
    role?: AdminRole
    name: string
    displayName: string
    selectedPerms: string[]
  } | null>(null)
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [r, p] = await Promise.all([adminListRoles(), adminListPermissions()])
      setRoles(r)
      setPerms(p)
    } catch (e: any) {
      setError(e.response?.data?.message || e.message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const openCreate = () => setEditModal({ mode: 'create', name: '', displayName: '', selectedPerms: [] })
  const openEdit = (r: AdminRole) => setEditModal({ mode: 'edit', role: r, name: r.Name, displayName: r.DisplayName, selectedPerms: [...r.Permissions] })

  const save = async () => {
    if (!editModal) return
    setSaving(true)
    try {
      if (editModal.mode === 'create') {
        await adminCreateRole(editModal.name, editModal.displayName, editModal.selectedPerms)
      } else if (editModal.role) {
        await adminUpdateRole(editModal.role.RoleID, editModal.displayName, editModal.selectedPerms)
      }
      setEditModal(null)
      load()
    } catch (e: any) {
      alert(e.response?.data?.message || e.message)
    } finally {
      setSaving(false)
    }
  }

  const del = async (role: AdminRole) => {
    if (!confirm(`确认删除角色 "${role.DisplayName}"？`)) return
    try {
      await adminDeleteRole(role.RoleID)
      load()
    } catch (e: any) {
      alert(e.response?.data?.message || e.message)
    }
  }

  const togglePerm = (code: string) => {
    if (!editModal) return
    const has = editModal.selectedPerms.includes(code)
    setEditModal({ ...editModal, selectedPerms: has ? editModal.selectedPerms.filter(p => p !== code) : [...editModal.selectedPerms, code] })
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h1 style={{ fontSize: 22, fontWeight: 700, color: 'var(--text-primary)', margin: 0 }}>角色管理</h1>
          <p style={{ color: 'var(--text-secondary)', fontSize: 13, margin: '4px 0 0' }}>共 {roles.length} 个角色</p>
        </div>
        <button onClick={openCreate} style={primaryBtn}>+ 新建角色</button>
      </div>

      {error && (
        <div style={{ padding: '10px 14px', borderRadius: 8, background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', color: 'var(--danger)', fontSize: 13 }}>
          {error}
        </div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {loading ? (
          <div style={{ textAlign: 'center', padding: 40, color: 'var(--text-muted)' }}>加载中...</div>
        ) : roles.map(r => (
          <div key={r.RoleID} style={{ background: 'var(--bg-surface)', border: '1px solid var(--border)', borderRadius: 12, padding: '16px 20px' }}>
            <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12 }}>
              <div style={{ flex: 1 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                  <span style={{ fontSize: 15, fontWeight: 600, color: 'var(--text-primary)' }}>{r.DisplayName}</span>
                  <span style={{ fontSize: 12, color: 'var(--text-muted)', fontFamily: 'monospace', background: 'var(--bg-hover)', padding: '1px 6px', borderRadius: 4 }}>{r.Name}</span>
                  {r.IsSystem && (
                    <span style={{ fontSize: 11, color: 'rgb(251,191,36)', background: 'rgba(251,191,36,0.1)', padding: '1px 6px', borderRadius: 4 }}>内置</span>
                  )}
                </div>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                  {(r.Permissions ?? []).length === 0 ? (
                    <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>无权限</span>
                  ) : r.Permissions.map(p => (
                    <span key={p} style={{ fontSize: 11, padding: '2px 7px', borderRadius: 4, background: 'rgba(139,92,246,0.12)', color: 'rgb(167,139,250)', fontFamily: 'monospace' }}>{p}</span>
                  ))}
                </div>
              </div>
              <div style={{ display: 'flex', gap: 8, flexShrink: 0 }}>
                <button onClick={() => openEdit(r)} style={outlineBtn}>编辑</button>
                {!r.IsSystem && (
                  <button onClick={() => del(r)} style={{ ...outlineBtn, color: 'var(--danger)', borderColor: 'rgba(239,68,68,0.3)' }}>删除</button>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>

      {editModal && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100 }}
          onClick={e => { if (e.target === e.currentTarget) setEditModal(null) }}>
          <div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border)', borderRadius: 12, padding: 24, width: 520, maxWidth: '90vw', maxHeight: '80vh', overflowY: 'auto' }}>
            <h2 style={{ fontSize: 15, fontWeight: 600, color: 'var(--text-primary)', marginBottom: 16 }}>
              {editModal.mode === 'create' ? '新建角色' : `编辑角色 — ${editModal.role?.Name}`}
            </h2>

            {editModal.mode === 'create' && (
              <div style={{ marginBottom: 12 }}>
                <label style={labelStyle}>角色标识（英文，如 analyst）</label>
                <input value={editModal.name} onChange={e => setEditModal({ ...editModal, name: e.target.value })}
                  style={inputStyle} placeholder="role_name" />
              </div>
            )}

            <div style={{ marginBottom: 16 }}>
              <label style={labelStyle}>展示名称</label>
              <input value={editModal.displayName} onChange={e => setEditModal({ ...editModal, displayName: e.target.value })}
                style={inputStyle} placeholder="数据分析师" />
            </div>

            <div style={{ marginBottom: 20 }}>
              <label style={labelStyle}>权限（{editModal.selectedPerms.length}/{perms.length}）</label>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 6, marginTop: 8 }}>
                {perms.map(p => {
                  const checked = editModal.selectedPerms.includes(p.Code)
                  return (
                    <label key={p.Code} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 10px', borderRadius: 8, border: `1px solid ${checked ? 'var(--accent)' : 'var(--border)'}`, background: checked ? 'var(--accent-glow)' : 'transparent', cursor: 'pointer', fontSize: 12 }}>
                      <input type="checkbox" checked={checked} onChange={() => togglePerm(p.Code)} style={{ accentColor: 'var(--accent)', width: 14, height: 14 }} />
                      <div>
                        <div style={{ color: 'var(--text-primary)', fontFamily: 'monospace' }}>{p.Code}</div>
                        <div style={{ color: 'var(--text-muted)', fontSize: 11 }}>{p.DisplayName}</div>
                      </div>
                    </label>
                  )
                })}
              </div>
            </div>

            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
              <button onClick={() => setEditModal(null)} style={outlineBtn}>取消</button>
              <button onClick={save} disabled={saving} style={primaryBtn}>{saving ? '保存中...' : '保存'}</button>
            </div>
          </div>
        </div>
      )}
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
