import axios from 'axios'
import { useAuthStore } from '../store/auth'

const api = axios.create({
  baseURL: '/api',
  timeout: 10000,
})

api.interceptors.request.use((config) => {
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

export const getGoogleAuthUrl = async (): Promise<string> => {
  const res = await api.get<ApiResponse<{ auth_url: string }>>('/v1/auth/google/url')
  return res.data.data.auth_url
}

export const getUserMe = async () => {
  const res = await api.get<ApiResponse<{
    user_id: string
    email: string
    name: string
    avatar_url: string
  }>>('/v1/user/me')
  return res.data.data
}

export const postLogout = async (refreshToken: string) => {
  await api.post('/v1/auth/logout', { refresh_token: refreshToken })
}

export default api
