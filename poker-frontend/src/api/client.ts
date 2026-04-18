import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '../stores/authStore'
import type { AuthResponse } from '../types'

interface RetryConfig extends InternalAxiosRequestConfig {
  _retry?: boolean
}

// Queue of callbacks waiting for a refresh to complete.
let isRefreshing = false
let waitQueue: Array<{ resolve: (token: string) => void; reject: (err: unknown) => void }> = []

function flushQueue(token: string | null, err?: unknown) {
  waitQueue.forEach((p) => (token ? p.resolve(token) : p.reject(err)))
  waitQueue = []
}

export const api = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
})

// Attach Bearer token to every outgoing request.
api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// On 401: try to refresh once, queue concurrent requests, logout on failure.
api.interceptors.response.use(
  (res) => res,
  async (err: AxiosError) => {
    const original = err.config as RetryConfig | undefined

    const isRefreshEndpoint = original?.url?.includes('/auth/refresh')
    const shouldRetry =
      original && err.response?.status === 401 && !original._retry && !isRefreshEndpoint

    if (!shouldRetry) return Promise.reject(err)

    // Mark all queued requests too — only one refresh cycle per burst of 401s.
    original._retry = true

    if (isRefreshing) {
      return new Promise<string>((resolve, reject) => waitQueue.push({ resolve, reject })).then(
        (token) => {
          original.headers!.Authorization = `Bearer ${token}`
          return api(original)
        },
      )
    }

    isRefreshing = true
    const { refreshToken, setTokens, logout } = useAuthStore.getState()

    if (!refreshToken) {
      isRefreshing = false
      flushQueue(null, err)
      logout()
      return Promise.reject(err)
    }

    try {
      const { data } = await api.post<AuthResponse>('/auth/refresh', {
        refresh_token: refreshToken,
      })
      setTokens(data.access_token, data.refresh_token)
      flushQueue(data.access_token)
      original.headers!.Authorization = `Bearer ${data.access_token}`
      return api(original)
    } catch (refreshErr) {
      flushQueue(null, refreshErr)
      logout()
      return Promise.reject(refreshErr)
    } finally {
      isRefreshing = false
    }
  },
)

export function getApiError(err: unknown): string {
  if (axios.isAxiosError(err) && err.response?.data?.error) {
    return err.response.data.error as string
  }
  return 'Something went wrong. Please try again.'
}
