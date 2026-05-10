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
  type: 'llm' | 'image'
  base_url: string
  default_model: string
  enabled: boolean
  created_at: number
}

export const modelListProviders = async (): Promise<ModelProvider[]> => {
  const res = await api.get<ApiResponse<ModelProvider[]>>('/v1/admin/models/providers')
  return res.data.data ?? []
}

export const modelCreateProvider = async (data: {
  name: string
  slug: string
  type: 'llm' | 'image'
  base_url: string
  api_key: string
  default_model: string
}): Promise<{ id: string }> => {
  const res = await api.post<ApiResponse<{ id: string }>>('/v1/admin/models/providers', data)
  return res.data.data
}

export const modelSetEnabled = async (id: string, enabled: boolean) => {
  await api.patch(`/v1/admin/models/providers/${id}/enabled`, { enabled })
}

export const modelUpdateAPIKey = async (id: string, apiKey: string) => {
  await api.patch(`/v1/admin/models/providers/${id}/api_key`, { api_key: apiKey })
}

export const modelChat = async (slug: string, messages: { role: string; content: string }[]): Promise<string> => {
  const res = await chatApi.post<ApiResponse<{ content: string }>>('/v1/admin/models/chat', { slug, messages })
  return res.data.data.content
}

// SSE stream chunk types
export type StreamChunkType = 'reasoning' | 'content' | 'done' | 'error'
export interface StreamChunk {
  type: StreamChunkType
  content?: string
  message?: string
}

/**
 * modelChatStream — 使用 fetch + ReadableStream 消费 SSE 流式输出。
 * @param slug      provider slug
 * @param messages  对话消息列表
 * @param onChunk   每收到一个 chunk 的回调
 * @param signal    AbortSignal，用于取消请求
 */
export const modelChatStream = async (
  slug: string,
  messages: { role: string; content: string }[],
  onChunk: (chunk: StreamChunk) => void,
  signal?: AbortSignal,
): Promise<void> => {
  const token = (await import('../store/auth')).useAuthStore.getState().accessToken
  const resp = await fetch('/api/v1/admin/models/chat/stream', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ slug, messages }),
    signal,
  })

  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}))
    throw new Error(body?.message ?? `HTTP ${resp.status}`)
  }

  const reader = resp.body!.getReader()
  const decoder = new TextDecoder()
  let buf = ''

  while (true) {
    const { value, done } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })

    // SSE 格式：每个事件以 "\n\n" 结尾
    const parts = buf.split('\n\n')
    buf = parts.pop() ?? ''

    for (const part of parts) {
      for (const line of part.split('\n')) {
        if (!line.startsWith('data: ')) continue
        const data = line.slice(6).trim()
        if (!data || data === '[DONE]') continue
        try {
          const chunk: StreamChunk = JSON.parse(data)
          onChunk(chunk)
          if (chunk.type === 'done' || chunk.type === 'error') return
        } catch { /* 忽略非 JSON 行 */ }
      }
    }
  }
}

export default api
