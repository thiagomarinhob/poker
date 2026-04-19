import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '../../stores/authStore'
import { useGameStore } from '../../stores/gameStore'
import { pokerSocket } from '../../ws/client'
import { fetchTables } from '../../api/tables'
import { ActionHistorySidebar } from './components/ActionHistorySidebar'
import { BetControls } from './components/BetControls'
import { BoardRail, HeroHoleStrip, TableOval } from './components/TableOval'
import { useActionDeadline } from './hooks/useActionDeadline'
import { useTableActionLog } from './hooks/useTableActionLog'
import { useSoundEnabledState, useTableSounds } from './hooks/useTableSounds'

export function TablePage() {
  const { id: tableId } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const accessToken = useAuthStore((s) => s.accessToken)

  const [seat, setSeat] = useState(0)
  const [buyIn, setBuyIn] = useState(2000)
  const [raiseTo, setRaiseTo] = useState(0)
  const [soundOn, setSoundOn] = useSoundEnabledState()

  const { data: tables } = useQuery({ queryKey: ['tables'], queryFn: fetchTables })
  const meta = useMemo(() => tables?.find((t) => t.id === tableId), [tables, tableId])

  const setActiveTable = useGameStore((s) => s.setActiveTable)
  const applyServerMessage = useGameStore((s) => s.applyServerMessage)
  const syncWsStatus = useGameStore((s) => s.syncWsStatus)
  const joinCurrentTable = useGameStore((s) => s.joinCurrentTable)
  const resetTableSession = useGameStore((s) => s.resetTableSession)
  const sendFold = useGameStore((s) => s.sendFold)
  const sendCall = useGameStore((s) => s.sendCall)
  const sendRaise = useGameStore((s) => s.sendRaise)
  const sendAllIn = useGameStore((s) => s.sendAllIn)
  const clearHandResult = useGameStore((s) => s.clearHandResult)
  const clearWsError = useGameStore((s) => s.clearWsError)

  const tableState = useGameStore((s) => s.tableState)
  const actionRequired = useGameStore((s) => s.actionRequired)
  const lastHandResult = useGameStore((s) => s.lastHandResult)
  const lastWsError = useGameStore((s) => s.lastWsError)
  const wsStatus = useGameStore((s) => s.wsStatus)

  const actionLog = useTableActionLog(tableState, lastHandResult)
  const { playYourTurn, playStreet, playHandEnd, lastTurnKey } = useTableSounds(soundOn)

  useEffect(() => {
    if (meta) {
      setBuyIn(Math.max(200, meta.big_blind * 100))
      setRaiseTo(meta.big_blind * 2)
    }
  }, [meta])

  useEffect(() => {
    if (actionRequired) {
      setRaiseTo(actionRequired.min_raise_to)
    }
  }, [actionRequired])

  useEffect(() => {
    if (!tableId || !accessToken) {
      navigate('/login', { replace: true })
      return
    }

    setActiveTable(tableId)
    pokerSocket.connect(accessToken)
    const off = pokerSocket.subscribe((msg) => {
      applyServerMessage(msg)
    })
    const t1 = window.setTimeout(() => syncWsStatus(), 200)
    const t2 = window.setTimeout(() => syncWsStatus(), 2000)

    return () => {
      off()
      window.clearTimeout(t1)
      window.clearTimeout(t2)
      pokerSocket.leaveTable(tableId)
      resetTableSession()
    }
  }, [tableId, accessToken, navigate, setActiveTable, applyServerMessage, syncWsStatus, resetTableSession])

  const handleJoin = useCallback(() => {
    clearWsError()
    joinCurrentTable(seat, buyIn)
  }, [joinCurrentTable, seat, buyIn, clearWsError])

  const yourId = tableState?.your_player_id
  const isYourTurn =
    actionRequired &&
    yourId &&
    actionRequired.player_id === yourId &&
    tableId === actionRequired.table_id

  const turnResetKey = actionRequired
    ? `${actionRequired.hand_id}-${actionRequired.player_id}-${actionRequired.min_raise_to}-${actionRequired.to_call}`
    : ''
  const timerFraction = useActionDeadline(actionRequired?.timeout_ms, turnResetKey)

  const timerSeatIndex =
    actionRequired && tableState?.action_on_index !== undefined && tableState?.action_on_index !== null
      ? tableState.action_on_index
      : null

  const prevStreet = useRef('')
  useEffect(() => {
    if (!tableState?.street) return
    if (prevStreet.current && prevStreet.current !== tableState.street) {
      playStreet()
    }
    prevStreet.current = tableState.street
  }, [tableState?.street, playStreet])

  const lastHandSound = useRef('')
  useEffect(() => {
    if (!lastHandResult) return
    const id = `${lastHandResult.hand_id}-${lastHandResult.hand_number}`
    if (lastHandSound.current === id) return
    lastHandSound.current = id
    playHandEnd()
  }, [lastHandResult, playHandEnd])

  const [handResultSecondsLeft, setHandResultSecondsLeft] = useState<number | null>(null)
  useEffect(() => {
    if (!lastHandResult) {
      setHandResultSecondsLeft(null)
      return
    }
    setHandResultSecondsLeft(5)
    let left = 5
    const tick = window.setInterval(() => {
      left -= 1
      setHandResultSecondsLeft(left)
      if (left <= 0) {
        window.clearInterval(tick)
        clearHandResult()
      }
    }, 1000)
    return () => window.clearInterval(tick)
  }, [lastHandResult?.hand_id, lastHandResult?.hand_number, clearHandResult])

  useEffect(() => {
    if (!actionRequired || !yourId) return
    if (actionRequired.player_id !== yourId) return
    const k = `${actionRequired.hand_id}-${actionRequired.player_id}-${actionRequired.min_raise_to}-${actionRequired.to_call}`
    if (lastTurnKey.current === k) return
    lastTurnKey.current = k
    playYourTurn()
  }, [actionRequired, yourId, playYourTurn, lastTurnKey])

  if (!tableId) {
    return null
  }

  const maxSeats = meta?.max_seats ?? 9

  const isSeated = Boolean(yourId && tableState?.seats?.some((s) => s.player_id === yourId))
  const showJoinPanel = !isSeated || !pokerSocket.isOpen

  const winnerDisplayList =
    lastHandResult == null
      ? []
      : lastHandResult.winner_emails &&
          lastHandResult.winner_emails.length === lastHandResult.winners.length
        ? lastHandResult.winner_emails
        : lastHandResult.winners

  return (
    <div className="flex min-h-screen flex-col bg-[#070a0c] text-white">
      <header className="shrink-0 border-b border-white/10 bg-black/50 backdrop-blur-sm">
        <div className="mx-auto flex h-14 max-w-7xl items-center justify-between gap-4 px-4">
          <button type="button" onClick={() => navigate('/lobby')} className="text-sm text-gray-400 hover:text-white">
            ← Lobby
          </button>
          <div className="flex items-center gap-4">
            <label className="flex cursor-pointer items-center gap-2 text-xs text-gray-400">
              <input type="checkbox" checked={soundOn} onChange={(e) => setSoundOn(e.target.checked)} className="accent-amber-500" />
              Som
            </label>
            <span className="hidden font-mono text-[10px] text-gray-500 sm:inline">
              WS {wsStatus} · {meta?.name ?? tableId.slice(0, 8)}
            </span>
          </div>
        </div>
      </header>

      <div className="mx-auto flex min-h-0 w-full max-w-7xl flex-1 flex-col gap-4 p-4 lg:flex-row">
        <div className="min-w-0 flex-1 space-y-4 overflow-y-auto pb-8">
          {lastWsError && (
            <div className="flex justify-between gap-4 rounded-xl border border-red-900/50 bg-red-950/40 px-4 py-3 text-sm text-red-200">
              <span>{lastWsError}</span>
              <button type="button" className="shrink-0 text-red-400 hover:text-red-200" onClick={clearWsError}>
                Fechar
              </button>
            </div>
          )}

          {lastHandResult && (
            <div className="flex justify-between gap-4 rounded-xl border border-emerald-900/40 bg-emerald-950/30 px-4 py-3 text-sm text-emerald-100">
              <span>
                Mão #{lastHandResult.hand_number} — vencedores: {winnerDisplayList.join(', ') || '—'}
                {handResultSecondsLeft !== null && handResultSecondsLeft > 0 && (
                  <span className="ml-2 text-emerald-300/90">· próxima em {handResultSecondsLeft}s</span>
                )}
              </span>
              <button type="button" className="shrink-0 text-emerald-400 hover:text-emerald-200" onClick={clearHandResult}>
                OK
              </button>
            </div>
          )}

          {showJoinPanel && (
            <section className="rounded-2xl border border-white/10 bg-white/5 p-5 shadow-lg backdrop-blur-sm">
              <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-gray-400">Entrar na mesa</h2>
              <div className="flex flex-wrap items-end gap-4">
                <label className="flex flex-col gap-1 text-xs text-gray-500">
                  Assento
                  <select
                    value={seat}
                    onChange={(e) => setSeat(Number(e.target.value))}
                    className="rounded-lg border border-white/15 bg-black/40 px-3 py-2 text-sm text-white"
                  >
                    {Array.from({ length: maxSeats }, (_, i) => (
                      <option key={i} value={i}>
                        {i}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="flex flex-col gap-1 text-xs text-gray-500">
                  Buy-in (fichas)
                  <input
                    type="number"
                    min={1}
                    value={buyIn}
                    onChange={(e) => setBuyIn(Number(e.target.value))}
                    className="w-32 rounded-lg border border-white/15 bg-black/40 px-3 py-2 text-sm text-white"
                  />
                </label>
                <button
                  type="button"
                  onClick={handleJoin}
                  disabled={!pokerSocket.isOpen}
                  className="rounded-xl bg-emerald-600 px-4 py-2 text-sm font-semibold hover:bg-emerald-500 disabled:opacity-40"
                >
                  Sentar / reconectar
                </button>
              </div>
              {!pokerSocket.isOpen && <p className="mt-2 text-xs text-amber-400">Aguardando WebSocket…</p>}
            </section>
          )}

          {tableState && (
            <>
              <TableOval
                tableState={tableState}
                yourId={yourId}
                timerSeatIndex={timerSeatIndex}
                timerFraction={timerFraction}
                hasActionClock={Boolean(actionRequired)}
                centerBoard={<BoardRail board={tableState.board} />}
                heroCards={
                  <HeroHoleStrip
                    heroCodes={tableState.your_cards ?? []}
                    handKey={tableState.hand_id ?? tableState.active_hand_id ?? ''}
                  />
                }
              />
              {isYourTurn && actionRequired && (
                <BetControls
                  tableState={tableState}
                  actionRequired={actionRequired}
                  raiseTo={raiseTo}
                  setRaiseTo={setRaiseTo}
                  onFold={() => sendFold()}
                  onCall={() => sendCall()}
                  onRaise={() => sendRaise(raiseTo)}
                  onAllIn={() => sendAllIn()}
                />
              )}
            </>
          )}
        </div>

        <div className="h-64 shrink-0 lg:h-auto lg:w-80 lg:min-w-[280px]">
          <ActionHistorySidebar entries={actionLog} />
        </div>
      </div>
    </div>
  )
}
