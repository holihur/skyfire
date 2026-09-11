import { useEffect, useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { GearIcon, LightningBoltIcon, ExitIcon, SunIcon, MoonIcon } from '@radix-ui/react-icons'
import { useAuth } from '../auth/AuthContext'
import { useI18n } from '../i18n'
import { useTheme } from '../theme'
import SettingsDialog from './SettingsDialog'

export default function Layout() {
  const { logout } = useAuth()
  const { t, lang, setLang } = useI18n()
  const { theme, toggle } = useTheme()
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
      <header className="flex items-center justify-between border-b border-edge bg-header/80 px-5 py-3 backdrop-blur">
        <div className="flex items-center gap-6">
          <Link to="/" className="flex items-center gap-2.5">
            <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-sky-500 to-blue-700 text-sm font-bold text-white shadow shadow-sky-900/50">
              S
            </span>
            <span className="text-base font-semibold tracking-tight text-fg">
              Skyfire
              <span className="ml-2 text-xs font-normal text-muted">{t('app.subtitle')}</span>
            </span>
          </Link>

          <nav className="hidden items-center gap-1 md:flex">
            <NavLink
              to="/"
              className="rounded-lg px-3 py-1.5 text-sm text-muted transition hover:text-fg data-active:bg-hover/5 data-active:text-fg"
            >
              {t('nav.interfaces')}
            </NavLink>
          </nav>
        </div>

        <div className="flex items-center gap-3">
          {health && (
            <span className="mono rounded-full border border-edge2 bg-panel2 px-3 py-1 text-muted">
              {health.driver}
              {health.dryRun && (
                <span className="ml-2 rounded-full bg-warn/15 px-2 py-0.5 text-warn">
                  {t('status.dryrun')}
                </span>
              )}
            </span>
          )}
          <span className="flex items-center gap-1 rounded-full bg-ok/10 px-2.5 py-1 text-xs font-medium text-ok">
            <LightningBoltIcon className="h-3.5 w-3.5" />
            {t('status.online')}
          </span>
          <select
            className="cursor-pointer rounded-lg border border-edge2 bg-panel2 px-2 py-1.5 text-xs font-medium text-fg2 outline-none transition hover:text-fg"
            value={lang}
            onChange={(e) => setLang(e.target.value as 'en' | 'zh')}
            title={t('lang.' + lang)}
            aria-label="Language"
          >
            <option value="en">{t('lang.en')}</option>
            <option value="zh">{t('lang.zh')}</option>
          </select>
          <button
            className="btn-ghost !p-2"
            title={theme === 'dark' ? t('theme.light') : t('theme.dark')}
            onClick={toggle}
          >
            {theme === 'dark' ? <SunIcon className="h-4 w-4" /> : <MoonIcon className="h-4 w-4" />}
          </button>
          <button
            className="btn-ghost !p-2"
            title={t('header.settings')}
            onClick={() => setSettingsOpen(true)}
          >
            <GearIcon className="h-4 w-4" />
          </button>
          <button className="btn-ghost !p-2" title={t('header.signout')} onClick={doLogout}>
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