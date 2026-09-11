import { useEffect, useState } from 'react'
import Dialog from './Dialog'
import { api } from '../lib/api'
import { useToast } from './Toast'
import { parseList, joinList } from '../lib/format'
import type { WireGuardInterface } from '../lib/types'

interface Props {
  open: boolean
  onOpenChange: (v: boolean) => void
  onDone: (iface: WireGuardInterface) => void
  existing?: WireGuardInterface | null
}

export default function InterfaceFormDialog({ open, onOpenChange, onDone, existing }: Props) {
  const { push } = useToast()
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
    setBusy(true)
    try {
      const body = {
        name,
        listenPort: Number(listenPort) || 0,
        addresses: parseList(addresses),
        mtu: Number(mtu) || 1420,
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
        push(`Interface ${updated.name} updated`, 'success')
        onDone(updated)
      } else {
        const created = await api.createInterface(body)
        push(`Interface ${created.name} created`, 'success')
        onDone(created)
      }
      onOpenChange(false)
    } catch (err) {
      push(String(err), 'error')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={editing ? `Edit ${existing!.name}` : 'New interface'}
      description={
        editing
          ? 'Changes are applied to the live interface immediately.'
          : 'WireGuard keys are generated automatically. The first address is the server address.'
      }
      footer={
        <>
          <button className="btn-primary" onClick={submit} disabled={busy}>
            {busy ? 'Saving…' : editing ? 'Save changes' : 'Create interface'}
          </button>
          <button className="btn-ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </button>
        </>
      }
    >
      <form onSubmit={submit} className="space-y-4">
        <div>
          <label className="label" htmlFor="iface-name">
            Interface name
          </label>
          <input
            id="iface-name"
            className="input"
            placeholder="wg0"
            value={name}
            disabled={editing}
            onChange={(e) => setName(e.target.value)}
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label" htmlFor="iface-port">
              Listen port
            </label>
            <input
              id="iface-port"
              className="input"
              type="number"
              min={0}
              max={65535}
              placeholder="0 = random"
              value={listenPort}
              onChange={(e) => setListenPort(e.target.value)}
            />
          </div>
          <div>
            <label className="label" htmlFor="iface-mtu">
              MTU
            </label>
            <input
              id="iface-mtu"
              className="input"
              type="number"
              min={576}
              max={65535}
              value={mtu}
              onChange={(e) => setMtu(e.target.value)}
            />
          </div>
        </div>

        <div>
          <label className="label" htmlFor="iface-addresses">
            Addresses (CIDR, one per line)
          </label>
          <textarea
            id="iface-addresses"
            className="input mono"
            rows={2}
            value={addresses}
            onChange={(e) => setAddresses(e.target.value)}
          />
        </div>

        <div>
          <label className="label" htmlFor="iface-dns">
            DNS for clients (one per line)
          </label>
          <textarea
            id="iface-dns"
            className="input mono"
            rows={2}
            value={dns}
            onChange={(e) => setDns(e.target.value)}
          />
        </div>

        <label className="flex cursor-pointer items-center justify-between rounded-lg border border-[#263450] bg-[#0a1426] px-3 py-2.5">
          <span className="text-sm text-slate-300">Create enabled (start now)</span>
          <input
            type="checkbox"
            className="h-4 w-4 accent-sky-500"
            checked={up}
            onChange={(e) => setUp(e.target.checked)}
          />
        </label>
      </form>
    </Dialog>
  )
}