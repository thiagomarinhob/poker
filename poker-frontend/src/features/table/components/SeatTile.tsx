import { motion } from 'framer-motion'
import type { TableSeatView } from '../../../ws/types'
import { AnimatedCardRow } from './PokerCard'

const TIMER_COLOR = 'rgba(52, 211, 153, 0.95)'

type SeatTileProps = {
  seat: TableSeatView | undefined
  seatIndex: number
  isHero: boolean
  isDealer: boolean
  showTimer: boolean
  timerFraction: number
}

function shortId(id: string | undefined): string {
  if (!id) return 'Vago'
  return id.length <= 10 ? id : `${id.slice(0, 4)}…${id.slice(-4)}`
}

export function SeatTile({
  seat,
  seatIndex,
  isHero,
  isDealer,
  showTimer,
  timerFraction,
}: SeatTileProps) {
  const occupied = Boolean(seat?.player_id)
  const hole = seat?.hole_cards ?? []
  const showCards = !isHero && hole.length > 0

  return (
    <div className="relative flex w-[112px] flex-col items-center gap-1">
      {showTimer && (
        <svg className="pointer-events-none absolute -inset-3 h-[calc(100%+24px)] w-[calc(100%+24px)] -rotate-90" viewBox="0 0 100 100">
          <circle cx="50" cy="50" r="46" fill="none" stroke="rgba(255,255,255,0.08)" strokeWidth="5" />
          <motion.circle
            cx="50"
            cy="50"
            r="46"
            fill="none"
            stroke={TIMER_COLOR}
            strokeWidth="5"
            strokeLinecap="round"
            strokeDasharray={289}
            initial={false}
            animate={{ strokeDashoffset: 289 * (1 - timerFraction) }}
            transition={{ duration: 0.12 }}
          />
        </svg>
      )}
      {isDealer && (
        <span className="absolute -right-0 -top-1 z-10 flex h-6 w-6 items-center justify-center rounded-full border border-white/30 bg-white text-[10px] font-bold text-gray-900 shadow">
          D
        </span>
      )}
      <div
        className={`relative z-0 w-full rounded-xl border px-2 py-2 text-center shadow-lg backdrop-blur-sm transition-colors ${
          isHero
            ? 'border-emerald-400/70 bg-emerald-950/70 ring-1 ring-emerald-500/40'
            : occupied
              ? 'border-white/15 bg-black/45'
              : 'border-white/10 bg-black/25'
        }`}
      >
        <p className="truncate text-[10px] font-mono text-gray-300">{shortId(seat?.player_id)}</p>
        <p className="text-xs font-semibold tabular-nums text-white">{occupied ? seat?.stack ?? 0 : '—'}</p>
        {occupied && (
          <p className="mt-0.5 truncate text-[9px] uppercase tracking-wide text-gray-500">{seat?.status ?? ''}</p>
        )}
        <p className="text-[9px] text-gray-600">#{seatIndex}</p>
      </div>
      {showCards && (
        <div className="scale-90">
          <AnimatedCardRow codes={hole} faceDown={false} muck={false} prefix={`s${seatIndex}`} />
        </div>
      )}
    </div>
  )
}
