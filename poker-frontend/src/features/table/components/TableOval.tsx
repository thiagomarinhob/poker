import { useEffect, useState, type ReactNode } from 'react'
import type { TableStateView } from '../../../ws/types'
import { AnimatedPot } from './AnimatedPot'
import { AnimatedCardRow } from './PokerCard'
import { SeatTile } from './SeatTile'

export const OVAL_SEAT_COUNT = 6

/** Posições fixas no felt (percentuais do retângulo da mesa). */
const SEAT_POS: { top: string; left: string }[] = [
  { top: '86%', left: '50%' },
  { top: '74%', left: '82%' },
  { top: '38%', left: '92%' },
  { top: '10%', left: '50%' },
  { top: '38%', left: '8%' },
  { top: '74%', left: '18%' },
]

type TableOvalProps = {
  tableState: TableStateView
  yourId: string | undefined
  timerSeatIndex: number | null
  timerFraction: number
  hasActionClock: boolean
  centerBoard: ReactNode
  heroCards: ReactNode
}

export function TableOval({
  tableState,
  yourId,
  timerSeatIndex,
  timerFraction,
  hasActionClock,
  centerBoard,
  heroCards,
}: TableOvalProps) {
  const dealer = tableState.dealer_seat ?? -1

  return (
    <div className="relative mx-auto w-full max-w-4xl px-2">
      <div
        className="relative aspect-[5/3] w-full overflow-visible rounded-[50%] border-[10px] border-amber-950/90 shadow-2xl"
        style={{
          background: 'radial-gradient(ellipse at center, #166534 0%, #0f3d24 45%, #0a2818 100%)',
          boxShadow: 'inset 0 0 80px rgba(0,0,0,0.45), 0 24px 48px rgba(0,0,0,0.55)',
        }}
      >
        <div
          className="pointer-events-none absolute inset-[4%] rounded-[50%] border border-dashed border-white/10"
          style={{ boxShadow: 'inset 0 0 40px rgba(0,0,0,0.2)' }}
        />
        <div className="absolute left-1/2 top-[40%] flex -translate-x-1/2 -translate-y-1/2 flex-col items-center gap-3">
          <AnimatedPot amount={tableState.pot} />
          {centerBoard}
        </div>
        {SEAT_POS.map((pos, i) => {
          const seat = tableState.seats[i]
          const isHero = Boolean(yourId && seat?.player_id === yourId)
          return (
            <div
              key={i}
              className="absolute z-10 -translate-x-1/2 -translate-y-1/2"
              style={{ top: pos.top, left: pos.left }}
            >
              <SeatTile
                seat={seat}
                seatIndex={i}
                isHero={isHero}
                isDealer={dealer === i}
                showTimer={hasActionClock && timerSeatIndex === i}
                timerFraction={timerFraction}
              />
            </div>
          )
        })}
      </div>
      <div className="mt-4 flex justify-center">{heroCards}</div>
    </div>
  )
}

export function BoardRail({ board }: { board: string[] }) {
  return (
    <div className="min-h-[52px]">
      <AnimatedCardRow codes={board} faceDown={false} muck={false} prefix="board" />
    </div>
  )
}

export function HeroHoleStrip({ heroCodes, handKey }: { heroCodes: string[]; handKey: string }) {
  return (
    <div className="rounded-2xl border border-emerald-700/40 bg-emerald-950/40 px-4 py-3 shadow-lg backdrop-blur-sm">
      <p className="mb-2 text-center text-[10px] uppercase tracking-widest text-emerald-200/70">Suas cartas</p>
      <HeroDealtRow heroCodes={heroCodes} handKey={handKey} />
      {heroCodes.length === 0 && (
        <p className="mt-2 text-center text-[10px] text-gray-600">Aguardando a próxima distribuição…</p>
      )}
    </div>
  )
}

function HeroDealtRow({ heroCodes, handKey }: { heroCodes: string[]; handKey: string }) {
  const [faceDown, setFaceDown] = useState(false)

  useEffect(() => {
    if (!heroCodes.length) {
      setFaceDown(false)
      return
    }
    setFaceDown(true)
    const t = window.setTimeout(() => setFaceDown(false), 480)
    return () => window.clearTimeout(t)
  }, [handKey, heroCodes.join('|')])

  return <AnimatedCardRow codes={heroCodes} faceDown={faceDown} muck={false} prefix="hero" />
}
