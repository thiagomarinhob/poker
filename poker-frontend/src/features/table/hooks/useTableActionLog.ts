import { useEffect, useRef, useState } from 'react'
import type { HandResultPayload, TableStateView } from '../../../ws/types'

export interface TableLogEntry {
  id: string
  at: number
  message: string
}

const MAX = 80

function push(prev: TableLogEntry[], message: string): TableLogEntry[] {
  const entry: TableLogEntry = {
    id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    at: Date.now(),
    message,
  }
  return [...prev, entry].slice(-MAX)
}

/** Histórico derivado de mudanças no estado da mesa (sem feed de ações no WS). */
export function useTableActionLog(
  tableState: TableStateView | null,
  lastHandResult: HandResultPayload | null,
): TableLogEntry[] {
  const [entries, setEntries] = useState<TableLogEntry[]>([])
  const prev = useRef({
    ready: false,
    hand: -1,
    street: '',
    boardLen: 0,
    actionIdx: null as number | null | undefined,
    lastResultId: '',
  })

  useEffect(() => {
    if (!lastHandResult) return
    const rid = `${lastHandResult.hand_id}-${lastHandResult.hand_number}`
    if (prev.current.lastResultId === rid) return
    prev.current.lastResultId = rid
    const names =
      lastHandResult.winner_emails &&
      lastHandResult.winner_emails.length === lastHandResult.winners.length
        ? lastHandResult.winner_emails
        : lastHandResult.winners
    setEntries((e) => push(e, `Fim da mão #${lastHandResult.hand_number} — vencedores: ${names.join(', ') || '—'}`))
  }, [lastHandResult])

  useEffect(() => {
    if (!tableState) {
      prev.current.ready = false
    }
  }, [tableState])

  useEffect(() => {
    if (!tableState) return
    const boardLen = (tableState.board ?? []).length
    const p = prev.current
    if (!p.ready) {
      p.ready = true
      p.hand = tableState.hand_number ?? -1
      p.street = tableState.street
      p.boardLen = boardLen
      p.actionIdx = tableState.action_on_index
      return
    }

    const updates: string[] = []
    const hn = tableState.hand_number ?? -1

    if (hn >= 0 && hn !== p.hand) {
      p.hand = hn
      updates.push(`Nova mão #${hn}`)
    }

    if (tableState.street !== p.street) {
      p.street = tableState.street
      updates.push(`Rua: ${tableState.street} · pote ${tableState.pot}`)
    }

    if (boardLen > p.boardLen) {
      updates.push(`Cartas no board: ${boardLen}`)
      p.boardLen = boardLen
    } else if (boardLen < p.boardLen) {
      updates.push('Board limpo')
      p.boardLen = boardLen
    }

    const ai = tableState.action_on_index
    if (ai !== p.actionIdx) {
      p.actionIdx = ai
      if (ai !== null && ai !== undefined) {
        updates.push(`Ação no assento ${ai}`)
      }
    }

    if (updates.length === 0) return
    setEntries((e) => updates.reduce((acc, m) => push(acc, m), e))
  }, [tableState])

  return entries
}
