import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuth } from '../auth/AuthContext'
import { useI18n } from '../i18n'

type Step = 'credentials' | 'totp' | 'enroll'

export default function Login() {
  const { login } = useAuth()
  const { t } = useI18n()
  const [step, setStep] = useState<Step>('credentials')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [totp, setTotp] = useState('')
  const [enroll, setEnroll] = useState<{ secret?: string; uri?: string; qr?: string }>({})
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submitCredentials = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    const result = await login(username.trim(), password)
    setBusy(false)
    if (result.ok) return
    if (result.networkError) {
      setError(t('login.unreachable'))
      return
    }
    if (result.body?.totpRequired) {
      setStep('totp')
      return
    }
    if (result.body?.enroll) {
      setEnroll(result.body)
      setStep('enroll')
      return
    }
    setError(t('login.invalid'))
  }

  const submitCode = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    const result = await login(username.trim(), password, totp.trim())
    setBusy(false)
    if (result.ok) return
    if (result.networkError) {
      setError(t('login.unreachable'))
      return
    }
    setError(result.error || t('login.totpInvalid'))
  }

  const back = () => {
    setStep('credentials')
    setTotp('')
    setError('')
  }

  return (
    <div className="flex h-full items-center justify-center">
      <Card className="w-80 space-y-4 p-8 shadow-2xl shadow-black/40">
        <div className="text-center">
          <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-sky-500 to-blue-700 text-2xl font-bold text-primary-foreground shadow-lg shadow-sky-900/50">
            S
          </div>
          <h1 className="text-xl font-semibold text-foreground">Skyfire</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {step === 'enroll' ? t('login.enrollTitle') : step === 'totp' ? t('login.totpTitle') : t('login.subtitle')}
          </p>
        </div>

        {step === 'credentials' && (
          <form onSubmit={submitCredentials} className="space-y-4">
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

            <Button type="submit" className="w-full" disabled={busy || !username.trim() || !password}>
              {busy ? t('login.signingIn') : t('login.signin')}
            </Button>
          </form>
        )}

        {step === 'totp' && (
          <form onSubmit={submitCode} className="space-y-4">
            <p className="text-sm text-muted-foreground">{t('login.totpPrompt')}</p>
            <div>
              <Label htmlFor="totp">{t('login.totpCode')}</Label>
              <Input
                id="totp"
                className="mono tracking-[0.3em]"
                inputMode="numeric"
                autoComplete="one-time-code"
                autoFocus
                maxLength={6}
                placeholder={t('login.totpPlaceholder')}
                value={totp}
                onChange={(e) => setTotp(e.target.value.replace(/\D/g, '').slice(0, 6))}
              />
            </div>

            {error && <p className="text-sm text-destructive">{error}</p>}

            <Button type="submit" className="w-full" disabled={busy || totp.length !== 6}>
              {busy ? t('login.signingIn') : t('login.totpVerify')}
            </Button>
            <Button type="button" variant="ghost" className="w-full" onClick={back}>
              {t('login.back')}
            </Button>
          </form>
        )}

        {step === 'enroll' && (
          <form onSubmit={submitCode} className="space-y-4">
            <p className="text-sm text-muted-foreground">{t('login.enrollPrompt')}</p>
            {enroll.qr && (
              <div className="rounded-xl border bg-white p-3">
                <img src={enroll.qr} alt="TOTP QR" className="mx-auto h-44 w-44" />
              </div>
            )}
            {enroll.secret && (
              <div>
                <div className="mb-1 text-xs uppercase tracking-wide text-muted-foreground">
                  {t('login.enrollSecret')}
                </div>
                <code className="mono block break-all rounded-lg border bg-secondary px-3 py-2 text-xs">
                  {enroll.secret}
                </code>
              </div>
            )}
            <div>
              <Label htmlFor="enroll-code">{t('login.totpCode')}</Label>
              <Input
                id="enroll-code"
                className="mono tracking-[0.3em]"
                inputMode="numeric"
                autoComplete="one-time-code"
                autoFocus
                maxLength={6}
                placeholder={t('login.totpPlaceholder')}
                value={totp}
                onChange={(e) => setTotp(e.target.value.replace(/\D/g, '').slice(0, 6))}
              />
            </div>

            {error && <p className="text-sm text-destructive">{error}</p>}

            <Button type="submit" className="w-full" disabled={busy || totp.length !== 6}>
              {busy ? t('login.signingIn') : t('login.enrollConfirm')}
            </Button>
            <Button type="button" variant="ghost" className="w-full" onClick={back}>
              {t('login.back')}
            </Button>
          </form>
        )}
      </Card>
    </div>
  )
}
