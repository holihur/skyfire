import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuth } from '../auth/AuthContext'
import { useI18n } from '../i18n'

export default function Login() {
  const { login } = useAuth()
  const { t } = useI18n()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    const ok = await login(username.trim(), password)
    setBusy(false)
    if (!ok) setError(t('login.invalid'))
  }

  return (
    <div className="flex h-full items-center justify-center">
      <Card className="w-80 space-y-4 p-8 shadow-2xl shadow-black/40">
        <form onSubmit={submit} className="space-y-4">
          <div className="text-center">
            <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-sky-500 to-blue-700 text-2xl font-bold text-primary-foreground shadow-lg shadow-sky-900/50">
              S
            </div>
            <h1 className="text-xl font-semibold text-foreground">Skyfire</h1>
            <p className="mt-1 text-sm text-muted-foreground">{t('login.subtitle')}</p>
          </div>

          <div>
            <Label htmlFor="username">{t('login.username')}</Label>
            <Input
              id="username"
              type="text"
              autoFocus
              autoComplete="username"
              placeholder="admin"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </div>

          <div>
            <Label htmlFor="password">{t('login.password')}</Label>
            <Input
              id="password"
              type="password"
              autoComplete="current-password"
              placeholder={t('login.passwordPlaceholder')}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}

          <Button
            type="submit"
            className="w-full"
            disabled={busy || !username.trim() || !password}
          >
            {busy ? t('login.signingIn') : t('login.signin')}
          </Button>
        </form>
      </Card>
    </div>
  )
}
