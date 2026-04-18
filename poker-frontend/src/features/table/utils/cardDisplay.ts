const RANKS = '23456789TJQKA'

export function parseCardCode(code: string): { rank: string; suit: string; suitClass: string } | null {
  if (!code || code.length < 2) return null
  const r = code[0].toUpperCase()
  const s = code[1].toLowerCase()
  const rank = r === 'T' ? '10' : r
  if (!RANKS.includes(r)) return null
  const suitMap: Record<string, { sym: string; cls: string }> = {
    h: { sym: '♥', cls: 'text-rose-400' },
    d: { sym: '♦', cls: 'text-sky-400' },
    c: { sym: '♣', cls: 'text-emerald-300' },
    s: { sym: '♠', cls: 'text-slate-200' },
  }
  const m = suitMap[s]
  if (!m) return null
  return { rank, suit: m.sym, suitClass: m.cls }
}
