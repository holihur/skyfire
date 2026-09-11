import { lazy, Suspense, useEffect, useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { LogOut, Rocket, Settings, Zap } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useAuth } from '../auth/AuthContext'
import { useI18n } from '../i18n'
import PageFallback from './PageFallback'

// Dialogs are only needed once the user opens them, so they are code-split and
// mounted on first use (kept mounted afterwards to preserve close animations).
const SettingsDialog = lazy(() => import('./SettingsDialog'))
const QuickConnectDialog = lazy(() => import('./QuickConnectDialog'))

export default function Layout() {
  const { logout } = useAuth()
  const { t } = useI18n()
  const navigate = useNavigate()
  const location = useLocation()
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [quickOpen, setQuickOpen] = useState(false)
  const [dialogsMounted, setDialogsMounted] = useState(false)
  const [health, setHealth] = useState<{ driver: string; dryRun: boolean; version: string } | null>(null)

  useEffect(() => {
    if (settingsOpen || quickOpen) setDialogsMounted(true)
  }, [settingsOpen, quickOpen])

  useEffect(() => {
    const load = () =>
      fetch('/api/health')
        .then((r) => (r.ok ? r.json() : null))
        .then((h) => setHealth(h))
        .catch(() => {})
    load()
    const id = setInterval(load, 10000)
    return () => clearInterval(id)
  }, [location.pathname])

  const doLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <div className="flex h-full flex-col">
      <header className="sticky top-0 z-30 border-b bg-card/80 backdrop-blur">
        <div className="flex h-14 items-center gap-3 px-4 sm:px-6">
          <Link to="/" className="flex shrink-0 items-center gap-2.5">
            <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-sky-500 to-blue-700 text-sm font-bold text-primary-foreground shadow shadow-sky-900/50">
              S
            </span>
            <span className="hidden text-base font-semibold tracking-tight text-foreground sm:inline">
              Skyfire
              <span className="ml-2 hidden text-xs font-normal text-muted-foreground lg:inline">
                {t('app.subtitle')}
              </span>
            </span>
          </Link>

          {health?.version && (
            <span
              className="mono hidden shrink-0 rounded-md border bg-popover px-2 py-0.5 text-[11px] text-muted-foreground sm:inline-block"
              title={health.version}
            >
              {health.version}
            </span>
          )}

          <nav className="flex items-center gap-1">
            <NavLink
              to="/"
              className={({ isActive }) =>
                `rounded-lg px-3 py-1.5 text-sm transition hover:text-foreground${
                  isActive ? ' bg-accent text-foreground' : ' text-muted-foreground'
                }`
              }
            >
              {t('nav.interfaces')}
            </NavLink>
          </nav>

          <div className="ml-auto flex items-center gap-2">
            {health && (
              <span className="mono hidden rounded-full border bg-popover px-3 py-1 text-xs text-muted-foreground md:inline-flex">
                {health.driver}
                {health.dryRun && (
                  <span className="ml-2 rounded-full bg-warning/15 px-2 py-0.5 text-warning">
                    {t('status.dryrun')}
                  </span>
                )}
              </span>
            )}
            <span className="hidden items-center gap-1 rounded-full bg-success/10 px-2.5 py-1 text-xs font-medium text-success sm:flex">
              <Zap className="h-3.5 w-3.5" />
              {t('status.online')}
            </span>

            <Button size="sm" onClick={() => setQuickOpen(true)} title={t('quick.title')}>
              <Rocket />
              <span className="hidden sm:inline">{t('nav.connect')}</span>
            </Button>

            <div className="mx-0.5 hidden h-5 w-px bg-border sm:block" />

            <Button variant="ghost" size="icon" title={t('header.settings')} onClick={() => setSettingsOpen(true)}>
              <Settings />
            </Button>
            <Button variant="ghost" size="icon" title={t('header.signout')} onClick={doLogout}>
              <LogOut />
            </Button>
          </div>
        </div>
      </header>

      <main className="flex-1 overflow-y-auto px-5 py-6">
        <Suspense fallback={<PageFallback />}>
          <Outlet />
        </Suspense>
      </main>

      {dialogsMounted && (
        <Suspense fallback={null}>
          <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} />
          <QuickConnectDialog open={quickOpen} onOpenChange={setQuickOpen} />
        </Suspense>
      )}
    </div>
  )
}
