import { useEffect, useState } from 'react'
import Dialog from './Dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { useToast } from '@/components/ui/use-toast'
import { api } from '../lib/api'
import { useI18n } from '../i18n'
import { useTheme } from '../theme'
import type { Settings } from '../lib/types'

export default function SettingsDialog({
  open,
  onOpenChange,
  onSaved,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  onSaved?: () => void
}) {
  const { toast } = useToast()
  const { t, lang, setLang } = useI18n()
  const { theme, setTheme } = useTheme()
  const [saving, setSaving] = useState(false)
  const [s, setS] = useState<Settings>({ publicEndpoint: '' })

  useEffect(() => {
    if (open) void api.settings().then(setS).catch(() => {})
  }, [open])

  const save = async () => {
    setSaving(true)
    try {
      await api.saveSettings(s)
      toast({ description: t('common.saved'), variant: 'success' })
      onOpenChange(false)
      onSaved?.()
    } catch (err) {
      toast({ description: String(err), variant: 'destructive' })
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('common.settings')}
      description={t('common.settingsDesc')}
      footer={
        <>
          <Button onClick={save} disabled={saving}>
            {saving ? t('common.saving') : t('common.save')}
          </Button>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
        </>
      }
    >
      <div className="space-y-6">
        <section className="space-y-3">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            {t('settings.section.general')}
          </h3>
          <div>
            <Label htmlFor="public-endpoint">{t('settings.publicEndpoint')}</Label>
            <Input
              id="public-endpoint"
              placeholder={t('settings.publicEndpointPh')}
              value={s.publicEndpoint}
              onChange={(e) => setS({ ...s, publicEndpoint: e.target.value })}
            />
            <p className="mt-1.5 text-xs text-muted-foreground">
              {t('settings.publicEndpointHelp')}
            </p>
          </div>

          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="forwarding">{t('settings.forwarding')}</Label>
              <p className="text-xs text-muted-foreground">{t('settings.forwardingHelp')}</p>
            </div>
            <Switch
              id="forwarding"
              checked={s.forwarding !== false}
              onCheckedChange={(v) => setS({ ...s, forwarding: v })}
            />
          </div>
        </section>

        <section className="space-y-3 border-t pt-4">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            {t('settings.section.appearance')}
          </h3>

          <div className="flex items-center justify-between gap-4">
            <Label>{t('settings.theme')}</Label>
            <div className="flex gap-1">
              <Button
                size="sm"
                variant={theme === 'light' ? 'secondary' : 'ghost'}
                onClick={() => setTheme('light')}
              >
                {t('settings.themeLight')}
              </Button>
              <Button
                size="sm"
                variant={theme === 'dark' ? 'secondary' : 'ghost'}
                onClick={() => setTheme('dark')}
              >
                {t('settings.themeDark')}
              </Button>
            </div>
          </div>

          <div className="flex items-center justify-between gap-4">
            <Label htmlFor="settings-language">{t('settings.language')}</Label>
            <select
              id="settings-language"
              className="cursor-pointer rounded-lg border bg-popover px-3 py-1.5 text-sm text-secondary-foreground outline-none transition hover:text-foreground"
              value={lang}
              onChange={(e) => setLang(e.target.value as 'en' | 'zh')}
            >
              <option value="en">{t('lang.en')}</option>
              <option value="zh">{t('lang.zh')}</option>
            </select>
          </div>
        </section>
      </div>
    </Dialog>
  )
}
