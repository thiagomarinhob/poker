import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '../../stores/authStore'
import { logout } from '../../api/auth'
import { fetchTables } from '../../api/tables'
import { getApiError } from '../../api/client'

export function LobbyPage() {
  const navigate = useNavigate()
  const { userId, role, refreshToken, logout: storeLogout } = useAuthStore()

  const {
    data: tables,
    isLoading,
    isError,
    error,
    refetch,
  } = useQuery({
    queryKey: ['tables'],
    queryFn: fetchTables,
  })

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
          <div className="flex items-center gap-2">
            <span className="text-xl select-none">♠</span>
            <span className="font-bold text-white">Poker</span>
          </div>
          <div className="flex items-center gap-4">
            {role === 'admin' && (
              <button
                onClick={() => navigate('/admin')}
                className="text-sm text-amber-400 hover:text-amber-300 font-medium"
              >
                Admin
              </button>
            )}
            <button
              onClick={handleLogout}
              className="text-sm text-gray-400 hover:text-white transition-colors"
            >
              Sair
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-4 py-10">
        <div className="mb-8">
          <h2 className="text-2xl font-bold">Lobby</h2>
          <p className="text-gray-500 text-sm mt-1">
            ID:{' '}
            <span className="font-mono text-gray-400">{userId ?? '—'}</span>
            {' · '}
            <span
              className={`font-medium ${role === 'admin' ? 'text-amber-400' : 'text-emerald-400'}`}
            >
              {role ?? '—'}
            </span>
          </p>
        </div>

        <div className="rounded-2xl border border-gray-800 bg-gray-900">
          <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between">
            <h3 className="font-semibold text-sm uppercase tracking-wider text-gray-400">Mesas</h3>
            <button
              type="button"
              onClick={() => refetch()}
              className="text-xs px-3 py-1.5 bg-gray-800 text-gray-300 rounded-lg hover:bg-gray-700"
            >
              Atualizar
            </button>
          </div>

          {isLoading && (
            <div className="py-16 text-center text-sm text-gray-500">Carregando mesas…</div>
          )}

          {isError && (
            <div className="px-5 py-8 text-center text-sm text-red-400">
              {getApiError(error)}
            </div>
          )}

          {!isLoading && !isError && tables?.length === 0 && (
            <div className="flex flex-col items-center justify-center py-20 text-center">
              <div className="text-4xl mb-3 opacity-20 select-none">🂡</div>
              <p className="text-gray-600 text-sm">Nenhuma mesa. Um admin pode criar em /admin.</p>
            </div>
          )}

          {!isLoading && !isError && tables && tables.length > 0 && (
            <ul className="divide-y divide-gray-800">
              {tables.map((t) => (
                <li key={t.id} className="px-5 py-4 flex flex-wrap items-center justify-between gap-4">
                  <div>
                    <p className="font-medium">{t.name}</p>
                    <p className="text-xs text-gray-500 font-mono mt-1">{t.id}</p>
                    <p className="text-xs text-gray-500 mt-2">
                      {t.max_seats} lugares · blinds {t.small_blind}/{t.big_blind} · timeout {t.turn_timeout_seconds}s
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => navigate(`/table/${t.id}`)}
                    className="text-sm px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 font-medium shrink-0"
                  >
                    Entrar na mesa
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </main>
    </div>
  )
}
