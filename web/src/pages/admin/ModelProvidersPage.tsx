import { useState, useEffect, useCallback } from 'react'
import {
  modelListProviders, modelCreateProvider, modelSetEnabled, modelUpdateAPIKey,
  modelUpdateProvider, modelDeleteProvider, modelListModels, modelCreateModel,
  modelUpdateModel, modelDeleteModel, modelSetModelEnabled,
  modelChat,
  type ModelProvider, type ModelInfo, type ProviderTestResult,
} from '../../lib/api'
import { getErrorMessage, getResponseStatus } from '../../lib/error'

type ProviderForm = {
  name: string
  slug: string
  vendor: string
  base_url: string
  api_key: string
  default_model_ref: string
}

type ModelForm = {
  provider_slug: string
  model: string
  display_name: string
  capabilities: string
  context_window: string
  max_output_tokens: string
  default_parameters_json: string
}

const DEEPSEEK_PROVIDER_SLUG = 'deepseek'
const DEEPSEEK_DEFAULT_MODEL = 'deepseek-v4-flash'
const DEEPSEEK_DEFAULT_MODEL_REF = `${DEEPSEEK_PROVIDER_SLUG}/${DEEPSEEK_DEFAULT_MODEL}`
const DEEPSEEK_DEFAULT_DISPLAY_NAME = 'DeepSeek V4 Flash'

const PROVIDER_TEXT_FIELDS: Array<{ label: string; key: keyof ProviderForm; placeholder: string }> = [
  { label: '名称 *', key: 'name', placeholder: 'DeepSeek' },
  { label: 'Slug *', key: 'slug', placeholder: 'deepseek' },
  { label: 'Vendor *', key: 'vendor', placeholder: 'openai_compatible' },
  { label: 'Base URL *', key: 'base_url', placeholder: 'https://api.deepseek.com' },
  { label: 'API Key', key: 'api_key', placeholder: 'sk-...' },
  { label: '默认模型引用', key: 'default_model_ref', placeholder: DEEPSEEK_DEFAULT_MODEL_REF },
]

const MODEL_TEXT_FIELDS: Array<{ label: string; key: keyof ModelForm; placeholder: string }> = [
  { label: 'Provider Slug *', key: 'provider_slug', placeholder: 'deepseek' },
  { label: '上游模型 *', key: 'model', placeholder: DEEPSEEK_DEFAULT_MODEL },
  { label: '显示名称', key: 'display_name', placeholder: DEEPSEEK_DEFAULT_DISPLAY_NAME },
  { label: 'Capabilities *', key: 'capabilities', placeholder: 'chat, stream, tool_calling' },
  { label: 'Context Window', key: 'context_window', placeholder: '65536' },
  { label: 'Max Output Tokens', key: 'max_output_tokens', placeholder: '8192' },
  { label: 'Default Parameters JSON', key: 'default_parameters_json', placeholder: '{"temperature":0.7}' },
]

const emptyProviderForm = (): ProviderForm => ({
  name: 'DeepSeek',
  slug: 'deepseek',
  vendor: 'openai_compatible',
  base_url: 'https://api.deepseek.com',
  api_key: '',
  default_model_ref: DEEPSEEK_DEFAULT_MODEL_REF,
})

const emptyModelForm = (): ModelForm => ({
  provider_slug: '',
  model: '',
  display_name: '',
  capabilities: 'chat, stream',
  context_window: '',
  max_output_tokens: '',
  default_parameters_json: '',
})

const modelFormForProvider = (provider?: ModelProvider): ModelForm => {
  const form = emptyModelForm()
  if (!provider) {
    return {
      ...form,
      provider_slug: DEEPSEEK_PROVIDER_SLUG,
      model: DEEPSEEK_DEFAULT_MODEL,
      display_name: DEEPSEEK_DEFAULT_DISPLAY_NAME,
    }
  }
  if (provider.slug === DEEPSEEK_PROVIDER_SLUG) {
    return {
      ...form,
      provider_slug: provider.slug,
      model: DEEPSEEK_DEFAULT_MODEL,
      display_name: DEEPSEEK_DEFAULT_DISPLAY_NAME,
    }
  }

  const prefix = `${provider.slug}/`
  const defaultModel = provider.default_model_ref?.startsWith(prefix)
    ? provider.default_model_ref.slice(prefix.length)
    : ''
  return {
    ...form,
    provider_slug: provider.slug,
    model: defaultModel,
    display_name: defaultModel,
  }
}

