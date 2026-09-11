import { useEffect, useState } from 'react'
import Dialog from './Dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useToast } from '@/components/ui/use-toast'
import { api } from '../lib/api'
import { useI18n } from '../i18n'
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
  const { t } = useI18n()
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
      <div className="space-y-4">
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
      </div>
    </Dialog>
  )
}
