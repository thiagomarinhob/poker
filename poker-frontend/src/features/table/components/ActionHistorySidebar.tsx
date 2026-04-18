import type { TableLogEntry } from '../hooks/useTableActionLog'

type ActionHistorySidebarProps = {
  entries: TableLogEntry[]
}

export function ActionHistorySidebar({ entries }: ActionHistorySidebarProps) {
  return (
    <aside className="flex h-full min-h-0 w-full flex-col rounded-2xl border border-white/10 bg-black/40 shadow-inner backdrop-blur-sm">
      <div className="border-b border-white/10 px-3 py-2">
        <h3 className="text-[11px] font-semibold uppercase tracking-[0.18em] text-gray-500">Histórico</h3>
      </div>
      <ul className="min-h-0 flex-1 space-y-2 overflow-y-auto p-3 text-xs text-gray-300">
        {entries.length === 0 && <li className="text-gray-600">Aguardando eventos…</li>}
        {[...entries].reverse().map((e) => (
          <li key={e.id} className="rounded-lg border border-white/5 bg-white/5 px-2 py-1.5 leading-snug">
            <span className="mr-2 font-mono text-[10px] text-gray-600">
              {new Date(e.at).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
            </span>
            {e.message}
          </li>
        ))}
      </ul>
    </aside>
  )
}
