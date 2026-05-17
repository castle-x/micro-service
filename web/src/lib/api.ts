import axios from 'axios'
import { useAuthStore } from '../store/auth'

const api = axios.create({
  baseURL: '/api',
  timeout: 10000,
})

// chat 接口单独用更长的超时（LLM 响应可能需要 60s+）
const chatApi = axios.create({
  baseURL: '/api',
  timeout: 120000,
})

api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

chatApi.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      useAuthStore.getState().logout()
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)

export interface ApiResponse<T> {
  code: number
  data: T
  message?: string
}

// ---- Auth ----

export const getGoogleAuthUrl = async (): Promise<string> => {
  const res = await api.get<ApiResponse<{ auth_url: string }>>('/v1/auth/google/url')
  return res.data.data.auth_url
}

export const getAlipayAuthUrl = async (): Promise<string> => {
  const res = await api.get<ApiResponse<{ auth_url: string }>>('/v1/auth/alipay/url')
  return res.data.data.auth_url
}

export interface AuthTokenData {
  access_token: string
  refresh_token: string
  expires_at: number
  user_id: string
}

export const register = async (email: string, password: string, name?: string): Promise<AuthTokenData> => {
  const res = await api.post<ApiResponse<AuthTokenData>>('/v1/auth/register', { email, password, name })
  return res.data.data
}

export const loginByPassword = async (email: string, password: string): Promise<AuthTokenData> => {
  const res = await api.post<ApiResponse<AuthTokenData>>('/v1/auth/login', { email, password })
  return res.data.data
}

export const getUserMe = async () => {
  const res = await api.get<ApiResponse<{
    user_id: string
    email: string
    name: string
    avatar_url: string
    role: string
  }>>('/v1/user/me')
  return res.data.data
}

export const postLogout = async (refreshToken: string) => {
  await api.post('/v1/auth/logout', { refresh_token: refreshToken })
}

// ---- Admin: Users ----

export interface AdminUser {
  UserID: string
  Email: string
  Name?: string
  AvatarURL?: string
  Phone?: string
  Role: string
  Status: number  // 1=active 2=disabled 3=banned
  CreatedAt: number
}

export const adminListUsers = async (page = 1, pageSize = 20, role = '') => {
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
  if (role) params.set('role', role)
  const res = await api.get<ApiResponse<{ users: AdminUser[]; total: number }>>(`/v1/admin/users?${params}`)
  return res.data.data
}

export const adminUpdateUserRole = async (userID: string, role: string) => {
  await api.put(`/v1/admin/users/${userID}/role`, { role })
}

export const adminUpdateUserStatus = async (userID: string, status: number) => {
  await api.put(`/v1/admin/users/${userID}/status`, { status })
}

// ---- Admin: Roles ----

export interface AdminRole {
  RoleID: string
  Name: string
  DisplayName: string
  Permissions: string[]
  IsSystem: boolean
}

export const adminListRoles = async (): Promise<AdminRole[]> => {
  const res = await api.get<ApiResponse<AdminRole[]>>('/v1/admin/roles')
  return res.data.data
}

export const adminCreateRole = async (name: string, displayName: string, permissions: string[]) => {
  const res = await api.post<ApiResponse<AdminRole>>('/v1/admin/roles', { name, display_name: displayName, permissions })
  return res.data.data
}

export const adminUpdateRole = async (roleID: string, displayName: string, permissions: string[]) => {
  await api.put(`/v1/admin/roles/${roleID}`, { display_name: displayName, permissions })
}

export const adminDeleteRole = async (roleID: string) => {
  await api.delete(`/v1/admin/roles/${roleID}`)
}

// ---- Admin: Permissions ----

export interface AdminPermission {
  Code: string
  DisplayName: string
  Description: string
  IsSystem: boolean
}

export const adminListPermissions = async (): Promise<AdminPermission[]> => {
  const res = await api.get<ApiResponse<AdminPermission[]>>('/v1/admin/permissions')
  return res.data.data
}

export const adminCreatePermission = async (code: string, displayName: string, description: string) => {
  const res = await api.post<ApiResponse<AdminPermission>>('/v1/admin/permissions', { code, display_name: displayName, description })
  return res.data.data
}

// ---- Model Providers ----

