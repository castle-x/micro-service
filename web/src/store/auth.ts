import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export interface User {
  userId: string
  name: string
  email: string
  avatar: string
}

interface AuthState {
  accessToken: string | null
  refreshToken: string | null
  userId: string | null
  user: User | null
  setAuth: (params: {
    accessToken: string
    refreshToken: string
    userId: string
  }) => void
  setUser: (user: User) => void
  logout: () => void
  isAuthenticated: () => boolean
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      accessToken: null,
      refreshToken: null,
      userId: null,
      user: null,

      setAuth: ({ accessToken, refreshToken, userId }) => {
        set({ accessToken, refreshToken, userId })
      },

      setUser: (user: User) => {
        set({ user })
      },

      logout: () => {
        set({ accessToken: null, refreshToken: null, userId: null, user: null })
      },

      isAuthenticated: () => {
        return !!get().accessToken
      },
    }),
    {
      name: 'auth-storage',
    }
  )
)
