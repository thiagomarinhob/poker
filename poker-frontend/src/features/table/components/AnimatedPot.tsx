import { animate, motion } from 'framer-motion'
import { useEffect, useRef, useState } from 'react'

export function AnimatedPot({ amount }: { amount: number }) {
  const [display, setDisplay] = useState(amount)
  const fromRef = useRef(amount)

  useEffect(() => {
    const controls = animate(fromRef.current, amount, {
      duration: 0.55,
      ease: [0.22, 1, 0.36, 1],
      onUpdate: (v) => setDisplay(Math.round(v)),
    })
    fromRef.current = amount
    return () => controls.stop()
  }, [amount])

  return (
    <motion.div className="flex flex-col items-center gap-1">
      <div className="relative flex h-10 items-center justify-center gap-0.5">
        {[0, 1, 2, 3, 4].map((i) => (
          <span
            key={i}
            className="h-7 w-7 rounded-full border-2 border-amber-200/90 bg-gradient-to-br from-amber-400 to-amber-700 shadow-md"
            style={{ marginLeft: i === 0 ? 0 : -10 }}
          />
        ))}
      </div>
      <motion.span
        key={amount}
        className="text-lg font-bold tabular-nums tracking-tight text-amber-200 drop-shadow-sm"
        initial={{ scale: 1 }}
        animate={{ scale: [1, 1.08, 1] }}
        transition={{ duration: 0.35 }}
      >
        {display}
      </motion.span>
      <span className="text-[10px] uppercase tracking-[0.2em] text-amber-100/60">pote</span>
    </motion.div>
  )
}
