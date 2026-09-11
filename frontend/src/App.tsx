import { lazy, Suspense } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from './auth/AuthContext'
import { useI18n } from './i18n'
import Layout from './components/Layout'
import PageFallback from './components/PageFallback'

// Each route is bound to a business page and loaded on demand, so the initial
// bundle only ships the shell and the page the user actually opens.
const Dashboard = lazy(() => import('./pages/Dashboard'))
const InterfaceDetail = lazy(() => import('./pages/InterfaceDetail'))
const Login = lazy(() => import('./pages/Login'))

export default function App() {
  const { authed, loading } = useAuth()
  const { t } = useI18n()

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <span className="text-sm text-muted-foreground">{t('iface.loading')}</span>
      </div>
    )
  }

  if (!authed) {
    return (
      <Suspense fallback={<PageFallback />}>
        <Login />
      </Suspense>
    )
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
