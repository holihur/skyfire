import { useEffect, useState } from 'react'
import Dialog from './Dialog'
import { api } from '../lib/api'
import { useToast } from './Toast'
import { useI18n } from '../i18n'
import { parseList, joinList } from '../lib/format'
import type { Peer } from '../lib/types'

interface Props {
  open: boolean
  onOpenChange: (v: boolean) => void
  onDone?: () => void
  ifaceName: string
  peer?: Peer | null
}

export default function PeerFormDialog({ open, onOpenChange, onDone, ifaceName, peer }: Props) {
  const { push } = useToast()
  const { t } = useI18n()
  const editing = !!peer
  const [busy, setBusy] = useState(false)

  const [name, setName] = useState('')
  const [address, setAddress] = useState('')
  const [endpoint, setEndpoint] = useState('')
  const [keepalive, setKeepalive] = useState('25')
  const [clientRoutes, setClientRoutes] = useState('0.0.0.0/0\n::/0')
  const [dns, setDns] = useState('')
  const [description, setDescription] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [usePsk, setUsePsk] = useState(false)
  const [psk, setPsk] = useState('')
  const [rotateKeys, setRotateKeys] = useState(false)

  useEffect(() => {
    if (open) {
      setName(peer?.name ?? '')
      setAddress(peer?.address ?? '')
      setEndpoint(peer?.endpoint ?? '')
      setKeepalive(String(peer?.persistentKeepalive ?? 25))
      setClientRoutes(peer ? joinList(peer.clientRoutes) : '0.0.0.0/0\n::/0')
      setDns(joinList(peer?.dns))
      setDescription(peer?.description ?? '')
      setEnabled(peer?.enabled ?? true)
      setUsePsk(!!peer?.presharedKey)
      setPsk(peer?.presharedKey ?? '')
      setRotateKeys(false)
    }
  }, [open, peer])

  const submit = async () => {
    setBusy(true)
    const input = {
      name,
      address: address.trim(),
      publicKey: peer?.publicKey ?? '',
      generateKeys: !editing || rotateKeys,
      presharedKey: psk,
      withPreshared: usePsk,
      allowedIPs: [],
      clientRoutes: parseList(clientRoutes),
      dns: parseList(dns),
      endpoint: endpoint.trim(),
      persistentKeepalive: Number(keepalive) || 0,
      description: description.trim(),
      enabled,
    }
    try {
      if (editing) {
        await api.updatePeer(ifaceName, peer!.publicKey, input)
        push(t('peer.updated'), 'success')
      } else {
        await api.addPeer(ifaceName, input)
        push(t('peer.created'), 'success')
      }
      onOpenChange(false)
      onDone?.()
    } catch (err) {
      push(String(err), 'error')
    } finally {
      setBusy(false)
    }
  }

  const row = 'flex items-center justify-between rounded-lg border border-edge2 bg-inset px-3 py-2.5'

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={editing ? t('iface.editPeerTitle', { name: peer!.name }) : t('iface.addPeer')}
      description={editing ? t('peer.editDesc') : t('peer.addDesc')}
      footer={
        <>
          <button className="btn-primary" onClick={submit} disabled={busy}>
            {busy ? t('common.saving') : t('peer.save')}
          </button>
          <button className="btn-ghost" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label" htmlFor="peer-name">
              {t('peer.label.name')}
            </label>
            <input
              id="peer-name"
              className="input"
              placeholder={t('peer.placeholder.name')}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div>
            <label className="label" htmlFor="peer-address">
              {t('peer.label.address')}
            </label>
            <input
              id="peer-address"
              className="input"
              placeholder={t('peer.placeholder.autoAssign')}
              value={address}
              onChange={(e) => setAddress(e.target.value)}
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label" htmlFor="peer-keepalive">
              {t('peer.label.keepalive')}
            </label>
            <input
              id="peer-keepalive"
              className="input"
              type="number"
              min={0}
              value={keepalive}
              onChange={(e) => setKeepalive(e.target.value)}
            />
          </div>
          <div>
            <label className="label" htmlFor="peer-endpoint">
              {t('peer.label.endpoint')}
            </label>
            <input
              id="peer-endpoint"
              className="input"
              placeholder={t('peer.placeholder.endpoint')}
              value={endpoint}
              onChange={(e) => setEndpoint(e.target.value)}
            />
          </div>
        </div>

        <div>
          <label className="label" htmlFor="peer-routes">
            {t('peer.label.clientRoutes')}
          </label>
          <textarea
            id="peer-routes"
            className="input mono"
            rows={2}
            value={clientRoutes}
            onChange={(e) => setClientRoutes(e.target.value)}
          />
        </div>

        <div>
          <label className="label" htmlFor="peer-client-dns">
            {t('peer.label.clientDns')}
          </label>
          <textarea
            id="peer-client-dns"
            className="input mono"
            rows={2}
            value={dns}
            onChange={(e) => setDns(e.target.value)}
          />
        </div>

        <div>
          <label className="label" htmlFor="peer-desc">
            {t('peer.label.description')}
          </label>
          <input
            id="peer-desc"
            className="input"
            placeholder={t('peer.placeholder.notes')}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>

        <label className={row}>
          <span className="text-sm text-fg2">{t('peer.enabled')}</span>
          <input
            type="checkbox"
            className="h-4 w-4 accent-sky-500"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
          />
        </label>

        <div className="rounded-lg border border-edge2 bg-inset p-3">
          <label className="flex cursor-pointer items-center justify-between">
            <span className="text-sm text-fg2">{t('peer.usePsk')}</span>
            <input
              type="checkbox"
              className="h-4 w-4 accent-sky-500"
              checked={usePsk}
              onChange={(e) => {
                setUsePsk(e.target.checked)
                if (e.target.checked && !psk && !editing) setPsk('generate')
              }}
            />
          </label>
          {usePsk && (
            <input
              className="input mono mt-2"
              placeholder={t('peer.placeholder.psk')}
              value={psk}
              onChange={(e) => setPsk(e.target.value)}
            />
          )}
        </div>

        {editing && (
          <label className="flex cursor-pointer items-center justify-between rounded-lg border border-warn/40 bg-warn/10 px-3 py-2.5">
            <span className="text-sm text-warn">{t('peer.rotateKeys')}</span>
            <input
              type="checkbox"
              className="h-4 w-4 accent-amber-500"
              checked={rotateKeys}
              onChange={(e) => setRotateKeys(e.target.checked)}
            />
          </label>
        )}
      </div>
    </Dialog>
  )
}