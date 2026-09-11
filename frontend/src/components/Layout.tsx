import { useEffect, useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { GearIcon, LightningBoltIcon, ExitIcon } from '@radix-ui/react-icons'
import { useAuth } from '../auth/AuthContext'
import SettingsDialog from './SettingsDialog'

export default function Layout() {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [health, setHealth] = useState<{ driver: string; dryRun: boolean } | null>(null)

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
      <header className="flex items-center justify-between border-b border-[#1e2a45] bg-[#0a1426]/80 px-5 py-3 backdrop-blur">
        <div className="flex items-center gap-6">
          <Link to="/" className="flex items-center gap-2.5">
            <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-sky-500 to-blue-700 text-sm font-bold text-white shadow shadow-sky-900/50">
              S
            </span>
            <span className="text-base font-semibold tracking-tight text-white">
              Skyfire
              <span className="ml-2 text-xs font-normal text-slate-500">WireGuard console</span>
            </span>
          </Link>

          <nav className="hidden items-center gap-1 md:flex">
            <NavLink
              to="/"
              className="rounded-lg px-3 py-1.5 text-sm text-slate-400 transition hover:text-white data-active:bg-white/5 data-active:text-white"
            >
              Interfaces
            </NavLink>
          </nav>
        </div>

        <div className="flex items-center gap-3">
          {health && (
            <span className="mono rounded-full border border-[#263450] bg-[#0e1930] px-3 py-1 text-slate-400">
              {health.driver}
              {health.dryRun && (
                <span className="ml-2 rounded-full bg-amber-500/15 px-2 py-0.5 text-amber-300">
                  dry-run
                </span>
              )}
            </span>
          )}
          <span className="flex items-center gap-1 rounded-full bg-emerald-500/10 px-2.5 py-1 text-xs text-emerald-300">
            <LightningBoltIcon className="h-3.5 w-3.5" />
            online
          </span>
          <button
            className="btn-ghost !p-2"
            title="Settings"
            onClick={() => setSettingsOpen(true)}
          >
            <GearIcon className="h-4 w-4" />
          </button>
<button className="btn-ghost !p-2" title="Sign out" onClick={doLogout}>
      <ExitIcon className="h-4 w-4" />
    </button>
        </div>
      </header>

      <main className="flex-1 overflow-y-auto px-5 py-6">
        <Outlet />
      </main>

      <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} />
    </div>
  )
}