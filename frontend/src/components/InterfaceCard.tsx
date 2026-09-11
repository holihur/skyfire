import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import {
  DotsHorizontalIcon,
  CopyIcon,
  DownloadIcon,
  TrashIcon,
  EyeOpenIcon,
  StopIcon,
  PlayIcon,
} from '@radix-ui/react-icons'
import { api, openDownload } from '../lib/api'
import { useToast } from './Toast'
import { fmtBytes, shortKey } from '../lib/format'
import StatusBadge from './StatusBadge'
import type { WireGuardInterface } from '../lib/types'

interface Props {
  iface: WireGuardInterface
  onDeleted: (name: string) => void
  onEdited: (iface: WireGuardInterface) => void
}

export default function InterfaceCard({ iface, onDeleted, onEdited }: Props) {
  const { push } = useToast()
  const navigate = useNavigate()
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [applying, setApplying] = useState(false)

  const tone = !iface.up ? 'idle' : iface.running ? 'ok' : 'err'
  const status =
    !iface.up ? 'Disabled'
    : iface.running ? 'Running'
    : iface.dryRun ? 'Would run (dry-run)'
    : 'Not running'

  const totalRx = useMemo(() => iface.peers.reduce((a, p) => a + p.transferRx, 0), [iface.peers])
  const totalTx = useMemo(() => iface.peers.reduce((a, p) => a + p.transferTx, 0), [iface.peers])

  const toggle = async () => {
    setApplying(true)
    try {
      const updated = await api.setInterfaceUp(iface.name, !iface.up)
      onEdited(updated)
      push(updated.up ? `${iface.name} enabled` : `${iface.name} disabled`, 'success')
    } catch (err) {
      push(String(err), 'error')
    } finally {
      setApplying(false)
    }
  }

  const del = async () => {
    try {
      await api.deleteInterface(iface.name)
      onDeleted(iface.name)
      push(`Interface ${iface.name} deleted`, 'success')
    } catch (err) {
      push(String(err), 'error')
    }
  }

  const copyPubkey = async () => {
    try {
      await navigator.clipboard.writeText(iface.publicKey)
      push('Public key copied', 'success')
    } catch {
      push('Clipboard unavailable', 'error')
    }
  }

  const copyWgQuick = async () => {
    try {
      const cfg = await api.getText(api.serverConfigUrl(iface.name))
      await navigator.clipboard.writeText(cfg)
      push('Server config copied', 'success')
    } catch (err) {
      push(String(err), 'error')
    }
  }

  return (
    <div className="card group relative overflow-hidden p-5 transition hover:border-[#2e3d5c] hover:shadow-lg hover:shadow-black/30">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <button
              onClick={() => navigate(`/interfaces/${iface.name}`)}
              className="truncate text-lg font-semibold text-white transition group-hover:text-sky-300"
            >
              {iface.name}
            </button>
            <StatusBadge tone={tone} label={status} />
          </div>
          <div className="mt-1 text-xs text-slate-400">
            {iface.up ? `${iface.listenPort || 'random port'} · ${iface.addresses.join(', ')}` : '—'}
          </div>
        </div>

        <DropdownMenu.Root>
          <DropdownMenu.Trigger asChild>
            <button className="btn-ghost !p-2" aria-label="Actions">
              <DotsHorizontalIcon className="h-4 w-4" />
            </button>
          </DropdownMenu.Trigger>
          <DropdownMenu.Portal>
            <DropdownMenu.Content
              align="end"
              sideOffset={6}
              className="z-50 min-w-44 rounded-xl border border-[#263450] bg-[#0d1830] p-1 shadow-2xl shadow-black/50"
            >
              <DropdownMenu.Item
                className="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-slate-300 hover:bg-white/5 hover:text-white"
                onSelect={() => navigate(`/interfaces/${iface.name}`)}
              >
                <EyeOpenIcon className="h-4 w-4" /> Details
              </DropdownMenu.Item>
              <DropdownMenu.Item
                className="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-slate-300 hover:bg-white/5 hover:text-white"
                onSelect={toggle}
                disabled={applying}
              >
                {iface.up ? <StopIcon className="h-4 w-4" /> : <PlayIcon className="h-4 w-4" />}
                {iface.up ? 'Disable' : 'Enable'}
              </DropdownMenu.Item>
              <DropdownMenu.Item
                className="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-slate-300 hover:bg-white/5 hover:text-white"
                onSelect={copyPubkey}
              >
                <CopyIcon className="h-4 w-4" /> Copy public key
              </DropdownMenu.Item>
              <DropdownMenu.Item
                className="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-slate-300 hover:bg-white/5 hover:text-white"
                onSelect={copyWgQuick}
              >
                <CopyIcon className="h-4 w-4" /> Copy server config
              </DropdownMenu.Item>
              <DropdownMenu.Item
                className="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-slate-300 hover:bg-white/5 hover:text-white"
                onSelect={() => openDownload(api.serverConfigUrl(iface.name))}
              >
                <DownloadIcon className="h-4 w-4" /> Download config
              </DropdownMenu.Item>
              <DropdownMenu.Separator className="my-1 h-px bg-[#263450]" />
              <DropdownMenu.Item
                className="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-red-300 hover:bg-red-950/40 hover:text-red-100"
                onSelect={() => setConfirmDelete(true)}
              >
                <TrashIcon className="h-4 w-4" /> Delete
              </DropdownMenu.Item>
            </DropdownMenu.Content>
          </DropdownMenu.Portal>
        </DropdownMenu.Root>
      </div>

      <div className="mt-4 grid grid-cols-3 gap-3 text-center">
        <Stat label="Peers" value={`${iface.connectedPeers}/${iface.totalPeers}`} sub="connected/total" />
        <Stat label="Download" value={fmtBytes(totalRx)} sub="received" />
        <Stat label="Upload" value={fmtBytes(totalTx)} sub="sent" />
      </div>

      <div className="mt-4 flex items-center justify-between border-t border-[#1e2a45] pt-3">
        <span className="mono text-[11px] text-slate-500">{shortKey(iface.publicKey, 22)}</span>
        <button
          className="btn-basic"
          onClick={() => navigate(`/interfaces/${iface.name}`)}
        >
          Configure
        </button>
      </div>

      {confirmDelete && (
        <div className="absolute inset-0 z-10 flex items-center justify-center rounded-2xl bg-black/70 backdrop-blur-sm">
          <div className="w-64 space-y-3 rounded-xl border border-red-800/60 bg-[#12060a] p-4">
            <p className="text-sm text-red-200">
              Delete <span className="font-semibold">{iface.name}</span> and all its peers?
            </p>
            <div className="flex justify-end gap-2">
              <button className="btn-ghost" onClick={() => setConfirmDelete(false)}>
                Cancel
              </button>
              <button className="btn-danger" onClick={del}>
                Delete
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function Stat({ label, value, sub }: { label: string; value: string; sub: string }) {
  return (
    <div className="rounded-xl border border-[#1e2a45] bg-[#0a1426] px-2 py-2.5">
      <div className="text-[11px] uppercase tracking-wide text-slate-500">{label}</div>
      <div className="mt-0.5 text-base font-semibold text-white">{value}</div>
      <div className="text-[11px] text-slate-500">{sub}</div>
    </div>
  )
}