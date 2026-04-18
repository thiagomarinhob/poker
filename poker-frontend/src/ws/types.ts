/**
 * Espelha `poker-backend/internal/ws/messages.go` e payloads em `hub.go` / `serve.go`.
 * Mantido alinhado manualmente ao Go — alterações no backend exigem atualização aqui.
 */

export const WS_CLIENT = {
  JoinTable: 'JoinTable',
  LeaveTable: 'LeaveTable',
  Action: 'Action',
  Chat: 'Chat',
  Ping: 'Ping',
} as const

export type WsClientType = (typeof WS_CLIENT)[keyof typeof WS_CLIENT]

export const WS_SERVER = {
  TableState: 'TableState',
  PlayerJoined: 'PlayerJoined',
  ActionRequired: 'ActionRequired',
  HandResult: 'HandResult',
  Error: 'Error',
  Pong: 'Pong',
  Chat: 'Chat',
} as const

export type WsServerType = (typeof WS_SERVER)[keyof typeof WS_SERVER]

/** `game.TableStateView` + `table_id` injetado em `tableStatePayload`. */
export interface TableStateView {
  table_id: string
  hand_id?: string
  hand_number?: number
  dealer_seat?: number
  street: string
  board: string[]
  pot: number
  seats: TableSeatView[]
  your_cards?: string[]
  your_player_id?: string
  active_hand_id?: string
  current_bet?: number
  min_raise_to?: number
  action_on_index?: number | null
}

/** `game.TableSeatView` */
export interface TableSeatView {
  index: number
  player_id?: string
  stack: number
  street_bet: number
  total_bet: number
  status: string
  in_hand: boolean
  hole_cards?: string[]
}

export interface JoinTablePayload {
  table_id: string
  seat: number
  buy_in: number
}

export interface LeaveTablePayload {
  table_id: string
}

export type PokerActionName = 'Fold' | 'Check' | 'Call' | 'Bet' | 'Raise' | 'AllIn'

export interface ActionPayload {
  table_id: string
  action: PokerActionName
  amount?: number
}

export interface ChatClientPayload {
  table_id: string
  text: string
}

export interface PlayerJoinedPayload {
  table_id: string
  player_id: string
  seat: number
  joiner_id: string
}

export interface ActionRequiredPayload {
  table_id: string
  hand_id: string
  player_id: string
  to_call: number
  can_check: boolean
  min_raise_to: number
  timeout_ms?: number
}

export interface HandResultPayload {
  table_id: string
  hand_id: string
  hand_number: number
  winners: string[]
}

export interface ErrorPayload {
  code: string
  message: string
}

export interface ChatServerPayload {
  table_id: string
  from_id: string
  text: string
}

export interface PongPayload {
  ts: number
}

export type ServerWsPayload =
  | { type: typeof WS_SERVER.TableState; payload: TableStateView }
  | { type: typeof WS_SERVER.PlayerJoined; payload: PlayerJoinedPayload }
  | { type: typeof WS_SERVER.ActionRequired; payload: ActionRequiredPayload }
  | { type: typeof WS_SERVER.HandResult; payload: HandResultPayload }
  | { type: typeof WS_SERVER.Error; payload: ErrorPayload }
  | { type: typeof WS_SERVER.Pong; payload: PongPayload }
  | { type: typeof WS_SERVER.Chat; payload: ChatServerPayload }

export function parseServerMessage(raw: unknown): ServerWsPayload | null {
  if (!raw || typeof raw !== 'object') return null
  const o = raw as { type?: unknown; payload?: unknown }
  if (typeof o.type !== 'string' || typeof o.payload !== 'object' || o.payload === null) {
    return null
  }
  const p = o.payload as Record<string, unknown>
  switch (o.type) {
    case WS_SERVER.TableState: {
      if (typeof p.table_id !== 'string' || typeof p.street !== 'string' || !Array.isArray(p.seats)) {
        return null
      }
      return { type: WS_SERVER.TableState, payload: p as unknown as TableStateView }
    }
    case WS_SERVER.PlayerJoined:
      if (typeof p.table_id !== 'string' || typeof p.player_id !== 'string') return null
      return { type: WS_SERVER.PlayerJoined, payload: p as unknown as PlayerJoinedPayload }
    case WS_SERVER.ActionRequired:
      if (typeof p.table_id !== 'string' || typeof p.player_id !== 'string') return null
      return { type: WS_SERVER.ActionRequired, payload: p as unknown as ActionRequiredPayload }
    case WS_SERVER.HandResult:
      if (typeof p.table_id !== 'string') return null
      return { type: WS_SERVER.HandResult, payload: p as unknown as HandResultPayload }
    case WS_SERVER.Error:
      if (typeof p.code !== 'string' || typeof p.message !== 'string') return null
      return { type: WS_SERVER.Error, payload: p as unknown as ErrorPayload }
    case WS_SERVER.Pong:
      return { type: WS_SERVER.Pong, payload: p as unknown as PongPayload }
    case WS_SERVER.Chat:
      if (typeof p.table_id !== 'string' || typeof p.text !== 'string') return null
      return { type: WS_SERVER.Chat, payload: p as unknown as ChatServerPayload }
    default:
      return null
  }
}
