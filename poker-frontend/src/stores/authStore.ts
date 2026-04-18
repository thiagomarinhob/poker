import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { TokenPayload } from '../types'

interface AuthState {
  accessToken: string | null
  refreshToken: string | null
  userId: string | null
  role: 'player' | 'admin' | null

  setTokens: (access: string, refresh: string) => void
  logout: () => void
  isAuthenticated: () => boolean
}

function parseJwt(token: string): TokenPayload | null {
  try {
    const b64 = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    return JSON.parse(atob(b64)) as TokenPayload
  } catch {
    return null
  }
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      accessToken: null,
      refreshToken: null,
      userId: null,
      role: null,

      setTokens(access, refresh) {
        const payload = parseJwt(access)
        set({
          accessToken: access,
          refreshToken: refresh,
          userId: payload?.sub ?? null,
          role: payload?.role ?? null,
        })
      },

      logout() {
        set({ accessToken: null, refreshToken: null, userId: null, role: null })
      },

      isAuthenticated() {
        return get().accessToken !== null
      },
    }),
    {
      name: 'auth',
      // Persist all token state — expired access tokens are handled by the interceptor.
      partialize: (s) => ({
        accessToken: s.accessToken,
        refreshToken: s.refreshToken,
        userId: s.userId,
        role: s.role,
      }),
    },
  ),
)
