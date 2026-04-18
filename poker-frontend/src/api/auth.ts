import { api } from './client'
import type { AuthResponse } from '../types'

export function register(email: string, password: string) {
  return api.post<AuthResponse>('/auth/register', { email, password }).then((r) => r.data)
}

export function login(email: string, password: string) {
  return api.post<AuthResponse>('/auth/login', { email, password }).then((r) => r.data)
}

export function refreshTokens(refreshToken: string) {
  return api.post<AuthResponse>('/auth/refresh', { refresh_token: refreshToken }).then((r) => r.data)
}

export function logout(refreshToken: string) {
  return api.post('/auth/logout', { refresh_token: refreshToken })
}
