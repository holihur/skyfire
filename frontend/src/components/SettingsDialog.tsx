import { useEffect, useState } from 'react'
import Dialog from './Dialog'
import { api } from '../lib/api'
import { useToast } from './Toast'
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
  const [saving, setSaving] = useState(false)
  const [s, setS] = useState<Settings>({ publicEndpoint: '' })

  useEffect(() => {
    if (open) void api.settings().then(setS).catch(() => {})
  }, [open])

  const save = async () => {
    setSaving(true)
    try {
      await api.saveSettings(s)
      push('Settings saved', 'success')
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
      title="Settings"
      description="Daemon-wide preferences used when generating client configurations."
      footer={
        <>
          <button className="btn-primary" onClick={save} disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </button>
          <button className="btn-ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="label" htmlFor="public-endpoint">
            Public endpoint
          </label>
          <input
            id="public-endpoint"
            className="input"
            placeholder="vpn.example.com:51820"
            value={s.publicEndpoint}
            onChange={(e) => setS({ ...s, publicEndpoint: e.target.value })}
          />
          <p className="mt-1.5 text-xs text-slate-400">
            Host and port clients should connect to. If the port is omitted the
            interface's listen port is appended automatically.
          </p>
        </div>
      </div>
    </Dialog>
  )
}