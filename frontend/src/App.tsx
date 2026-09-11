import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from './auth/AuthContext'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import InterfaceDetail from './pages/InterfaceDetail'
import Login from './pages/Login'

export default function App() {
  const { authed, loading } = useAuth()

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <span className="text-sm text-slate-500">Connecting…</span>
      </div>
    )
  }

  if (!authed) {
    return <Login />
  }

  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/interfaces/:name" element={<InterfaceDetail />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}