const parseModelRef = (modelRef: string): { providerSlug: string; model: string; modelRef: string } | null => {
  const trimmed = modelRef.trim()
  const slash = trimmed.indexOf('/')
  if (slash <= 0 || slash === trimmed.length - 1) return null
  const providerSlug = trimmed.slice(0, slash).trim()
  const model = trimmed.slice(slash + 1).trim()
  if (!providerSlug || !model) return null
  return { providerSlug, model, modelRef: `${providerSlug}/${model}` }
}

const displayNameForModel = (providerSlug: string, model: string): string => {
  if (providerSlug === DEEPSEEK_PROVIDER_SLUG && model === DEEPSEEK_DEFAULT_MODEL) return DEEPSEEK_DEFAULT_DISPLAY_NAME
  return model
}

export default function ModelProvidersPage() {
  const [providers, setProviders] = useState<ModelProvider[]>([])
  const [models, setModels] = useState<ModelInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [showCreate, setShowCreate] = useState(false)
  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState<ProviderForm>(emptyProviderForm)
  const [editingProviderID, setEditingProviderID] = useState<string | null>(null)

  const [showCreateModel, setShowCreateModel] = useState(false)
  const [creatingModel, setCreatingModel] = useState(false)
  const [modelForm, setModelForm] = useState<ModelForm>(emptyModelForm)
  const [editingModelID, setEditingModelID] = useState<string | null>(null)

  const [apiKeyModal, setAPIKeyModal] = useState<{ id: string; value: string } | null>(null)
  const [savingKey, setSavingKey] = useState(false)
  const [testingProviderID, setTestingProviderID] = useState<string | null>(null)
  const [testResults, setTestResults] = useState<Record<string, ProviderTestResult>>({})
  const [testingModelID, setTestingModelID] = useState<string | null>(null)
  const [modelTestResults, setModelTestResults] = useState<Record<string, ProviderTestResult>>({})

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [nextProviders, nextModels] = await Promise.all([modelListProviders(), modelListModels()])
      setProviders(nextProviders)
      setModels(nextModels)
    } catch (e: unknown) {
      setError(getErrorMessage(e, 'Failed to load LLM setup'))
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

  const toggleModelEnabled = async (m: ModelInfo) => {
    try {
      await modelSetModelEnabled(m.id, !m.enabled)
      setModels((prev) => prev.map((x) => x.id === m.id ? { ...x, enabled: !m.enabled } : x))
    } catch (e: unknown) {
      alert(getErrorMessage(e, 'Failed'))
    }
  }

  const openCreateProvider = () => {
    setEditingProviderID(null)
    setForm(emptyProviderForm())
    setShowCreate(true)
  }

  const openEditProvider = (p: ModelProvider) => {
    setEditingProviderID(p.id)
    setForm({
      name: p.name,
      slug: p.slug,
      vendor: p.vendor,
      base_url: p.base_url,
      api_key: '',
      default_model_ref: p.default_model_ref ?? '',
    })
    setShowCreate(true)
  }

  const closeProviderModal = () => {
    setShowCreate(false)
    setEditingProviderID(null)
    setForm(emptyProviderForm())
  }

  const ensureDefaultModelRegistered = async (providerSlug: string, defaultModelRef: string) => {
    const parsed = parseModelRef(defaultModelRef)
    if (!parsed || parsed.providerSlug !== providerSlug) return
    if (models.some((m) => m.model_ref === parsed.modelRef)) return
    try {
      await modelCreateModel({
        provider_slug: parsed.providerSlug,
        model: parsed.model,
        display_name: displayNameForModel(parsed.providerSlug, parsed.model),
        capabilities: ['chat', 'stream'],
        enabled: true,
      })
    } catch (e: unknown) {
      if (getResponseStatus(e) !== 409) throw e
    }
  }

  const handleSaveProvider = async () => {
    if (!form.name || !form.vendor || !form.base_url || (!editingProviderID && !form.slug)) {
      alert(editingProviderID ? 'name, vendor, base_url 必填' : 'name, slug, vendor, base_url 必填')
      return
    }
    setCreating(true)
    try {
      if (editingProviderID) {
        const providerSlug = form.slug.trim()
        const defaultModelRef = form.default_model_ref.trim()
        await modelUpdateProvider(editingProviderID, {
          name: form.name.trim(),
          vendor: form.vendor.trim(),
          base_url: form.base_url.trim(),
          default_model_ref: defaultModelRef,
        })
        await ensureDefaultModelRegistered(providerSlug, defaultModelRef)
      } else {
        const providerSlug = form.slug.trim()
        const defaultModelRef = form.default_model_ref.trim()
        await modelCreateProvider({
          ...form,
          name: form.name.trim(),
          slug: providerSlug,
          vendor: form.vendor.trim(),
          base_url: form.base_url.trim(),
          api_key: form.api_key.trim(),
          default_model_ref: defaultModelRef,
        })
        await ensureDefaultModelRegistered(providerSlug, defaultModelRef)
      }
      closeProviderModal()
      await load()
    } catch (e: unknown) {
      alert(getErrorMessage(e, 'Failed'))
    } finally {
      setCreating(false)
    }
  }

  const handleDeleteProvider = async (p: ModelProvider) => {
    if (!window.confirm(`删除供应商 ${p.name}？请先删除该供应商下的模型。`)) return
    try {
      await modelDeleteProvider(p.id)
      await load()
    } catch (e: unknown) {
      alert(getErrorMessage(e, 'Failed'))
    }
  }

  const openCreateModel = (provider?: ModelProvider) => {
    setEditingModelID(null)
    setModelForm(modelFormForProvider(provider))
    setShowCreateModel(true)
  }

  const openEditModel = (m: ModelInfo) => {
    setEditingModelID(m.id)
    setModelForm({
      provider_slug: m.provider_slug,
      model: m.model,
      display_name: m.display_name ?? '',
      capabilities: m.capabilities.join(', '),
      context_window: m.context_window ? String(m.context_window) : '',
      max_output_tokens: m.max_output_tokens ? String(m.max_output_tokens) : '',
      default_parameters_json: m.default_parameters_json ?? '',
    })
    setShowCreateModel(true)
  }

  const closeModelModal = () => {
    setShowCreateModel(false)
    setEditingModelID(null)
    setModelForm(emptyModelForm())
  }

  const handleSaveModel = async () => {
    const capabilities = modelForm.capabilities.split(',').map((s) => s.trim()).filter(Boolean)
    if ((!editingModelID && (!modelForm.provider_slug || !modelForm.model)) || capabilities.length === 0) {
      alert(editingModelID ? 'capabilities 必填' : 'provider_slug, model, capabilities 必填')
      return
    }
    if (modelForm.default_parameters_json.trim()) {
      try { JSON.parse(modelForm.default_parameters_json) } catch {
        alert('Default Parameters JSON 格式错误')
        return
      }
    }
    setCreatingModel(true)
    try {
      const payload = {
        display_name: modelForm.display_name.trim(),
        capabilities,
        context_window: modelForm.context_window ? Number(modelForm.context_window) : undefined,
        max_output_tokens: modelForm.max_output_tokens ? Number(modelForm.max_output_tokens) : undefined,
        default_parameters_json: modelForm.default_parameters_json.trim() || undefined,
      }
      if (editingModelID) {
        await modelUpdateModel(editingModelID, payload)
      } else {
        await modelCreateModel({
          provider_slug: modelForm.provider_slug.trim(),
          model: modelForm.model.trim(),
          ...payload,
          enabled: true,
        })
      }
      closeModelModal()
      await load()
    } catch (e: unknown) {
      alert(getErrorMessage(e, 'Failed'))
    } finally {
      setCreatingModel(false)
    }
  }

  const handleDeleteModel = async (m: ModelInfo) => {
    if (!window.confirm(`删除模型 ${m.display_name || m.model_ref}？`)) return
    try {
      await modelDeleteModel(m.id)
      await load()
    } catch (e: unknown) {
      alert(getErrorMessage(e, 'Failed'))
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

  const handleTestProvider = async (p: ModelProvider) => {
    setTestingProviderID(p.id)
    try {
      const modelRef = p.default_model_ref?.trim()
      if (!modelRef) {
        setTestResults((prev) => ({ ...prev, [p.id]: { ok: false, message: '默认模型引用为空' } }))
        return
      }
      const content = await modelChat(modelRef, [{ role: 'user', content: 'ping' }], { max_tokens: 32 })
      const summary = content.trim()
      setTestResults((prev) => ({ ...prev, [p.id]: { ok: true, message: summary ? summary.slice(0, 120) : 'generate succeeded' } }))
    } catch (e: unknown) {
      setTestResults((prev) => ({ ...prev, [p.id]: { ok: false, message: getErrorMessage(e, 'Provider test failed') } }))
    } finally {
      setTestingProviderID(null)
    }
  }

  const handleTestModel = async (m: ModelInfo) => {
    setTestingModelID(m.id)
    try {
      const content = await modelChat(m.model_ref, [{ role: 'user', content: 'ping' }], { max_tokens: 32 })
      const summary = content.trim()
      setModelTestResults((prev) => ({ ...prev, [m.id]: { ok: true, message: summary ? summary.slice(0, 120) : 'generate succeeded' } }))
    } catch (e: unknown) {
      setModelTestResults((prev) => ({ ...prev, [m.id]: { ok: false, message: getErrorMessage(e, 'Model test failed') } }))
    } finally {
      setTestingModelID(null)
    }
  }

  const statusButtonStyle = (enabled: boolean): React.CSSProperties => ({
    background: enabled ? 'rgba(34,197,94,0.15)' : 'rgba(156,163,175,0.15)',
    color: enabled ? 'rgb(34,197,94)' : 'var(--text-muted)',
    border: 'none', borderRadius: 999, padding: '2px 12px', fontSize: 12,
    fontWeight: 600, cursor: 'pointer',
  })

  const outlineButtonStyle: React.CSSProperties = {
    background: 'transparent', color: 'var(--accent)', border: '1px solid var(--accent)',
    borderRadius: 6, padding: '3px 10px', fontSize: 12, cursor: 'pointer',
  }

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 24 }}>
        <h2 style={{ margin: 0, fontSize: 20, fontWeight: 700, color: 'var(--text-primary)' }}>AI 模型供应商</h2>
        <button
          onClick={openCreateProvider}
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
        <>
          <div style={{ overflowX: 'auto', marginBottom: 28 }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
              <thead>
                <tr style={{ background: 'var(--bg-secondary)', borderBottom: '1px solid var(--border)' }}>
                  {['名称', 'Slug', 'Vendor', '地址', '默认模型', '状态', '测试结果', '操作'].map((h) => (
                    <th key={h} style={{ padding: '10px 14px', textAlign: 'left', fontWeight: 600, color: 'var(--text-secondary)', fontSize: 12 }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {providers.length === 0 && (
                  <tr>
                    <td colSpan={8} style={{ padding: 32, textAlign: 'center', color: 'var(--text-muted)' }}>暂无供应商</td>
                  </tr>
                )}
                {providers.map((p) => {
                  const result = testResults[p.id]
                  return (
                    <tr key={p.id} style={{ borderBottom: '1px solid var(--border)' }}>
                      <td style={{ padding: '10px 14px', color: 'var(--text-primary)', fontWeight: 500 }}>{p.name}</td>
                      <td style={{ padding: '10px 14px', color: 'var(--text-secondary)', fontFamily: 'monospace' }}>{p.slug}</td>
                      <td style={{ padding: '10px 14px' }}>
                        <span style={{ background: 'rgba(99,102,241,0.15)', color: 'rgb(99,102,241)', padding: '2px 8px', borderRadius: 999, fontSize: 12, fontWeight: 500 }}>
                          {p.vendor}
                        </span>
                      </td>
                      <td style={{ padding: '10px 14px', color: 'var(--text-secondary)', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.base_url}</td>
                      <td style={{ padding: '10px 14px', color: 'var(--text-secondary)', fontFamily: 'monospace', fontSize: 12 }}>{p.default_model_ref || '—'}</td>
                      <td style={{ padding: '10px 14px' }}>
                        <button onClick={() => toggleEnabled(p)} style={statusButtonStyle(p.enabled)}>
                          {p.enabled ? '已启用' : '已禁用'}
                        </button>
                      </td>
                      <td style={{ padding: '10px 14px', color: result?.ok === false ? 'rgb(239,68,68)' : 'var(--text-secondary)', fontSize: 12, maxWidth: 240 }}>
                        {result ? `${result.ok ? 'OK' : 'FAIL'} ${result.message ?? ''}` : '—'}
                      </td>
                      <td style={{ padding: '10px 14px' }}>
                        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                          <button onClick={() => setAPIKeyModal({ id: p.id, value: '' })} style={outlineButtonStyle}>更新 Key</button>
                          <button onClick={() => handleTestProvider(p)} disabled={testingProviderID === p.id} style={{ ...outlineButtonStyle, opacity: testingProviderID === p.id ? 0.6 : 1 }}>
                            {testingProviderID === p.id ? '测试中...' : '测试'}
                          </button>
                          <button onClick={() => openEditProvider(p)} style={outlineButtonStyle}>编辑</button>
                          <button onClick={() => handleDeleteProvider(p)} style={{ ...outlineButtonStyle, color: 'rgb(239,68,68)', borderColor: 'rgb(239,68,68)' }}>删除</button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
            <h3 style={{ margin: 0, fontSize: 16, fontWeight: 700, color: 'var(--text-primary)' }}>模型列表</h3>
            <button
              onClick={() => {
                const first = providers[0]
                openCreateModel(first)
              }}
              style={{
                background: 'transparent', color: 'var(--accent)', border: '1px solid var(--accent)',
                borderRadius: 8, padding: '6px 14px', fontSize: 13, fontWeight: 600, cursor: 'pointer',
              }}
            >
              + 新增模型
            </button>
          </div>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
              <thead>
                <tr style={{ background: 'var(--bg-secondary)', borderBottom: '1px solid var(--border)' }}>
                  {['Provider', 'Model Ref', '显示名称', 'Capabilities', 'Context', '状态', '测试结果', '操作'].map((h) => (
                    <th key={h} style={{ padding: '10px 14px', textAlign: 'left', fontWeight: 600, color: 'var(--text-secondary)', fontSize: 12 }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {models.length === 0 && (
                  <tr>
                    <td colSpan={8} style={{ padding: 32, textAlign: 'center', color: 'var(--text-muted)' }}>暂无模型</td>
                  </tr>
                )}
                {models.map((m) => {
                  const result = modelTestResults[m.id]
                  return (
                    <tr key={m.id} style={{ borderBottom: '1px solid var(--border)' }}>
                      <td style={{ padding: '10px 14px', color: 'var(--text-secondary)', fontFamily: 'monospace' }}>{m.provider_slug}</td>
                      <td style={{ padding: '10px 14px', color: 'var(--text-primary)', fontFamily: 'monospace', fontSize: 12 }}>{m.model_ref}</td>
                      <td style={{ padding: '10px 14px', color: 'var(--text-secondary)' }}>{m.display_name || m.model}</td>
                      <td style={{ padding: '10px 14px', color: 'var(--text-secondary)', fontSize: 12 }}>{m.capabilities.join(', ')}</td>
                      <td style={{ padding: '10px 14px', color: 'var(--text-secondary)', fontSize: 12 }}>{m.context_window || '—'}</td>
                      <td style={{ padding: '10px 14px' }}>
                        <button onClick={() => toggleModelEnabled(m)} style={statusButtonStyle(m.enabled)}>
                          {m.enabled ? '已启用' : '已禁用'}
                        </button>
                      </td>
                      <td style={{ padding: '10px 14px', color: result?.ok === false ? 'rgb(239,68,68)' : 'var(--text-secondary)', fontSize: 12, maxWidth: 240 }}>
                        {result ? `${result.ok ? 'OK' : 'FAIL'} ${result.message ?? ''}` : '—'}
                      </td>
                      <td style={{ padding: '10px 14px' }}>
                        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                          <button onClick={() => handleTestModel(m)} disabled={testingModelID === m.id} style={{ ...outlineButtonStyle, opacity: testingModelID === m.id ? 0.6 : 1 }}>
                            {testingModelID === m.id ? '测试中...' : '测试'}
                          </button>
                          <button onClick={() => openEditModel(m)} style={outlineButtonStyle}>编辑</button>
                          <button onClick={() => handleDeleteModel(m)} style={{ ...outlineButtonStyle, color: 'rgb(239,68,68)', borderColor: 'rgb(239,68,68)' }}>删除</button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </>
      )}

      {showCreate && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100 }}>
          <div style={{ background: 'var(--bg-primary)', borderRadius: 12, padding: 28, width: 440, boxShadow: '0 8px 32px rgba(0,0,0,0.3)' }}>
            <h3 style={{ margin: '0 0 20px', color: 'var(--text-primary)', fontSize: 16, fontWeight: 700 }}>{editingProviderID ? '编辑供应商' : '新增供应商'}</h3>
            {PROVIDER_TEXT_FIELDS
              .filter(({ key }) => !editingProviderID || (key !== 'slug' && key !== 'api_key'))
              .map(({ label, key, placeholder }) => (
              <div key={key} style={{ marginBottom: 14 }}>
                <label style={{ display: 'block', fontSize: 12, color: 'var(--text-secondary)', marginBottom: 4 }}>{label}</label>
                <input
                  aria-label={label}
                  value={form[key]}
                  onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
                  placeholder={placeholder}
                  style={{ width: '100%', boxSizing: 'border-box', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--bg-secondary)', color: 'var(--text-primary)', fontSize: 13 }}
                />
              </div>
            ))}
            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <button onClick={closeProviderModal} style={{ padding: '8px 16px', borderRadius: 6, border: '1px solid var(--border)', background: 'transparent', color: 'var(--text-secondary)', cursor: 'pointer', fontSize: 13 }}>取消</button>
              <button onClick={handleSaveProvider} disabled={creating} style={{ padding: '8px 16px', borderRadius: 6, border: 'none', background: 'var(--accent)', color: '#fff', cursor: 'pointer', fontWeight: 600, fontSize: 13, opacity: creating ? 0.7 : 1 }}>
                {creating ? '提交中...' : (editingProviderID ? '保存' : '创建')}
              </button>
            </div>
          </div>
        </div>
      )}

      {showCreateModel && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100 }}>
          <div style={{ background: 'var(--bg-primary)', borderRadius: 12, padding: 28, width: 460, boxShadow: '0 8px 32px rgba(0,0,0,0.3)' }}>
            <h3 style={{ margin: '0 0 20px', color: 'var(--text-primary)', fontSize: 16, fontWeight: 700 }}>{editingModelID ? '编辑模型' : '新增模型'}</h3>
            {MODEL_TEXT_FIELDS.map(({ label, key, placeholder }) => (
              <div key={key} style={{ marginBottom: 14 }}>
                <label style={{ display: 'block', fontSize: 12, color: 'var(--text-secondary)', marginBottom: 4 }}>{label}</label>
                <input
                  aria-label={label}
                  value={modelForm[key]}
                  disabled={!!editingModelID && (key === 'provider_slug' || key === 'model')}
                  onChange={(e) => setModelForm((f) => ({ ...f, [key]: e.target.value }))}
                  placeholder={placeholder}
                  style={{ width: '100%', boxSizing: 'border-box', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--bg-secondary)', color: 'var(--text-primary)', fontSize: 13, opacity: editingModelID && (key === 'provider_slug' || key === 'model') ? 0.6 : 1 }}
                />
              </div>
            ))}
            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <button onClick={closeModelModal} style={{ padding: '8px 16px', borderRadius: 6, border: '1px solid var(--border)', background: 'transparent', color: 'var(--text-secondary)', cursor: 'pointer', fontSize: 13 }}>取消</button>
              <button onClick={handleSaveModel} disabled={creatingModel} style={{ padding: '8px 16px', borderRadius: 6, border: 'none', background: 'var(--accent)', color: '#fff', cursor: 'pointer', fontWeight: 600, fontSize: 13, opacity: creatingModel ? 0.7 : 1 }}>
                {creatingModel ? '提交中...' : (editingModelID ? '保存' : '创建模型')}
              </button>
            </div>
          </div>
        </div>
      )}

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
