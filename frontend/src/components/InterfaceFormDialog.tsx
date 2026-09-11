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
import type { WireGuardInterface } from '../lib/types'

interface Props {
  open: boolean
  onOpenChange: (v: boolean) => void
  onDone: (iface: WireGuardInterface) => void
  existing?: WireGuardInterface | null
}

export default function InterfaceFormDialog({ open, onOpenChange, onDone, existing }: Props) {
  const { toast } = useToast()
  const { t } = useI18n()
  const editing = !!existing
  const [busy, setBusy] = useState(false)
  const [name, setName] = useState('')
  const [listenPort, setListenPort] = useState('51820')
  const [addresses, setAddresses] = useState('10.42.0.1/24')
  const [mtu, setMtu] = useState('1420')
  const [dns, setDns] = useState('1.1.1.1\n9.9.9.9')
  const [up, setUp] = useState(true)

  useEffect(() => {
    if (open) {
      setName(existing?.name ?? '')
      setListenPort(existing ? String(existing.listenPort || 0) : '51820')
      setAddresses(existing ? joinList(existing.addresses) : '10.42.0.1/24')
      setMtu(existing ? String(existing.mtu) : '1420')
      setDns(joinList(existing?.dns))
      setUp(existing?.up ?? true)
    }
  }, [open, existing])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      toast({ description: t('ifaceForm.nameRequired'), variant: 'destructive' })
      return
    }
    const addressList = parseList(addresses)
    if (addressList.length === 0) {
      toast({ description: t('ifaceForm.addressesRequired'), variant: 'destructive' })
      return
    }
    const mtuStr = mtu.trim()
    if (mtuStr === '' || Number(mtuStr) === 0) {
      toast({ description: t('ifaceForm.mtuInvalid'), variant: 'destructive' })
      return
    }
    const mtuNum = Number(mtuStr)
    setBusy(true)
    try {
      const body = {
        name,
        listenPort: Number(listenPort) || 0,
        addresses: addressList,
        mtu: Number.isFinite(mtuNum) && mtuNum > 0 ? mtuNum : 1420,
        dns: parseList(dns),
        up,
      }
      if (editing) {
        const patch = {
          listenPort: body.listenPort,
          addresses: body.addresses,
          mtu: body.mtu,
          dns: body.dns,
          up: body.up,
        }
        const updated = await api.updateInterface(existing!.name, patch)
        toast({ description: t('iface.updated', { name: updated.name }), variant: 'success' })
        onDone(updated)
      } else {
        const created = await api.createInterface(body)
        toast({ description: t('iface.created', { name: created.name }), variant: 'success' })
        onDone(created)
      }
      onOpenChange(false)
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
      title={editing ? t('ifaceForm.editTitle', { name: existing!.name }) : t('ifaceForm.new')}
      description={editing ? t('ifaceForm.editDesc') : t('ifaceForm.createDesc')}
      footer={
        <>
          <Button onClick={submit} disabled={busy}>
            {busy ? t('common.saving') : editing ? t('ifaceForm.saveChanges') : t('ifaceForm.create')}
          </Button>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
        </>
      }
    >
      <form onSubmit={submit} className="space-y-4">
        <div>
          <Label htmlFor="iface-name">{t('ifaceForm.name')}</Label>
          <Input
            id="iface-name"
            placeholder="wg0"
            value={name}
            disabled={editing}
            onChange={(e) => setName(e.target.value)}
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <Label htmlFor="iface-port">{t('ifaceForm.listenPort')}</Label>
            <Input
              id="iface-port"
              type="number"
              min={0}
              max={65535}
              placeholder={t('ifaceForm.randomPort')}
              value={listenPort}
              onChange={(e) => setListenPort(e.target.value)}
            />
          </div>
          <div>
            <Label htmlFor="iface-mtu">{t('ifaceForm.mtu')}</Label>
            <Input
              id="iface-mtu"
              type="number"
              min={576}
              max={65535}
              value={mtu}
              onChange={(e) => setMtu(e.target.value)}
            />
          </div>
        </div>

        <div>
          <Label htmlFor="iface-addresses">{t('ifaceForm.addresses')}</Label>
          <Textarea id="iface-addresses" className="mono" rows={2} value={addresses} onChange={(e) => setAddresses(e.target.value)} />
        </div>

        <div>
          <Label htmlFor="iface-dns">{t('ifaceForm.dns')}</Label>
          <Textarea id="iface-dns" className="mono" rows={2} value={dns} onChange={(e) => setDns(e.target.value)} />
        </div>

        <div className="flex items-center justify-between rounded-lg border bg-secondary px-3 py-2.5">
          <span className="text-sm">{t('ifaceForm.createEnabled')}</span>
          <Switch checked={up} onCheckedChange={setUp} />
        </div>
      </form>
    </Dialog>
  )
}
