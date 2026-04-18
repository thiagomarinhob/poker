import { api } from './client'

/** Resposta de `GET /api/tables` — espelha `table.Handler.List`. */
export interface TableListRow {
  id: string
  name: string
  max_seats: number
  small_blind: number
  big_blind: number
  turn_timeout_seconds: number
  created_at: string
}

export async function fetchTables(): Promise<TableListRow[]> {
  const { data } = await api.get<{ tables: TableListRow[] }>('/tables')
  return data.tables
}
