import { useState, useEffect, useCallback } from 'react'
import {
  modelListProviders, modelCreateProvider, modelSetEnabled, modelUpdateAPIKey,
  type ModelProvider,
} from '../../lib/api'
import { getErrorMessage } from '../../lib/error'

const TYPE_LABEL: Record<string, string> = { llm: 'LLM', image: '图像生成' }
type ProviderForm = {
  name: string
  slug: string
  type: 'llm' | 'image'
  base_url: string
  api_key: string
  default_model: string
}
const PROVIDER_TEXT_FIELDS: Array<{ label: string; key: Exclude<keyof ProviderForm, 'type'>; placeholder: string }> = [
  { label: '名称 *', key: 'name', placeholder: 'DeepSeek' },
  { label: 'Slug *', key: 'slug', placeholder: 'deepseek' },
  { label: 'Base URL *', key: 'base_url', placeholder: 'https://api.deepseek.com' },
  { label: 'API Key', key: 'api_key', placeholder: 'sk-...' },
  { label: '默认模型', key: 'default_model', placeholder: 'deepseek-chat' },
]

export default function ModelProvidersPage() {
  const [providers, setProviders] = useState<ModelProvider[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // create modal
  const [showCreate, setShowCreate] = useState(false)
  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState<ProviderForm>({
    name: 'DeepSeek',
    slug: 'deepseek',
    type: 'llm' as 'llm' | 'image',
    base_url: 'https://api.deepseek.com',
    api_key: '',
    default_model: 'deepseek-v4-pro',
  })

  // api_key edit
  const [apiKeyModal, setAPIKeyModal] = useState<{ id: string; value: string } | null>(null)
  const [savingKey, setSavingKey] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setProviders(await modelListProviders())
    } catch (e: unknown) {
      setError(getErrorMessage(e, 'Failed to load providers'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    queueMicrotask(() => { void load() })
  }, [load])

  const toggleEnabled = async (p: ModelProvider) => {
    try {
      await modelSetEnabled(p.id, !p.enabled)
      setProviders((prev) => prev.map((x) => x.id === p.id ? { ...x, enabled: !p.enabled } : x))
    } catch (e: unknown) {
      alert(getErrorMessage(e, 'Failed'))
    }
  }

  const handleCreate = async () => {
    if (!form.name || !form.slug || !form.base_url) {
      alert('name, slug, base_url 必填')
      return
    }
    setCreating(true)
    try {
      await modelCreateProvider(form)
      setShowCreate(false)
      setForm({ name: '', slug: '', type: 'llm', base_url: '', api_key: '', default_model: '' })
      await load()
    } catch (e: unknown) {
      alert(getErrorMessage(e, 'Failed'))
    } finally {
      setCreating(false)
    }
  }

  const handleUpdateKey = async () => {
    if (!apiKeyModal || !apiKeyModal.value.trim()) return
    setSavingKey(true)
    try {
      await modelUpdateAPIKey(apiKeyModal.id, apiKeyModal.value.trim())
      setAPIKeyModal(null)
    } catch (e: unknown) {
      alert(getErrorMessage(e, 'Failed'))
    } finally {
      setSavingKey(false)
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 24 }}>
        <h2 style={{ margin: 0, fontSize: 20, fontWeight: 700, color: 'var(--text-primary)' }}>AI 模型供应商</h2>
        <button
          onClick={() => setShowCreate(true)}
          style={{
            background: 'var(--accent)', color: '#fff', border: 'none', borderRadius: 8,
            padding: '8px 18px', fontSize: 14, fontWeight: 600, cursor: 'pointer',
          }}
        >
          + 新增供应商
        </button>
      </div>

      {error && (
        <div style={{ background: 'rgba(239,68,68,0.1)', color: 'rgb(239,68,68)', padding: '10px 16px', borderRadius: 8, marginBottom: 16, fontSize: 14 }}>
          {error}
        </div>
      )}

      {loading ? (
        <div style={{ color: 'var(--text-secondary)', padding: 32, textAlign: 'center' }}>加载中...</div>
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
            <thead>
              <tr style={{ background: 'var(--bg-secondary)', borderBottom: '1px solid var(--border)' }}>
                {['名称', 'Slug', '类型', '地址', '默认模型', '状态', '操作'].map((h) => (
                  <th key={h} style={{ padding: '10px 14px', textAlign: 'left', fontWeight: 600, color: 'var(--text-secondary)', fontSize: 12 }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {providers.length === 0 && (
                <tr>
                  <td colSpan={7} style={{ padding: 32, textAlign: 'center', color: 'var(--text-muted)' }}>暂无供应商</td>
                </tr>
              )}
              {providers.map((p) => (
                <tr key={p.id} style={{ borderBottom: '1px solid var(--border)' }}>
                  <td style={{ padding: '10px 14px', color: 'var(--text-primary)', fontWeight: 500 }}>{p.name}</td>
                  <td style={{ padding: '10px 14px', color: 'var(--text-secondary)', fontFamily: 'monospace' }}>{p.slug}</td>
                  <td style={{ padding: '10px 14px' }}>
                    <span style={{
                      background: p.type === 'llm' ? 'rgba(99,102,241,0.15)' : 'rgba(234,179,8,0.15)',
                      color: p.type === 'llm' ? 'rgb(99,102,241)' : 'rgb(234,179,8)',
                      padding: '2px 8px', borderRadius: 999, fontSize: 12, fontWeight: 500,
                    }}>
                      {TYPE_LABEL[p.type] ?? p.type}
                    </span>
                  </td>
                  <td style={{ padding: '10px 14px', color: 'var(--text-secondary)', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.base_url}</td>
                  <td style={{ padding: '10px 14px', color: 'var(--text-secondary)', fontFamily: 'monospace', fontSize: 12 }}>{p.default_model || '—'}</td>
                  <td style={{ padding: '10px 14px' }}>
                    <button
                      onClick={() => toggleEnabled(p)}
                      style={{
                        background: p.enabled ? 'rgba(34,197,94,0.15)' : 'rgba(156,163,175,0.15)',
                        color: p.enabled ? 'rgb(34,197,94)' : 'var(--text-muted)',
                        border: 'none', borderRadius: 999, padding: '2px 12px', fontSize: 12,
                        fontWeight: 600, cursor: 'pointer',
                      }}
                    >
                      {p.enabled ? '已启用' : '已禁用'}
                    </button>
                  </td>
                  <td style={{ padding: '10px 14px' }}>
                    <button
                      onClick={() => setAPIKeyModal({ id: p.id, value: '' })}
                      style={{
                        background: 'transparent', color: 'var(--accent)', border: '1px solid var(--accent)',
                        borderRadius: 6, padding: '3px 10px', fontSize: 12, cursor: 'pointer',
                      }}
                    >
                      更新 Key
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Create Modal */}
      {showCreate && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100 }}>
          <div style={{ background: 'var(--bg-primary)', borderRadius: 12, padding: 28, width: 440, boxShadow: '0 8px 32px rgba(0,0,0,0.3)' }}>
            <h3 style={{ margin: '0 0 20px', color: 'var(--text-primary)', fontSize: 16, fontWeight: 700 }}>新增供应商</h3>
            {PROVIDER_TEXT_FIELDS.map(({ label, key, placeholder }) => (
              <div key={key} style={{ marginBottom: 14 }}>
                <label style={{ display: 'block', fontSize: 12, color: 'var(--text-secondary)', marginBottom: 4 }}>{label}</label>
                <input
                  value={form[key]}
                  onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
                  placeholder={placeholder}
                  style={{ width: '100%', boxSizing: 'border-box', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--bg-secondary)', color: 'var(--text-primary)', fontSize: 13 }}
                />
              </div>
            ))}
            <div style={{ marginBottom: 20 }}>
              <label style={{ display: 'block', fontSize: 12, color: 'var(--text-secondary)', marginBottom: 4 }}>类型</label>
              <select
                value={form.type}
                onChange={(e) => setForm((f) => ({ ...f, type: e.target.value as 'llm' | 'image' }))}
                style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--bg-secondary)', color: 'var(--text-primary)', fontSize: 13 }}
              >
                <option value="llm">LLM</option>
                <option value="image">图像生成</option>
              </select>
            </div>
            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <button onClick={() => setShowCreate(false)} style={{ padding: '8px 16px', borderRadius: 6, border: '1px solid var(--border)', background: 'transparent', color: 'var(--text-secondary)', cursor: 'pointer', fontSize: 13 }}>取消</button>
              <button onClick={handleCreate} disabled={creating} style={{ padding: '8px 16px', borderRadius: 6, border: 'none', background: 'var(--accent)', color: '#fff', cursor: 'pointer', fontWeight: 600, fontSize: 13, opacity: creating ? 0.7 : 1 }}>
                {creating ? '提交中...' : '创建'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Update API Key Modal */}
      {apiKeyModal && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100 }}>
          <div style={{ background: 'var(--bg-primary)', borderRadius: 12, padding: 28, width: 380, boxShadow: '0 8px 32px rgba(0,0,0,0.3)' }}>
            <h3 style={{ margin: '0 0 16px', color: 'var(--text-primary)', fontSize: 16, fontWeight: 700 }}>更新 API Key</h3>
            <input
              type="password"
              value={apiKeyModal.value}
              onChange={(e) => setAPIKeyModal((m) => m ? { ...m, value: e.target.value } : null)}
              placeholder="新的 API Key"
              style={{ width: '100%', boxSizing: 'border-box', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--bg-secondary)', color: 'var(--text-primary)', fontSize: 13, marginBottom: 20 }}
            />
            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <button onClick={() => setAPIKeyModal(null)} style={{ padding: '8px 16px', borderRadius: 6, border: '1px solid var(--border)', background: 'transparent', color: 'var(--text-secondary)', cursor: 'pointer', fontSize: 13 }}>取消</button>
              <button onClick={handleUpdateKey} disabled={savingKey} style={{ padding: '8px 16px', borderRadius: 6, border: 'none', background: 'var(--accent)', color: '#fff', cursor: 'pointer', fontWeight: 600, fontSize: 13, opacity: savingKey ? 0.7 : 1 }}>
                {savingKey ? '保存中...' : '保存'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
