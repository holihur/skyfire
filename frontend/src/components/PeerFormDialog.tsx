import { useEffect, useState } from 'react'
import Dialog from './Dialog'
import { api } from '../lib/api'
import { useToast } from './Toast'
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
        push('Peer updated', 'success')
      } else {
        await api.addPeer(ifaceName, input)
        push('Peer created', 'success')
      }
      onOpenChange(false)
      onDone?.()
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
      title={editing ? `Edit peer · ${peer!.name}` : 'Add peer'}
      description={
        editing
          ? 'Updates are applied to the live interface immediately.'
          : 'A client keypair is generated automatically. Leave the address empty to auto-assign from the subnet.'
      }
      footer={
        <>
          <button className="btn-primary" onClick={submit} disabled={busy}>
            {busy ? 'Saving…' : 'Save peer'}
          </button>
          <button className="btn-ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label" htmlFor="peer-name">
              Name
            </label>
            <input
              id="peer-name"
              className="input"
              placeholder="laptop"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div>
            <label className="label" htmlFor="peer-address">
              Assigned address
            </label>
            <input
              id="peer-address"
              className="input"
              placeholder="auto-assign"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label" htmlFor="peer-keepalive">
              Persistent keepalive (s)
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
              Endpoint (optional)
            </label>
            <input
              id="peer-endpoint"
              className="input"
              placeholder="203.0.113.5:51820"
              value={endpoint}
              onChange={(e) => setEndpoint(e.target.value)}
            />
          </div>
        </div>

        <div>
          <label className="label" htmlFor="peer-routes">
            Client routes (AllowedIPs pushed to the client)
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
            Client DNS (optional, one per line)
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
            Description
          </label>
          <input
            id="peer-desc"
            className="input"
            placeholder="Optional notes"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>

        <label className="flex cursor-pointer items-center justify-between rounded-lg border border-[#263450] bg-[#0a1426] px-3 py-2.5">
          <span className="text-sm text-slate-300">Enabled</span>
          <input
            type="checkbox"
            className="h-4 w-4 accent-sky-500"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
          />
        </label>

        <div className="rounded-lg border border-[#263450] bg-[#0a1426] p-3">
          <label className="flex cursor-pointer items-center justify-between">
            <span className="text-sm text-slate-300">Use a preshared key</span>
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
              placeholder='Empty or "generate" creates one automatically'
              value={psk}
              onChange={(e) => setPsk(e.target.value)}
            />
          )}
        </div>

        {editing && (
          <label className="flex cursor-pointer items-center justify-between rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2.5">
            <span className="text-sm text-amber-200">Rotate keys (new keypair)</span>
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