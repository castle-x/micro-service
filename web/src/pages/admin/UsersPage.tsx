import React, { useState, useEffect, useCallback } from 'react'
import {
  adminListUsers, adminListRoles, adminUpdateUserRole, adminUpdateUserStatus,
  type AdminUser, type AdminRole
} from '../../lib/api'

const STATUS_LABEL: Record<number, string> = { 1: 'Active', 2: 'Disabled', 3: 'Banned' }
const STATUS_COLOR: Record<number, string> = {
  1: 'rgba(34,197,94,0.15)',
  2: 'rgba(234,179,8,0.15)',
  3: 'rgba(239,68,68,0.15)',
}
const STATUS_TEXT: Record<number, string> = {
  1: 'rgb(34,197,94)',
  2: 'rgb(234,179,8)',
  3: 'rgb(239,68,68)',
}

function Badge({ text, bg, color }: { text: string; bg: string; color: string }) {
  return (
    <span style={{ background: bg, color, padding: '2px 8px', borderRadius: 999, fontSize: 12, fontWeight: 500 }}>
      {text}
    </span>
  )
}

export default function UsersPage() {
  const [users, setUsers] = useState<AdminUser[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [roles, setRoles] = useState<AdminRole[]>([])
  const [error, setError] = useState<string | null>(null)

  // modal state
  const [roleModal, setRoleModal] = useState<{ user: AdminUser; newRole: string } | null>(null)
  const [statusModal, setStatusModal] = useState<{ user: AdminUser; newStatus: number } | null>(null)
  const [saving, setSaving] = useState(false)

  const pageSize = 15

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [ud, rd] = await Promise.all([adminListUsers(page, pageSize), adminListRoles()])
      setUsers(ud.users ?? [])
      setTotal(ud.total)
      setRoles(rd)
    } catch (e: any) {
      setError(e.response?.data?.message || e.message)
    } finally {
      setLoading(false)
    }
  }, [page])

  useEffect(() => { load() }, [load])

  const saveRole = async () => {
    if (!roleModal) return
    setSaving(true)
    try {
      await adminUpdateUserRole(roleModal.user.UserID, roleModal.newRole)
      setRoleModal(null)
      load()
    } catch (e: any) {
      alert(e.response?.data?.message || e.message)
    } finally {
      setSaving(false)
    }
  }

  const saveStatus = async () => {
    if (!statusModal) return
    setSaving(true)
    try {
      await adminUpdateUserStatus(statusModal.user.UserID, statusModal.newStatus)
      setStatusModal(null)
      load()
    } catch (e: any) {
      alert(e.response?.data?.message || e.message)
    } finally {
      setSaving(false)
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h1 style={{ fontSize: 22, fontWeight: 700, color: 'var(--text-primary)', margin: 0 }}>用户管理</h1>
          <p style={{ color: 'var(--text-secondary)', fontSize: 13, margin: '4px 0 0' }}>共 {total} 个用户</p>
        </div>
        <button onClick={load} style={{ padding: '6px 14px', borderRadius: 8, background: 'var(--bg-hover)', border: '1px solid var(--border)', color: 'var(--text-secondary)', cursor: 'pointer', fontSize: 13 }}>
          刷新
        </button>
      </div>

      {error && (
        <div style={{ padding: '10px 14px', borderRadius: 8, background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', color: 'var(--danger)', fontSize: 13 }}>
          {error}
        </div>
      )}

      <div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border)', borderRadius: 12, overflow: 'hidden' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--border)' }}>
              {['邮箱', '姓名', '角色', '状态', '注册时间', '操作'].map(h => (
                <th key={h} style={{ padding: '10px 16px', textAlign: 'left', color: 'var(--text-secondary)', fontWeight: 500 }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={6} style={{ padding: 32, textAlign: 'center', color: 'var(--text-muted)' }}>加载中...</td></tr>
            ) : users.length === 0 ? (
              <tr><td colSpan={6} style={{ padding: 32, textAlign: 'center', color: 'var(--text-muted)' }}>暂无数据</td></tr>
            ) : users.map(u => (
              <tr key={u.UserID} style={{ borderBottom: '1px solid var(--border)' }}
                onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-hover)')}
                onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}>
                <td style={{ padding: '10px 16px', color: 'var(--text-primary)' }}>{u.Email}</td>
                <td style={{ padding: '10px 16px', color: 'var(--text-secondary)' }}>{u.Name || '-'}</td>
                <td style={{ padding: '10px 16px' }}>
                  <Badge text={u.Role} bg='rgba(139,92,246,0.15)' color='rgb(167,139,250)' />
                </td>
                <td style={{ padding: '10px 16px' }}>
                  <Badge text={STATUS_LABEL[u.Status] ?? String(u.Status)} bg={STATUS_COLOR[u.Status] ?? 'transparent'} color={STATUS_TEXT[u.Status] ?? 'var(--text-muted)'} />
                </td>
                <td style={{ padding: '10px 16px', color: 'var(--text-muted)' }}>
                  {new Date(u.CreatedAt * 1000).toLocaleDateString()}
                </td>
                <td style={{ padding: '10px 16px' }}>
                  <div style={{ display: 'flex', gap: 8 }}>
                    <button onClick={() => setRoleModal({ user: u, newRole: u.Role })}
                      style={{ padding: '4px 10px', borderRadius: 6, border: '1px solid var(--border)', background: 'transparent', color: 'var(--text-secondary)', cursor: 'pointer', fontSize: 12 }}>
                      改角色
                    </button>
                    <button onClick={() => setStatusModal({ user: u, newStatus: u.Status })}
                      style={{ padding: '4px 10px', borderRadius: 6, border: '1px solid var(--border)', background: 'transparent', color: 'var(--text-secondary)', cursor: 'pointer', fontSize: 12 }}>
                      改状态
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>

        {/* Pagination */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 8, padding: '10px 16px', borderTop: '1px solid var(--border)' }}>
          <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1}
            style={{ padding: '4px 10px', borderRadius: 6, border: '1px solid var(--border)', background: 'transparent', color: 'var(--text-secondary)', cursor: page === 1 ? 'not-allowed' : 'pointer', opacity: page === 1 ? 0.5 : 1, fontSize: 12 }}>
            上一页
          </button>
          <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{page} / {totalPages}</span>
          <button onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={page === totalPages}
            style={{ padding: '4px 10px', borderRadius: 6, border: '1px solid var(--border)', background: 'transparent', color: 'var(--text-secondary)', cursor: page === totalPages ? 'not-allowed' : 'pointer', opacity: page === totalPages ? 0.5 : 1, fontSize: 12 }}>
            下一页
          </button>
        </div>
      </div>

      {/* Role Modal */}
      {roleModal && (
        <Modal title={`修改角色 — ${roleModal.user.Email}`} onClose={() => setRoleModal(null)}>
          <p style={{ fontSize: 13, color: 'var(--text-secondary)', marginBottom: 12 }}>
            当前角色：<strong style={{ color: 'var(--text-primary)' }}>{roleModal.user.Role}</strong>
          </p>
          <select value={roleModal.newRole} onChange={e => setRoleModal({ ...roleModal, newRole: e.target.value })}
            style={{ width: '100%', padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border)', background: 'var(--bg-primary)', color: 'var(--text-primary)', fontSize: 13, marginBottom: 16 }}>
            {roles.map(r => <option key={r.RoleID} value={r.Name}>{r.DisplayName}（{r.Name}）</option>)}
          </select>
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <button onClick={() => setRoleModal(null)} style={outlineBtn}>取消</button>
            <button onClick={saveRole} disabled={saving} style={primaryBtn}>{saving ? '保存中...' : '确认修改'}</button>
          </div>
        </Modal>
      )}

      {/* Status Modal */}
      {statusModal && (
        <Modal title={`修改状态 — ${statusModal.user.Email}`} onClose={() => setStatusModal(null)}>
          <p style={{ fontSize: 13, color: 'var(--text-secondary)', marginBottom: 12 }}>
            当前状态：<strong style={{ color: 'var(--text-primary)' }}>{STATUS_LABEL[statusModal.user.Status]}</strong>
          </p>
          <select value={statusModal.newStatus} onChange={e => setStatusModal({ ...statusModal, newStatus: Number(e.target.value) })}
            style={{ width: '100%', padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border)', background: 'var(--bg-primary)', color: 'var(--text-primary)', fontSize: 13, marginBottom: 16 }}>
            <option value={1}>Active（正常）</option>
            <option value={2}>Disabled（禁用）</option>
            <option value={3}>Banned（封禁）</option>
          </select>
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <button onClick={() => setStatusModal(null)} style={outlineBtn}>取消</button>
            <button onClick={saveStatus} disabled={saving} style={primaryBtn}>{saving ? '保存中...' : '确认修改'}</button>
          </div>
        </Modal>
      )}
    </div>
  )
}

function Modal({ title, children, onClose }: { title: string; children: React.ReactNode; onClose: () => void }) {
  return (
    <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100 }}
      onClick={e => { if (e.target === e.currentTarget) onClose() }}>
      <div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border)', borderRadius: 12, padding: 24, width: 400, maxWidth: '90vw' }}>
        <h2 style={{ fontSize: 15, fontWeight: 600, color: 'var(--text-primary)', marginBottom: 16 }}>{title}</h2>
        {children}
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
