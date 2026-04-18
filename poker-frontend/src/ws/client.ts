import { parseServerMessage, WS_CLIENT, type ServerWsPayload } from './types'

const INITIAL_BACKOFF_MS = 1000
const MAX_BACKOFF_MS = 30_000
const BACKOFF_FACTOR = 2

export type ServerMessageListener = (msg: ServerWsPayload) => void

function wsBaseUrl(): string {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${window.location.host}`
}

function buildUrl(token: string): string {
  const q = new URLSearchParams({ token })
  return `${wsBaseUrl()}/api/ws?${q.toString()}`
}

export class PokerSocket {
  private ws: WebSocket | null = null
  /** Token usado para reconectar após quedas. */
  private token: string | null = null
  private intentionalClose = false
  private reconnectAttempt = 0
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private listeners = new Set<ServerMessageListener>()

  subscribe(listener: ServerMessageListener): () => void {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  private emit(msg: ServerWsPayload): void {
    this.listeners.forEach((l) => {
      l(msg)
    })
  }

  /** Conecta (ou mantém conexão) com o token atual; reconexão exponencial se cair. */
  connect(token: string): void {
    this.intentionalClose = false
    if (this.ws?.readyState === WebSocket.OPEN && this.token === token) {
      return
    }
    if (this.ws?.readyState === WebSocket.CONNECTING) {
      return
    }

    this.token = token
    this.clearReconnectTimer()
    this.ws?.close()
    this.openSocket(token)
  }

  private openSocket(token: string): void {
    const socket = new WebSocket(buildUrl(token))
    this.ws = socket

    socket.onmessage = (event) => {
      let data: unknown
      try {
        data = JSON.parse(event.data as string) as unknown
      } catch {
        return
      }
      const msg = parseServerMessage(data)
      if (msg) this.emit(msg)
    }

    socket.onopen = () => {
      this.reconnectAttempt = 0
    }

    socket.onerror = () => {
      // onclose fará backoff
    }

    socket.onclose = () => {
      this.ws = null
      if (this.intentionalClose || !this.token) return
      this.scheduleReconnect()
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer !== null) return
    const exp = Math.min(
      MAX_BACKOFF_MS,
      INITIAL_BACKOFF_MS * BACKOFF_FACTOR ** this.reconnectAttempt,
    )
    const jitter = Math.floor(Math.random() * 400)
    const delay = exp + jitter
    this.reconnectAttempt += 1
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      if (this.token && !this.intentionalClose) {
        this.openSocket(this.token)
      }
    }, delay)
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  send<T extends Record<string, unknown>>(type: string, payload: T): void {
    if (this.ws?.readyState !== WebSocket.OPEN) return
    this.ws.send(JSON.stringify({ type, payload }))
  }

  joinTable(body: { table_id: string; seat: number; buy_in: number }): void {
    this.send(WS_CLIENT.JoinTable, body)
  }

  leaveTable(tableId: string): void {
    this.send(WS_CLIENT.LeaveTable, { table_id: tableId })
  }

  sendAction(body: { table_id: string; action: string; amount?: number }): void {
    this.send(WS_CLIENT.Action, body)
  }

  ping(): void {
    this.send(WS_CLIENT.Ping, {} as Record<string, unknown>)
  }

  /** Encerra o socket e cancela reconexões automáticas. */
  disconnect(): void {
    this.intentionalClose = true
    this.token = null
    this.clearReconnectTimer()
    this.reconnectAttempt = 0
    this.ws?.close()
    this.ws = null
  }

  get readyState(): number {
    return this.ws?.readyState ?? WebSocket.CLOSED
  }

  get isOpen(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }
}

export const pokerSocket = new PokerSocket()
