import { AnimatePresence, motion } from 'framer-motion'
import { parseCardCode } from '../utils/cardDisplay'

type PokerCardProps = {
  code: string
  faceDown?: boolean
  className?: string
  dealDelay?: number
  layoutId?: string
}

export function PokerCard({ code, faceDown, className = '', dealDelay = 0, layoutId }: PokerCardProps) {
  const parsed = parseCardCode(code)

  return (
    <motion.div
      layoutId={layoutId}
      initial={{ opacity: 0, y: -36, scale: 0.85, rotateZ: -6 }}
      animate={{
        opacity: 1,
        y: 0,
        scale: 1,
        rotateZ: 0,
        transition: { type: 'spring', stiffness: 420, damping: 28, delay: dealDelay * 0.07 },
      }}
      exit={{ opacity: 0, y: 28, scale: 0.75, rotateZ: 8, transition: { duration: 0.35 } }}
      className={`relative h-14 w-10 sm:h-16 sm:w-11 select-none ${className}`}
      style={{ perspective: 900 }}
    >
      <motion.div
        className="relative h-full w-full"
        animate={{ rotateY: faceDown ? 180 : 0 }}
        transition={{ type: 'spring', stiffness: 280, damping: 24 }}
        style={{ transformStyle: 'preserve-3d' }}
      >
        <div
          className="absolute inset-0 rounded-md border border-white/25 bg-white text-gray-900 shadow-lg overflow-hidden"
          style={{ backfaceVisibility: 'hidden', WebkitBackfaceVisibility: 'hidden' }}
        >
          {parsed ? (
            <div className="flex h-full flex-col items-center justify-center leading-none">
              <span className="text-sm sm:text-base font-bold font-mono">{parsed.rank}</span>
              <span className={`text-lg sm:text-xl ${parsed.suitClass}`}>{parsed.suit}</span>
            </div>
          ) : (
            <span className="flex h-full items-center justify-center text-[10px] text-gray-500 font-mono">{code}</span>
          )}
        </div>
        <div
          className="absolute inset-0 rounded-md border border-indigo-900/80 bg-gradient-to-br from-indigo-950 via-slate-900 to-indigo-950 shadow-lg"
          style={{
            transform: 'rotateY(180deg)',
            backfaceVisibility: 'hidden',
            WebkitBackfaceVisibility: 'hidden',
          }}
        >
          <div
            className="absolute inset-1 rounded border border-amber-700/40 opacity-80"
            style={{
              backgroundImage: `repeating-linear-gradient(
                45deg,
                transparent,
                transparent 3px,
                rgba(180, 120, 60, 0.12) 3px,
                rgba(180, 120, 60, 0.12) 6px
              )`,
            }}
          />
          <span className="absolute bottom-1 left-0 right-0 text-center text-[8px] font-semibold tracking-widest text-amber-200/70">
            POKER
          </span>
        </div>
      </motion.div>
    </motion.div>
  )
}

type CardRowProps = {
  codes: string[]
  faceDown?: boolean
  muck?: boolean
  prefix?: string
}

export function AnimatedCardRow({ codes, faceDown, muck, prefix = 'c' }: CardRowProps) {
  return (
    <div className="flex gap-1.5 justify-center">
      <AnimatePresence mode="popLayout">
        {!muck &&
          codes.map((c, i) => (
            <PokerCard key={`${prefix}-${i}-${c}`} code={c} faceDown={faceDown} dealDelay={i} layoutId={`${prefix}-${i}`} />
          ))}
      </AnimatePresence>
    </div>
  )
}
