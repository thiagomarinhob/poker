import { useEffect, useMemo, useState } from 'react'

/** Progresso 0 = esgotado, 1 = cheio — para anel de tempo da vez. */
export function useActionDeadline(timeoutMs: number | undefined, resetKey: string): number {
  const [fraction, setFraction] = useState(1)

  const total = useMemo(() => {
    const t = timeoutMs ?? 30_000
    return t > 0 ? t : 30_000
  }, [timeoutMs])

  useEffect(() => {
    if (!resetKey) {
      setFraction(1)
      return
    }
    const started = performance.now()
    const tick = () => {
      const elapsed = performance.now() - started
      const next = Math.max(0, 1 - elapsed / total)
      setFraction(next)
    }
    tick()
    const id = window.setInterval(tick, 80)
    return () => window.clearInterval(id)
  }, [resetKey, total])

  return fraction
}
