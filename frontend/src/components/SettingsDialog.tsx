import { useEffect, useState } from 'react'
import Dialog from './Dialog'
import { api } from '../lib/api'
import { useToast } from './Toast'
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
  const { push } = useToast()
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
      push(t('common.saved'), 'success')
      onOpenChange(false)
      onSaved?.()
    } catch (err) {
      push(String(err), 'error')
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
          <button className="btn-primary" onClick={save} disabled={saving}>
            {saving ? t('common.saving') : t('common.save')}
          </button>
          <button className="btn-ghost" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="label" htmlFor="public-endpoint">
            {t('settings.publicEndpoint')}
          </label>
          <input
            id="public-endpoint"
            className="input"
            placeholder={t('settings.publicEndpointPh')}
            value={s.publicEndpoint}
            onChange={(e) => setS({ ...s, publicEndpoint: e.target.value })}
          />
          <p className="mt-1.5 text-xs text-muted">
            {t('settings.publicEndpointHelp')}
          </p>
        </div>
      </div>
    </Dialog>
  )
}