import { create } from 'zustand'
import { pokerSocket } from '../ws/client'
import { WS_SERVER, type ActionRequiredPayload, type HandResultPayload, type ServerWsPayload, type TableStateView } from '../ws/types'

export type WsConnectionStatus = 'idle' | 'connecting' | 'open' | 'closed'

function mapReadyState(): WsConnectionStatus {
  const s = pokerSocket.readyState
  if (s === WebSocket.OPEN) return 'open'
  if (s === WebSocket.CONNECTING) return 'connecting'
  return 'closed'
}

interface GameStoreState {
  activeTableId: string | null
  tableState: TableStateView | null
  actionRequired: ActionRequiredPayload | null
  lastHandResult: HandResultPayload | null
  lastWsError: string | null
  wsStatus: WsConnectionStatus
}

interface GameStoreActions {
  setActiveTable: (tableId: string | null) => void
  syncWsStatus: () => void
  applyServerMessage: (msg: ServerWsPayload) => void
  joinCurrentTable: (seat: number, buyIn: number) => void
  leaveCurrentTable: () => void
  sendFold: () => void
  sendCall: () => void
  sendRaise: (raiseTo: number) => void
  sendAllIn: () => void
  clearHandResult: () => void
  clearWsError: () => void
  resetTableSession: () => void
}

export const useGameStore = create<GameStoreState & GameStoreActions>((set, get) => ({
  activeTableId: null,
  tableState: null,
  actionRequired: null,
  lastHandResult: null,
  lastWsError: null,
  wsStatus: 'idle',

  setActiveTable(tableId) {
    set({ activeTableId: tableId })
  },

  syncWsStatus() {
    set({ wsStatus: mapReadyState() })
  },

  applyServerMessage(msg) {
    const tid = get().activeTableId
    switch (msg.type) {
      case WS_SERVER.TableState:
        if (!tid || msg.payload.table_id === tid) {
          set({ tableState: msg.payload, lastWsError: null, wsStatus: mapReadyState() })
        }
        break
      case WS_SERVER.ActionRequired:
        if (tid && msg.payload.table_id === tid) {
          set({ actionRequired: msg.payload })
        }
        break
      case WS_SERVER.HandResult:
        if (tid && msg.payload.table_id === tid) {
          set({ lastHandResult: msg.payload, actionRequired: null })
        }
        break
      case WS_SERVER.Error:
        set({ lastWsError: msg.payload.message })
        break
      case WS_SERVER.PlayerJoined:
      case WS_SERVER.Pong:
      case WS_SERVER.Chat:
      default:
        set({ wsStatus: mapReadyState() })
        break
    }
  },

  joinCurrentTable(seat, buyIn) {
    const tid = get().activeTableId
    if (!tid) return
    pokerSocket.joinTable({ table_id: tid, seat, buy_in: buyIn })
  },

  leaveCurrentTable() {
    const tid = get().activeTableId
    if (!tid) return
    pokerSocket.leaveTable(tid)
    set({
      activeTableId: null,
      tableState: null,
      actionRequired: null,
      lastHandResult: null,
    })
  },

  sendFold() {
    const tid = get().activeTableId
    if (!tid) return
    pokerSocket.sendAction({ table_id: tid, action: 'Fold' })
  },

  sendCall() {
    const tid = get().activeTableId
    if (!tid) return
    const ar = get().actionRequired
    pokerSocket.sendAction({
      table_id: tid,
      action: ar?.can_check ? 'Check' : 'Call',
    })
  },

  sendRaise(raiseTo) {
    const tid = get().activeTableId
    if (!tid) return
    pokerSocket.sendAction({ table_id: tid, action: 'Raise', amount: raiseTo })
  },

  sendAllIn() {
    const tid = get().activeTableId
    if (!tid) return
    pokerSocket.sendAction({ table_id: tid, action: 'AllIn' })
  },

  clearHandResult() {
    set({ lastHandResult: null })
  },

  clearWsError() {
    set({ lastWsError: null })
  },

  resetTableSession() {
    set({
      activeTableId: null,
      tableState: null,
      actionRequired: null,
      lastHandResult: null,
      lastWsError: null,
      wsStatus: mapReadyState(),
    })
  },
}))
