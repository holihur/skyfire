import { useEffect, useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { LogOut, Moon, Settings, Sun, Zap } from 'lucide-react'
import { Button } from '@/components/ui/button'
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
      <header className="flex items-center justify-between border-b bg-card/80 px-5 py-3 backdrop-blur">
        <div className="flex items-center gap-6">
          <Link to="/" className="flex items-center gap-2.5">
            <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-sky-500 to-blue-700 text-sm font-bold text-primary-foreground shadow shadow-sky-900/50">
              S
            </span>
            <span className="text-base font-semibold tracking-tight text-foreground">
              Skyfire
              <span className="ml-2 text-xs font-normal text-muted-foreground">{t('app.subtitle')}</span>
            </span>
          </Link>

          <nav className="hidden items-center gap-1 md:flex">
            <NavLink
              to="/"
              className="rounded-lg px-3 py-1.5 text-sm text-muted-foreground transition hover:text-foreground data-active:bg-accent data-active:text-foreground"
            >
              {t('nav.interfaces')}
            </NavLink>
          </nav>
        </div>

        <div className="flex items-center gap-3">
          {health && (
            <span className="mono hidden rounded-full border bg-popover px-3 py-1 text-muted-foreground sm:inline-flex">
              {health.driver}
              {health.dryRun && (
                <span className="ml-2 rounded-full bg-warning/15 px-2 py-0.5 text-warning">
                  {t('status.dryrun')}
                </span>
              )}
            </span>
          )}
          <span className="flex items-center gap-1 rounded-full bg-success/10 px-2.5 py-1 text-xs font-medium text-success">
            <Zap className="h-3.5 w-3.5" />
            {t('status.online')}
          </span>
          <select
            className="cursor-pointer rounded-lg border bg-popover px-2 py-1.5 text-xs font-medium text-secondary-foreground outline-none transition hover:text-foreground"
            value={lang}
            onChange={(e) => setLang(e.target.value as 'en' | 'zh')}
            title={t('lang.' + lang)}
            aria-label="Language"
          >
            <option value="en">{t('lang.en')}</option>
            <option value="zh">{t('lang.zh')}</option>
          </select>
          <Button
            variant="ghost"
            size="icon"
            title={theme === 'dark' ? t('theme.light') : t('theme.dark')}
            onClick={toggle}
          >
            {theme === 'dark' ? <Sun /> : <Moon />}
          </Button>
          <Button variant="ghost" size="icon" title={t('header.settings')} onClick={() => setSettingsOpen(true)}>
            <Settings />
          </Button>
          <Button variant="ghost" size="icon" title={t('header.signout')} onClick={doLogout}>
            <LogOut />
          </Button>
        </div>
      </header>

      <main className="flex-1 overflow-y-auto px-5 py-6">
        <Outlet />
      </main>

      <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} />
    </div>
  )
}
