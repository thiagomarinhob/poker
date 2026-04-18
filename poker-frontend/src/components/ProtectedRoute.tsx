import { Navigate, Outlet } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'

interface Props {
  requiredRole?: 'player' | 'admin'
}

export function ProtectedRoute({ requiredRole }: Props) {
  const accessToken = useAuthStore((s) => s.accessToken)
  const role = useAuthStore((s) => s.role)

  if (!accessToken) return <Navigate to="/login" replace />
  if (requiredRole && role !== requiredRole) return <Navigate to="/lobby" replace />

  return <Outlet />
}
