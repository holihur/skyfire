import { useEffect, useState } from 'react'
import Dialog from './Dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useToast } from '@/components/ui/use-toast'
import { api } from '../lib/api'
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
  const { toast } = useToast()
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
        toast({ description: t('peer.updated'), variant: 'success' })
      } else {
        await api.addPeer(ifaceName, input)
        toast({ description: t('peer.created'), variant: 'success' })
      }
      onOpenChange(false)
      onDone?.()
    } catch (err) {
      toast({ description: String(err), variant: 'destructive' })
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={editing ? t('iface.editPeerTitle', { name: peer!.name }) : t('iface.addPeer')}
      description={editing ? t('peer.editDesc') : t('peer.addDesc')}
      footer={
        <>
          <Button onClick={submit} disabled={busy}>
            {busy ? t('common.saving') : t('peer.save')}
          </Button>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <Label htmlFor="peer-name">{t('peer.label.name')}</Label>
            <Input
              id="peer-name"
              placeholder={t('peer.placeholder.name')}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div>
            <Label htmlFor="peer-address">{t('peer.label.address')}</Label>
            <Input
              id="peer-address"
              placeholder={t('peer.placeholder.autoAssign')}
              value={address}
              onChange={(e) => setAddress(e.target.value)}
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <Label htmlFor="peer-keepalive">{t('peer.label.keepalive')}</Label>
            <Input
              id="peer-keepalive"
              type="number"
              min={0}
              value={keepalive}
              onChange={(e) => setKeepalive(e.target.value)}
            />
          </div>
          <div>
            <Label htmlFor="peer-endpoint">{t('peer.label.endpoint')}</Label>
            <Input
              id="peer-endpoint"
              placeholder={t('peer.placeholder.endpoint')}
              value={endpoint}
              onChange={(e) => setEndpoint(e.target.value)}
            />
          </div>
        </div>

        <div>
          <Label htmlFor="peer-routes">{t('peer.label.clientRoutes')}</Label>
          <Textarea id="peer-routes" className="mono" rows={2} value={clientRoutes} onChange={(e) => setClientRoutes(e.target.value)} />
        </div>

        <div>
          <Label htmlFor="peer-client-dns">{t('peer.label.clientDns')}</Label>
          <Textarea id="peer-client-dns" className="mono" rows={2} value={dns} onChange={(e) => setDns(e.target.value)} />
        </div>

        <div>
          <Label htmlFor="peer-desc">{t('peer.label.description')}</Label>
          <Input
            id="peer-desc"
            placeholder={t('peer.placeholder.notes')}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>

        <div className="flex items-center justify-between rounded-lg border bg-secondary px-3 py-2.5">
          <span className="text-sm">{t('peer.enabled')}</span>
          <Switch checked={enabled} onCheckedChange={setEnabled} />
        </div>

        <div className="rounded-lg border bg-secondary p-3">
          <div className="flex cursor-pointer items-center justify-between">
            <span className="text-sm">{t('peer.usePsk')}</span>
            <Switch
              checked={usePsk}
              onCheckedChange={(v) => {
                setUsePsk(v)
                if (v && !psk && !editing) setPsk('generate')
              }}
            />
          </div>
          {usePsk && (
            <Input
              className="mono mt-2"
              placeholder={t('peer.placeholder.psk')}
              value={psk}
              onChange={(e) => setPsk(e.target.value)}
            />
          )}
        </div>

        {editing && (
          <div className="flex items-center justify-between rounded-lg border border-warning/40 bg-warning/10 px-3 py-2.5">
            <span className="text-sm text-warning">{t('peer.rotateKeys')}</span>
            <Switch checked={rotateKeys} onCheckedChange={setRotateKeys} />
          </div>
        )}
      </div>
    </Dialog>
  )
}
