import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import {
  adjustUserChips,
  createAdminTable,
  deleteAdminTable,
  fetchAdminDashboard,
  fetchAdminHandDetail,
  fetchAdminHands,
  fetchAdminTables,
  fetchAdminUsers,
  updateAdminTable,
  type AdminTableRow,
  type AdminUserRow,
  type CreateTablePayload,
} from '../../api/admin'
import { getApiError } from '../../api/client'
import { logout } from '../../api/auth'
import { useAuthStore } from '../../stores/authStore'

const emptyForm: CreateTablePayload = {
  name: '',
  max_seats: 6,
  small_blind: 1,
  big_blind: 2,
  turn_timeout_seconds: 30,
}

export function AdminPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { userId, refreshToken, logout: storeLogout } = useAuthStore()

  const [searchInput, setSearchInput] = useState('')
  const [searchQ, setSearchQ] = useState('')
  useEffect(() => {
    const t = window.setTimeout(() => setSearchQ(searchInput.trim()), 350)
    return () => window.clearTimeout(t)
  }, [searchInput])

  const dashboardQ = useQuery({
    queryKey: ['admin', 'dashboard'],
    queryFn: fetchAdminDashboard,
  })
  const tablesQ = useQuery({
    queryKey: ['admin', 'tables'],
    queryFn: fetchAdminTables,
  })
  const usersQ = useQuery({
    queryKey: ['admin', 'users', searchQ],
    queryFn: () => fetchAdminUsers({ limit: 50, q: searchQ || undefined }),
  })
  const handsQ = useQuery({
    queryKey: ['admin', 'hands'],
    queryFn: () => fetchAdminHands({ limit: 30 }),
  })

  const [form, setForm] = useState<CreateTablePayload>(emptyForm)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [formErr, setFormErr] = useState<string | null>(null)

  const [balanceUser, setBalanceUser] = useState<AdminUserRow | null>(null)
  const [delta, setDelta] = useState('')
  const [reason, setReason] = useState('')
  const [balanceErr, setBalanceErr] = useState<string | null>(null)

  const [handDetailId, setHandDetailId] = useState<string | null>(null)
  const handDetailQ = useQuery({
    queryKey: ['admin', 'hand', handDetailId],
    queryFn: () => fetchAdminHandDetail(handDetailId!),
    enabled: handDetailId !== null,
  })

  const createMut = useMutation({
    mutationFn: createAdminTable,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin', 'tables'] })
      void queryClient.invalidateQueries({ queryKey: ['admin', 'dashboard'] })
      setForm(emptyForm)
      setFormErr(null)
      setEditingId(null)
    },
    onError: (e) => setFormErr(getApiError(e)),
  })

  const updateMut = useMutation({
    mutationFn: ({ id, body }: { id: string; body: CreateTablePayload }) => updateAdminTable(id, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin', 'tables'] })
      void queryClient.invalidateQueries({ queryKey: ['admin', 'dashboard'] })
      setForm(emptyForm)
      setFormErr(null)
      setEditingId(null)
    },
    onError: (e) => setFormErr(getApiError(e)),
  })

  const deleteMut = useMutation({
    mutationFn: deleteAdminTable,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin', 'tables'] })
      void queryClient.invalidateQueries({ queryKey: ['admin', 'dashboard'] })
    },
  })

  const balanceMut = useMutation({
    mutationFn: () =>
      adjustUserChips(balanceUser!.id, {
        delta: Number(delta),
        reason: reason.trim(),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
      setBalanceUser(null)
      setDelta('')
      setReason('')
      setBalanceErr(null)
    },
    onError: (e) => setBalanceErr(getApiError(e)),
  })

  function startEdit(t: AdminTableRow) {
    setEditingId(t.id)
    setForm({
      name: t.name,
      max_seats: t.max_seats,
      small_blind: t.small_blind,
      big_blind: t.big_blind,
      turn_timeout_seconds: t.turn_timeout_seconds,
    })
    setFormErr(null)
  }

  function cancelEdit() {
    setEditingId(null)
    setForm(emptyForm)
    setFormErr(null)
  }

  function submitTable(e: FormEvent) {
    e.preventDefault()
    setFormErr(null)
    if (editingId) {
      updateMut.mutate({ id: editingId, body: form })
    } else {
      createMut.mutate(form)
    }
  }

  const stats = useMemo(
    () => [
      {
        label: 'Usuários com WS ativo',
        value: dashboardQ.data?.connected_users_ws ?? '—',
        sub: 'Conexões WebSocket abertas',
      },
      {
        label: 'Mesas rodando',
        value: dashboardQ.data?.tables_running ?? '—',
        sub: 'Instâncias em memória',
      },
      {
        label: 'Mãos hoje (UTC)',
        value: dashboardQ.data?.hands_today ?? '—',
        sub: 'Registradas no banco',
      },
    ],
    [dashboardQ.data],
  )

  async function handleLogout() {
    if (refreshToken) {
      await logout(refreshToken).catch(() => {})
    }
    storeLogout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="min-h-screen bg-gray-950 text-white">
      <header className="border-b border-gray-800 bg-gray-900">
        <div className="max-w-6xl mx-auto px-4 h-14 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <span className="text-xl select-none">♠</span>
            <span className="font-bold">Poker</span>
            <span className="text-xs px-2 py-0.5 bg-amber-500/20 text-amber-400 rounded-full border border-amber-500/30 font-medium">
              Admin
            </span>
          </div>
          <div className="flex items-center gap-4">
            <button
              type="button"
              onClick={() => navigate('/lobby')}
              className="text-sm text-gray-400 hover:text-white transition-colors"
            >
              ← Lobby
            </button>
            <button
              type="button"
              onClick={() => void handleLogout()}
              className="text-sm text-gray-400 hover:text-white transition-colors"
            >
              Sair
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-4 py-10 space-y-10">
        <div>
          <h2 className="text-2xl font-bold">Painel administrativo</h2>
          <p className="text-gray-500 text-sm mt-1">
            Sessão: <span className="font-mono text-gray-400 text-xs">{userId ?? '—'}</span>
          </p>
        </div>

        <section>
          <h3 className="text-sm font-semibold text-gray-400 uppercase tracking-wide mb-3">
            Visão geral
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            {stats.map((s) => (
              <div
                key={s.label}
                className="bg-gray-900 rounded-2xl border border-gray-800 p-5"
              >
                <div className="text-2xl font-bold text-white">{s.value}</div>
                <div className="text-xs text-gray-500 mt-1">{s.label}</div>
                <div className="text-[11px] text-gray-600 mt-0.5">{s.sub}</div>
              </div>
            ))}
          </div>
          {dashboardQ.isError && (
            <p className="text-red-400 text-sm mt-2">{getApiError(dashboardQ.error)}</p>
          )}
        </section>

        <section className="grid lg:grid-cols-2 gap-8">
          <div>
            <h3 className="text-sm font-semibold text-gray-400 uppercase tracking-wide mb-3">
              Mesas {editingId ? `(editando ${editingId.slice(0, 8)}…)` : '(nova)'}
            </h3>
            <form
              onSubmit={submitTable}
              className="bg-gray-900 rounded-2xl border border-gray-800 p-4 space-y-3 mb-4"
            >
              <div className="grid grid-cols-2 gap-2">
                <label className="col-span-2 text-xs text-gray-500">
                  Nome
                  <input
                    className="mt-1 w-full rounded-lg bg-gray-950 border border-gray-700 px-2 py-1.5 text-sm"
                    value={form.name}
                    onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                    required
                  />
                </label>
                <label className="text-xs text-gray-500">
                  Max assentos
                  <input
                    type="number"
                    min={2}
                    max={10}
                    className="mt-1 w-full rounded-lg bg-gray-950 border border-gray-700 px-2 py-1.5 text-sm"
                    value={form.max_seats}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, max_seats: Number(e.target.value) }))
                    }
                  />
                </label>
                <label className="text-xs text-gray-500">
                  Timeout (s)
                  <input
                    type="number"
                    min={5}
                    max={600}
                    className="mt-1 w-full rounded-lg bg-gray-950 border border-gray-700 px-2 py-1.5 text-sm"
                    value={form.turn_timeout_seconds}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        turn_timeout_seconds: Number(e.target.value),
                      }))
                    }
                  />
                </label>
                <label className="text-xs text-gray-500">
                  SB
                  <input
                    type="number"
                    min={1}
                    className="mt-1 w-full rounded-lg bg-gray-950 border border-gray-700 px-2 py-1.5 text-sm"
                    value={form.small_blind}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, small_blind: Number(e.target.value) }))
                    }
                  />
                </label>
                <label className="text-xs text-gray-500">
                  BB
                  <input
                    type="number"
                    min={1}
                    className="mt-1 w-full rounded-lg bg-gray-950 border border-gray-700 px-2 py-1.5 text-sm"
                    value={form.big_blind}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, big_blind: Number(e.target.value) }))
                    }
                  />
                </label>
              </div>
              {formErr && <p className="text-red-400 text-xs">{formErr}</p>}
              <div className="flex gap-2">
                <button
                  type="submit"
                  disabled={createMut.isPending || updateMut.isPending}
                  className="px-4 py-2 rounded-lg bg-amber-600 hover:bg-amber-500 text-sm font-medium disabled:opacity-50"
                >
                  {editingId ? 'Salvar mesa' : 'Criar mesa'}
                </button>
                {editingId && (
                  <button
                    type="button"
                    onClick={cancelEdit}
                    className="px-4 py-2 rounded-lg border border-gray-600 text-sm text-gray-300"
                  >
                    Cancelar
                  </button>
                )}
              </div>
            </form>

            <div className="rounded-2xl border border-gray-800 overflow-hidden">
              <table className="w-full text-sm">
                <thead className="bg-gray-900 text-gray-500 text-left">
                  <tr>
                    <th className="px-3 py-2 font-medium">Nome</th>
                    <th className="px-3 py-2 font-medium">Blinds</th>
                    <th className="px-3 py-2 font-medium w-28"></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-800 bg-gray-900/50">
                  {tablesQ.data?.map((t) => (
                    <tr key={t.id} className="text-gray-300">
                      <td className="px-3 py-2">{t.name}</td>
                      <td className="px-3 py-2 font-mono text-xs">
                        {t.small_blind}/{t.big_blind}
                      </td>
                      <td className="px-3 py-2 flex gap-1">
                        <button
                          type="button"
                          onClick={() => startEdit(t)}
                          className="text-amber-400 text-xs hover:underline"
                        >
                          Editar
                        </button>
                        <button
                          type="button"
                          disabled={deleteMut.isPending}
                          onClick={() => {
                            if (
                              window.confirm(
                                `Remover mesa "${t.name}"? Só é permitido se estiver vazia.`,
                              )
                            ) {
                              deleteMut.mutate(t.id)
                            }
                          }}
                          className="text-red-400 text-xs hover:underline disabled:opacity-50"
                        >
                          Excluir
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {tablesQ.isError && (
                <p className="p-3 text-red-400 text-xs">{getApiError(tablesQ.error)}</p>
              )}
            </div>
          </div>

          <div>
            <h3 className="text-sm font-semibold text-gray-400 uppercase tracking-wide mb-3">
              Usuários
            </h3>
            <input
              type="search"
              placeholder="Buscar por e-mail…"
              className="w-full mb-3 rounded-xl bg-gray-900 border border-gray-700 px-3 py-2 text-sm"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
            <div className="rounded-2xl border border-gray-800 overflow-hidden max-h-96 overflow-y-auto">
              <table className="w-full text-sm">
                <thead className="bg-gray-900 text-gray-500 text-left sticky top-0">
                  <tr>
                    <th className="px-3 py-2 font-medium">E-mail</th>
                    <th className="px-3 py-2 font-medium">Saldo</th>
                    <th className="px-3 py-2 font-medium"></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-800 bg-gray-900/50">
                  {usersQ.data?.map((u) => (
                    <tr key={u.id} className="text-gray-300">
                      <td className="px-3 py-2 truncate max-w-[200px]" title={u.email}>
                        {u.email}
                      </td>
                      <td className="px-3 py-2 font-mono text-xs">{u.chips_balance}</td>
                      <td className="px-3 py-2">
                        <button
                          type="button"
                          onClick={() => {
                            setBalanceUser(u)
                            setDelta('')
                            setReason('')
                            setBalanceErr(null)
                          }}
                          className="text-amber-400 text-xs hover:underline"
                        >
                          Saldo
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {usersQ.isError && (
                <p className="p-3 text-red-400 text-xs">{getApiError(usersQ.error)}</p>
              )}
            </div>
          </div>
        </section>

        <section>
          <h3 className="text-sm font-semibold text-gray-400 uppercase tracking-wide mb-3">
            Histórico de mãos (banco)
          </h3>
          <div className="rounded-2xl border border-gray-800 overflow-x-auto">
            <table className="w-full text-sm min-w-[640px]">
              <thead className="bg-gray-900 text-gray-500 text-left">
                <tr>
                  <th className="px-3 py-2 font-medium">ID</th>
                  <th className="px-3 py-2 font-medium">Mesa</th>
                  <th className="px-3 py-2 font-medium">#</th>
                  <th className="px-3 py-2 font-medium">Status</th>
                  <th className="px-3 py-2 font-medium">Criada</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800 bg-gray-900/50">
                {handsQ.data?.map((h) => (
                  <tr key={h.id} className="text-gray-300">
                    <td className="px-3 py-2 font-mono text-[11px]">
                      <button
                        type="button"
                        onClick={() => setHandDetailId(h.id)}
                        className="text-amber-400 hover:underline text-left"
                      >
                        {h.id.slice(0, 8)}…
                      </button>
                    </td>
                    <td className="px-3 py-2 font-mono text-xs truncate max-w-[120px]">
                      {h.room_id}
                    </td>
                    <td className="px-3 py-2">{h.hand_number}</td>
                    <td className="px-3 py-2">{h.status}</td>
                    <td className="px-3 py-2 text-xs text-gray-500">{h.created_at}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {handsQ.isError && (
              <p className="p-3 text-red-400 text-xs">{getApiError(handsQ.error)}</p>
            )}
          </div>
        </section>
      </main>

      {balanceUser && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center p-4 z-50">
          <div className="bg-gray-900 border border-gray-700 rounded-2xl p-6 max-w-md w-full space-y-3">
            <h4 className="font-semibold">Ajustar saldo</h4>
            <p className="text-xs text-gray-500 break-all">{balanceUser.email}</p>
            <label className="block text-xs text-gray-500">
              Delta (positivo ou negativo)
              <input
                type="number"
                className="mt-1 w-full rounded-lg bg-gray-950 border border-gray-700 px-2 py-1.5 text-sm"
                value={delta}
                onChange={(e) => setDelta(e.target.value)}
              />
            </label>
            <label className="block text-xs text-gray-500">
              Motivo (auditoria)
              <textarea
                className="mt-1 w-full rounded-lg bg-gray-950 border border-gray-700 px-2 py-1.5 text-sm min-h-[72px]"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </label>
            {balanceErr && <p className="text-red-400 text-xs">{balanceErr}</p>}
            <div className="flex gap-2 justify-end pt-2">
              <button
                type="button"
                onClick={() => setBalanceUser(null)}
                className="px-3 py-1.5 rounded-lg border border-gray-600 text-sm"
              >
                Fechar
              </button>
              <button
                type="button"
                disabled={balanceMut.isPending || !reason.trim() || delta === ''}
                onClick={() => balanceMut.mutate()}
                className="px-3 py-1.5 rounded-lg bg-amber-600 text-sm font-medium disabled:opacity-50"
              >
                Aplicar
              </button>
            </div>
          </div>
        </div>
      )}

      {handDetailId && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center p-4 z-50">
          <div className="bg-gray-900 border border-gray-700 rounded-2xl p-6 max-w-2xl w-full max-h-[85vh] overflow-y-auto space-y-3">
            <div className="flex justify-between items-start gap-4">
              <h4 className="font-semibold">Detalhe da mão</h4>
              <button
                type="button"
                onClick={() => setHandDetailId(null)}
                className="text-gray-400 text-sm hover:text-white"
              >
                Fechar
              </button>
            </div>
            {handDetailQ.isLoading && <p className="text-gray-500 text-sm">Carregando…</p>}
            {handDetailQ.isError && (
              <p className="text-red-400 text-sm">{getApiError(handDetailQ.error)}</p>
            )}
            {handDetailQ.data && (
              <>
                <pre className="text-xs bg-gray-950 p-3 rounded-lg overflow-x-auto text-gray-400">
                  {JSON.stringify(handDetailQ.data.hand, null, 2)}
                </pre>
                <p className="text-xs text-gray-500">
                  {handDetailQ.data.actions.length} ações
                </p>
                <ul className="text-xs font-mono space-y-1 text-gray-400 max-h-64 overflow-y-auto">
                  {handDetailQ.data.actions.map((a) => (
                    <li key={a.id}>
                      #{a.action_seq} {a.action_type} {a.street}{' '}
                      {a.amount != null ? `amt=${a.amount}` : ''}
                    </li>
                  ))}
                </ul>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
