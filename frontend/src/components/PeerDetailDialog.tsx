import { useState } from 'react'
import * as Tabs from '@radix-ui/react-tabs'
import { CopyIcon, DownloadIcon } from '@radix-ui/react-icons'
import Dialog from './Dialog'
import { api, openDownload } from '../lib/api'
import { useToast } from './Toast'
import { fmtBytes, fmtAge, shortKey } from '../lib/format'
import type { Peer } from '../lib/types'

interface Props {
  open: boolean
  onOpenChange: (v: boolean) => void
  ifaceName: string
  peer: Peer
  onEdited?: () => void
  onDeleted?: (pub: string) => void
}

export default function PeerDetailDialog({ open, onOpenChange, ifaceName, peer, onEdited, onDeleted }: Props) {
  const { push } = useToast()
  const [tab, setTab] = useState('qr')
  const [config, setConfig] = useState('')
  const [confirmDelete, setConfirmDelete] = useState(false)

  const loadConfig = async () => {
    try {
      const cfg = await api.getText(api.peerConfigUrl(ifaceName, peer.publicKey))
      setConfig(cfg)
    } catch (err) {
      push(String(err), 'error')
    }
  }

  const copyConfig = async () => {
    if (!config) await loadConfig()
    try {
      await navigator.clipboard.writeText(config)
      push('Configuration copied', 'success')
    } catch {
      push('Clipboard unavailable', 'error')
    }
  }

  const doDelete = async () => {
    try {
      await api.deletePeer(ifaceName, peer.publicKey)
      onDeleted?.(peer.publicKey)
      onOpenChange(false)
      push('Peer removed', 'success')
    } catch (err) {
      push(String(err), 'error')
    }
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const tabTrigger = 'rounded-lg px-3 py-1.5 text-sm text-slate-400 transition hover:text-white data-selected:bg-white/5 data-selected:text-white'

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={peer.name || 'Peer'}
      description={shortKey(peer.publicKey, 24)}
      width="max-w-xl"
      footer={
        <div className="flex items-center gap-3">
          <div className="mr-auto flex gap-2">
            <button className="btn-ghost" onClick={() => onEdited?.()}>
              Edit
            </button>
            <button className="btn-danger" onClick={() => setConfirmDelete(true)}>
              Delete
            </button>
          </div>
          <button className="btn-ghost" onClick={copyConfig}>
            <CopyIcon className="h-4 w-4" /> Copy
          </button>
          <button
            className="btn-ghost"
            onClick={() => openDownload(api.peerConfigUrl(ifaceName, peer.publicKey))}
          >
            <DownloadIcon className="h-4 w-4" /> Download
          </button>
        </div>
      }
    >
      {confirmDelete && (
        <div className="mb-4 rounded-xl border border-red-800/60 bg-red-950/30 p-4">
          <p className="text-sm text-red-200">
            Remove <span className="font-semibold">{peer.name}</span>? Its client
            configuration will stop working.
          </p>
          <div className="mt-3 flex justify-end gap-2">
            <button className="btn-ghost" onClick={() => setConfirmDelete(false)}>
              Cancel
            </button>
            <button className="btn-danger" onClick={doDelete}>
              Remove peer
            </button>
          </div>
        </div>
      )}

      <Tabs.Root value={tab} onValueChange={setTab}>
        <Tabs.List className="mb-4 flex gap-1 rounded-xl border border-[#263450] bg-[#0a1426] p-1">
          <Tabs.Trigger value="qr" className={tabTrigger}>
            QR code
          </Tabs.Trigger>
          <Tabs.Trigger value="conf" className={tabTrigger}>
            Configuration
          </Tabs.Trigger>
          <Tabs.Trigger value="info" className={tabTrigger}>
            Info
          </Tabs.Trigger>
        </Tabs.List>

        <Tabs.Content value="qr" className="flex flex-col items-center gap-4">
          <div className="rounded-2xl border border-[#263450] bg-white p-4">
            <img
              src={api.peerQrUrl(ifaceName, peer.publicKey)}
              alt={`${peer.name} QR code`}
              className="h-64 w-64"
              onLoad={loadConfig}
            />
          </div>
          <p className="text-center text-xs text-slate-400">
            Scan with the WireGuard mobile app (import from QR code).
          </p>
        </Tabs.Content>

        <Tabs.Content value="conf">
          <pre className="mono max-h-80 overflow-auto rounded-xl border border-[#263450] bg-[#070d1a] p-4 text-slate-300">
            {config || 'Loading…'}
          </pre>
        </Tabs.Content>

        <Tabs.Content value="info">
          <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
            <Info label="Name" value={peer.name} />
            <Info label="Address" value={peer.address} />
            <Info label="Public key" value={shortKey(peer.publicKey, 32)} mono />
            {peer.presharedKey && (
              <Info label="Preshared key" value={`present`} />
            )}
            <Info label="Endpoint" value={peer.endpoint || '—'} mono />
            <Info label="Status" value={peer.connected ? 'Connected' : 'Idle'} />
            <Info label="Last handshake" value={fmtAge(peer.latestHandshake)} />
            <Info label="Transfer" value={`↑ ${fmtBytes(peer.transferTx)} · ↓ ${fmtBytes(peer.transferRx)}`} />
            <Info label="Client routes" value={peer.clientRoutes.join(', ')} mono />
            <Info label="Keepalive" value={peer.persistentKeepalive ? `${peer.persistentKeepalive}s` : 'off'} />
          </dl>
        </Tabs.Content>
      </Tabs.Root>
    </Dialog>
  )
}

function Info({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <>
      <dt className="text-xs uppercase tracking-wide text-slate-500">{label}</dt>
      <dd className={`break-all text-slate-200 ${mono ? 'mono' : ''}`}>{value || '—'}</dd>
    </>
  )
}