import { Navigate, Route, Routes } from 'react-router-dom'
import { LoginPage } from '../features/auth/LoginPage'
import { RegisterPage } from '../features/auth/RegisterPage'
import { LobbyPage } from '../features/lobby/LobbyPage'
import { TablePage } from '../features/table/TablePage'
import { AdminPage } from '../features/admin/AdminPage'
import { ProtectedRoute } from '../components/ProtectedRoute'

export function AppRoutes() {
  return (
    <Routes>
      {/* Public */}
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />

      {/* Protected — any authenticated user */}
      <Route element={<ProtectedRoute />}>
        <Route path="/lobby" element={<LobbyPage />} />
        <Route path="/table/:id" element={<TablePage />} />
      </Route>

      {/* Protected — admin only */}
      <Route element={<ProtectedRoute requiredRole="admin" />}>
        <Route path="/admin" element={<AdminPage />} />
      </Route>

      {/* Fallback */}
      <Route path="/" element={<Navigate to="/lobby" replace />} />
      <Route path="*" element={<Navigate to="/lobby" replace />} />
    </Routes>
  )
}
