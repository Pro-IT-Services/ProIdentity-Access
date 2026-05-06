import { useEffect, lazy, Suspense } from 'react'
import { Routes, Route, Navigate, useNavigate } from 'react-router-dom'
import { useAuthStore } from './stores/useAuthStore'
import { api } from './api/client'
import Layout from './components/Layout'
import Login from './pages/Login'
import Overview from './pages/Overview'
import Access from './pages/Access'
import ServersList from './pages/ServersList'
import ServerDetail from './pages/ServerDetail'
import System from './pages/System'
import Sessions from './pages/Sessions'
import ConnectionHistory from './pages/ConnectionHistory'
import Profile from './pages/Profile'

// Topology pulls in @xyflow/react + dagre — lazy-load so the main bundle stays slim.
const Topology = lazy(() => import('./pages/Topology'))

function RequireAuth({ children }: { children: React.ReactNode }) {
  const token = useAuthStore(s => s.token)
  const ready = useAuthStore(s => s.ready)
  if (!ready) return null
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

function RequireAdmin({ children }: { children: React.ReactNode }) {
  const user = useAuthStore(s => s.user)
  const ready = useAuthStore(s => s.ready)
  if (!ready) return null
  if (!user?.is_admin) return <Navigate to="/" replace />
  return <>{children}</>
}

export default function App() {
  const { token, setAuth, logout } = useAuthStore()
  const navigate = useNavigate()

  useEffect(() => {
    if (!token) return
    api.me().then(u => setAuth(token, u)).catch(() => { logout(); navigate('/login') })
  }, [token])

  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/" element={<RequireAuth><Layout /></RequireAuth>}>
        <Route index element={<Overview />} />
        <Route path="profile" element={<Profile />} />
        <Route path="sessions" element={<Sessions />} />

        <Route path="access" element={<RequireAdmin><Access /></RequireAdmin>} />
        <Route path="servers" element={<RequireAdmin><ServersList /></RequireAdmin>} />
        <Route path="servers/:id" element={<RequireAdmin><ServerDetail /></RequireAdmin>} />
        <Route path="topology" element={<RequireAdmin><Suspense fallback={<div className="p-6 text-sm text-muted-foreground">Loading topology…</div>}><Topology /></Suspense></RequireAdmin>} />
        <Route path="connection-history" element={<RequireAdmin><ConnectionHistory /></RequireAdmin>} />
        <Route path="system" element={<RequireAdmin><System /></RequireAdmin>} />

        {/* Legacy redirects — preserve old deep links */}
        <Route path="users" element={<Navigate to="/access#people" replace />} />
        <Route path="groups" element={<Navigate to="/access#roles" replace />} />
        <Route path="resources" element={<Navigate to="/access#resources" replace />} />
        <Route path="resource-groups" element={<Navigate to="/access#resources" replace />} />
        <Route path="user-configs" element={<Navigate to="/servers" replace />} />
        <Route path="installations" element={<Navigate to="/servers" replace />} />
        <Route path="settings" element={<Navigate to="/system" replace />} />
      </Route>
    </Routes>
  )
}