export interface ModelProvider {
  id: string
  name: string
  slug: string
  vendor: string
  base_url: string
  default_model_ref?: string | null
  enabled: boolean
  created_at: number
  updated_at: number
}

export interface ModelInfo {
  id: string
  provider_id?: string
  provider_slug: string
  model: string
  model_ref: string
  display_name?: string
  capabilities: string[]
  context_window?: number
  max_output_tokens?: number
  default_parameters_json?: string
  enabled: boolean
  created_at?: number
  updated_at?: number
}

export interface ProviderTestResult {
  ok: boolean
  message?: string
}

export const modelListProviders = async (): Promise<ModelProvider[]> => {
  const res = await api.get<ApiResponse<ModelProvider[]>>('/v1/admin/llm/providers')
  return res.data.data ?? []
}

export const modelCreateProvider = async (data: {
  name: string
  slug: string
  vendor: string
  base_url: string
  api_key: string
  default_model_ref: string
}): Promise<{ id: string }> => {
  const res = await api.post<ApiResponse<{ id: string }>>('/v1/admin/llm/providers', data)
  return res.data.data
}

export const modelUpdateProvider = async (id: string, data: {
  name: string
  vendor: string
  base_url: string
  default_model_ref: string
}) => {
  await api.put(`/v1/admin/llm/providers/${id}`, data)
}

export const modelDeleteProvider = async (id: string) => {
  await api.delete(`/v1/admin/llm/providers/${id}`)
}

export const modelSetEnabled = async (id: string, enabled: boolean) => {
  await api.patch(`/v1/admin/llm/providers/${id}/enabled`, { enabled })
}

export const modelUpdateAPIKey = async (id: string, apiKey: string) => {
  await api.patch(`/v1/admin/llm/providers/${id}/api-key`, { api_key: apiKey })
}

export const modelTestProvider = async (id: string): Promise<ProviderTestResult> => {
  const res = await api.post<ApiResponse<ProviderTestResult>>(`/v1/admin/llm/providers/${id}/test`)
  return res.data.data
}

export const modelListModels = async (params: { provider_slug?: string; enabled?: boolean } = {}): Promise<ModelInfo[]> => {
  const query = new URLSearchParams()
  if (params.provider_slug) query.set('provider_slug', params.provider_slug)
  if (params.enabled !== undefined) query.set('enabled', String(params.enabled))
  const suffix = query.toString() ? `?${query}` : ''
  const res = await api.get<ApiResponse<ModelInfo[]>>(`/v1/admin/llm/models${suffix}`)
  return res.data.data ?? []
}

export const modelCreateModel = async (data: {
  provider_slug: string
  model: string
  display_name?: string
  capabilities: string[]
  context_window?: number
  max_output_tokens?: number
  default_parameters_json?: string
  enabled?: boolean
}): Promise<{ id: string; model_ref?: string }> => {
  const res = await api.post<ApiResponse<{ id: string; model_ref?: string }>>('/v1/admin/llm/models', data)
  return res.data.data
}

export const modelUpdateModel = async (id: string, data: {
  display_name?: string
  capabilities: string[]
  context_window?: number
  max_output_tokens?: number
  default_parameters_json?: string
}) => {
  await api.put(`/v1/admin/llm/models/${id}`, data)
}

export const modelDeleteModel = async (id: string) => {
  await api.delete(`/v1/admin/llm/models/${id}`)
}

export const modelSetModelEnabled = async (id: string, enabled: boolean) => {
  await api.patch(`/v1/admin/llm/models/${id}/enabled`, { enabled })
}

type GenerateData = {
  message?: { content?: unknown } | string
  assistant?: { content?: unknown } | string
  content?: unknown
}

const readGenerateContent = (data?: GenerateData): string => {
  if (!data) return ''

  if (typeof data.message === 'string') return data.message
  if (data.message && typeof data.message === 'object' && typeof data.message.content === 'string') {
    return data.message.content
  }

  if (typeof data.assistant === 'string') return data.assistant
  if (data.assistant && typeof data.assistant === 'object' && typeof data.assistant.content === 'string') {
    return data.assistant.content
  }

  return typeof data.content === 'string' ? data.content : ''
}

