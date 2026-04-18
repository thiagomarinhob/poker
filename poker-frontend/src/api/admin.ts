import { api } from './client'

export interface AdminDashboard {
  connected_users_ws: number
  tables_running: number
  hands_today: number
}

export interface AdminTableRow {
  id: string
  name: string
  max_seats: number
  small_blind: number
  big_blind: number
  turn_timeout_seconds: number
  created_at: string
}

export interface CreateTablePayload {
  name: string
  max_seats: number
  small_blind: number
  big_blind: number
  turn_timeout_seconds: number
}

export interface AdminUserRow {
  id: string
  email: string
  role: string
  chips_balance: number
  created_at: string
  updated_at: string
}

export interface AdminHandRow {
  id: string
  room_id: string
  hand_number: number
  dealer_seat: number
  small_blind: number
  big_blind: number
  status: string
  pot_total?: number
  created_at: string
  completed_at: string
}

export interface AdminHandAction {
  id: number
  hand_id: string
  action_seq: number
  table_seat?: number
  hand_player_index?: number
  user_id?: string
  action_type: string
  amount?: number
  street: string
  is_timeout: boolean
  created_at: string
}

export async function fetchAdminDashboard(): Promise<AdminDashboard> {
  const { data } = await api.get<AdminDashboard>('/admin/dashboard')
  return data
}

export async function fetchAdminTables(): Promise<AdminTableRow[]> {
  const { data } = await api.get<{ tables: AdminTableRow[] }>('/admin/tables')
  return data.tables
}

export type AdminCreatedTable = Pick<
  AdminTableRow,
  'id' | 'name' | 'max_seats' | 'small_blind' | 'big_blind' | 'turn_timeout_seconds'
>

export async function createAdminTable(payload: CreateTablePayload): Promise<AdminCreatedTable> {
  const { data } = await api.post<AdminCreatedTable>('/admin/tables', payload)
  return data
}

export async function updateAdminTable(
  id: string,
  payload: CreateTablePayload,
): Promise<{ id: string; updated: boolean }> {
  const { data } = await api.put<{ id: string; updated: boolean }>(`/admin/tables/${id}`, payload)
  return data
}

export async function deleteAdminTable(id: string): Promise<void> {
  await api.delete(`/admin/tables/${id}`)
}

export async function fetchAdminUsers(params: {
  limit?: number
  offset?: number
  q?: string
}): Promise<AdminUserRow[]> {
  const { data } = await api.get<{ users: AdminUserRow[] }>('/admin/users', { params })
  return data.users
}

export async function adjustUserChips(
  userId: string,
  body: { delta: number; reason: string },
): Promise<{ id: string; chips_balance: number }> {
  const { data } = await api.post<{ id: string; chips_balance: number }>(
    `/admin/users/${userId}/chips`,
    body,
  )
  return data
}

export async function fetchAdminHands(params?: {
  limit?: number
  offset?: number
}): Promise<AdminHandRow[]> {
  const { data } = await api.get<{ hands: AdminHandRow[] }>('/admin/hands', { params })
  return data.hands
}

export async function fetchAdminHandDetail(id: string): Promise<{
  hand: AdminHandRow
  actions: AdminHandAction[]
}> {
  const { data } = await api.get<{ hand: AdminHandRow; actions: AdminHandAction[] }>(
    `/admin/hands/${id}`,
  )
  return data
}
