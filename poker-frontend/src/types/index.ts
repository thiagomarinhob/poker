export interface AuthResponse {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
}

export interface TokenPayload {
  sub: string
  role: 'player' | 'admin'
  iat: number
  exp: number
}

export interface User {
  id: string
  email: string
  role: 'player' | 'admin'
  chipsBalance: number
  createdAt: string
  updatedAt: string
}

/** Estado de mesa em tempo real — ver `src/ws/types.ts` (espelha `game.TableStateView`). */
export type { TableStateView, TableSeatView } from '../ws/types'
