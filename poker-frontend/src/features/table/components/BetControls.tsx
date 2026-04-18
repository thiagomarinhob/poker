import type { ActionRequiredPayload, TableStateView } from '../../../ws/types'

type BetControlsProps = {
  tableState: TableStateView
  actionRequired: ActionRequiredPayload
  raiseTo: number
  setRaiseTo: (n: number) => void
  onFold: () => void
  onCall: () => void
  onRaise: () => void
  onAllIn: () => void
}

export function BetControls({
  tableState,
  actionRequired,
  raiseTo,
  setRaiseTo,
  onFold,
  onCall,
  onRaise,
  onAllIn,
}: BetControlsProps) {
  const heroSeat = tableState.seats.find((s) => s.player_id === tableState.your_player_id)
  const minR = actionRequired.min_raise_to
  const maxR = heroSeat ? heroSeat.stack + heroSeat.street_bet : minR
  const pot = tableState.pot
  const clamp = (n: number) => Math.min(maxR, Math.max(minR, Math.round(n)))

  const halfPotTarget = clamp(minR + Math.floor(pot / 2))
  const potTarget = clamp(minR + pot)

  const sliderMin = minR
  const sliderMax = Math.max(minR, maxR)

  return (
    <div className="rounded-2xl border border-white/10 bg-black/55 p-4 shadow-xl backdrop-blur-md">
      <div className="mb-3 flex flex-wrap gap-2 text-xs text-gray-400">
        <span>Pagar: {actionRequired.to_call}</span>
        <span>·</span>
        <span>{actionRequired.can_check ? 'Pode check' : 'Sem check'}</span>
        <span>·</span>
        <span>
          Raise mín.: {minR} · máx.: {maxR}
        </span>
      </div>
      <div className="mb-4 flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => setRaiseTo(halfPotTarget)}
          className="rounded-lg border border-white/15 bg-white/5 px-3 py-2 text-xs font-medium text-gray-200 hover:bg-white/10"
        >
          ½ pot
        </button>
        <button
          type="button"
          onClick={() => setRaiseTo(potTarget)}
          className="rounded-lg border border-white/15 bg-white/5 px-3 py-2 text-xs font-medium text-gray-200 hover:bg-white/10"
        >
          Pot
        </button>
        <button
          type="button"
          onClick={() => setRaiseTo(maxR)}
          className="rounded-lg border border-amber-600/50 bg-amber-900/30 px-3 py-2 text-xs font-medium text-amber-100 hover:bg-amber-900/50"
        >
          Máx
        </button>
      </div>
      <label className="mb-4 flex flex-col gap-2">
        <span className="text-[11px] uppercase tracking-wider text-gray-500">Raise até</span>
        <input
          type="range"
          min={sliderMin}
          max={sliderMax}
          step={1}
          value={Math.min(sliderMax, Math.max(sliderMin, raiseTo))}
          onChange={(e) => setRaiseTo(Number(e.target.value))}
          className="h-2 w-full cursor-pointer accent-amber-500"
        />
        <div className="flex justify-between text-[11px] font-mono text-gray-500">
          <span>{sliderMin}</span>
          <span className="text-amber-200">{raiseTo}</span>
          <span>{sliderMax}</span>
        </div>
      </label>
      <div className="flex flex-wrap gap-2">
        <button type="button" onClick={onFold} className="rounded-xl bg-red-900/70 px-4 py-2.5 text-sm font-semibold hover:bg-red-800/80">
          Fold
        </button>
        <button
          type="button"
          onClick={onCall}
          className="rounded-xl bg-slate-700 px-4 py-2.5 text-sm font-semibold hover:bg-slate-600"
        >
          {actionRequired.can_check ? 'Check' : 'Call'}
        </button>
        <button type="button" onClick={onRaise} className="rounded-xl bg-amber-600 px-4 py-2.5 text-sm font-semibold hover:bg-amber-500">
          Raise
        </button>
        <button type="button" onClick={onAllIn} className="rounded-xl bg-violet-800/80 px-4 py-2.5 text-sm font-semibold hover:bg-violet-700">
          All-in
        </button>
      </div>
    </div>
  )
}