export const modelChat = async (modelRef: string, messages: { role: string; content: string }[], extra: Record<string, unknown> = {}): Promise<string> => {
  const res = await chatApi.post<ApiResponse<GenerateData>>('/v1/admin/llm/generate', { model_ref: modelRef, messages, ...extra })
  return readGenerateContent(res.data.data)
}

// SSE stream chunk types
export type StreamChunkType = 'reasoning' | 'content' | 'done' | 'error'
export interface StreamChunk {
  type: StreamChunkType
  content?: string
  message?: string
  // token usage — only present when type === 'done'
  prompt_tokens?: number
  completion_tokens?: number
  total_tokens?: number
}

export const hasStreamUsage = (chunk: StreamChunk): boolean => (
  chunk.prompt_tokens !== undefined ||
  chunk.completion_tokens !== undefined ||
  chunk.total_tokens !== undefined
)

/**
 * modelChatStream — 使用 fetch + ReadableStream 消费 SSE 流式输出。
 * @param modelRef  model_ref
 * @param messages  对话消息列表
 * @param onChunk   每收到一个 chunk 的回调
 * @param signal    AbortSignal，用于取消请求
 */
export const modelChatStream = async (
  modelRef: string,
  messages: { role: string; content: string }[],
  onChunk: (chunk: StreamChunk) => void,
  signal?: AbortSignal,
  extra: Record<string, unknown> = {},
): Promise<void> => {
  const token = (await import('../store/auth')).useAuthStore.getState().accessToken
  const resp = await fetch('/api/v1/admin/llm/stream', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ model_ref: modelRef, messages, ...extra }),
    signal,
  })

  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}))
    throw new Error(body?.message ?? `HTTP ${resp.status}`)
  }

  const reader = resp.body!.getReader()
  const decoder = new TextDecoder()
  let buf = ''
  let usage: Pick<StreamChunk, 'prompt_tokens' | 'completion_tokens' | 'total_tokens'> = {}
  const readUsage = (payload: Record<string, unknown>) => {
    const next: Pick<StreamChunk, 'prompt_tokens' | 'completion_tokens' | 'total_tokens'> = {}
    if (typeof payload.prompt_tokens === 'number') next.prompt_tokens = payload.prompt_tokens
    if (typeof payload.completion_tokens === 'number') next.completion_tokens = payload.completion_tokens
    if (typeof payload.total_tokens === 'number') next.total_tokens = payload.total_tokens
    return next
  }

  while (true) {
    const { value, done } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })

    // SSE 格式：每个事件以 "\n\n" 结尾
    const parts = buf.split('\n\n')
    buf = parts.pop() ?? ''

    for (const part of parts) {
      let eventName = ''
      for (const line of part.split('\n')) {
        if (line.startsWith('event: ')) {
          eventName = line.slice(7).trim()
          continue
        }
        if (!line.startsWith('data: ')) continue
        const data = line.slice(6).trim()
        if (!data || data === '[DONE]') continue
        try {
          const payload = JSON.parse(data) as Record<string, unknown>
          if (eventName === 'reasoning_delta') {
            onChunk({ type: 'reasoning', content: String(payload.content ?? payload.delta ?? '') })
          } else if (eventName === 'content_delta') {
            onChunk({ type: 'content', content: String(payload.content ?? payload.delta ?? '') })
          } else if (eventName === 'usage') {
            usage = readUsage(payload)
          } else if (eventName === 'done') {
            onChunk({ type: 'done', ...usage, ...readUsage(payload) })
            return
          } else if (eventName === 'error') {
            onChunk({ type: 'error', message: String(payload.message ?? '流式输出错误') })
            return
          } else if (typeof payload.type === 'string') {
            if (payload.type === 'usage') {
              usage = readUsage(payload)
              continue
            }
            const chunk = payload as unknown as StreamChunk
            if (chunk.type === 'reasoning' || chunk.type === 'content') {
              onChunk({ ...chunk, content: chunk.content ?? String(payload.delta ?? '') })
              continue
            }
            if (chunk.type === 'done') {
              onChunk({ type: 'done', ...usage, ...readUsage(payload) })
              return
            }
            onChunk(chunk)
            if (chunk.type === 'error') return
          }
        } catch { /* 忽略非 JSON 行 */ }
      }
    }
  }
}

export default api
