import { useCallback, useEffect, useRef, useState } from 'react'

const STORAGE_KEY = 'poker:table-sound'

export function readSoundEnabled(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === '1'
  } catch {
    return false
  }
}

export function writeSoundEnabled(on: boolean): void {
  try {
    localStorage.setItem(STORAGE_KEY, on ? '1' : '0')
  } catch {
    /* ignore */
  }
}

function beep(freq: number, durationMs: number, gain = 0.08): void {
  const Ctx = window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
  if (!Ctx) return
  const ctx = new Ctx()
  const osc = ctx.createOscillator()
  const g = ctx.createGain()
  osc.type = 'sine'
  osc.frequency.value = freq
  g.gain.value = gain
  osc.connect(g)
  g.connect(ctx.destination)
  osc.start()
  window.setTimeout(() => {
    osc.stop()
    void ctx.close()
  }, durationMs)
}

export function useTableSounds(enabled: boolean) {
  const lastTurnKey = useRef<string>('')

  const playYourTurn = useCallback(() => {
    if (!enabled) return
    beep(880, 120, 0.06)
    window.setTimeout(() => beep(1174, 140, 0.05), 100)
  }, [enabled])

  const playStreet = useCallback(() => {
    if (!enabled) return
    beep(523, 90, 0.05)
  }, [enabled])

  const playHandEnd = useCallback(() => {
    if (!enabled) return
    beep(392, 200, 0.06)
  }, [enabled])

  return { playYourTurn, playStreet, playHandEnd, lastTurnKey }
}

export function useSoundEnabledState(): [boolean, (v: boolean) => void] {
  const [on, setOn] = useState(readSoundEnabled)

  useEffect(() => {
    writeSoundEnabled(on)
  }, [on])

  return [on, setOn]
}
