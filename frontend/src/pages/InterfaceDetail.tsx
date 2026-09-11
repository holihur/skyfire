import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import * as Switch from '@radix-ui/react-switch'
import * as Tabs from '@radix-ui/react-tabs'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import {
  ArrowLeftIcon,
  CopyIcon,
  DownloadIcon,
  DotsHorizontalIcon,
  EyeClosedIcon,
  EyeOpenIcon,
  MobileIcon,
  Pencil1Icon,
  PlusIcon,
  TrashIcon,
} from '@radix-ui/react-icons'
import { api, openDownload } from '../lib/api'
import { useToast } from '../components/Toast'
import { fmtBytes, fmtAge, shortKey, ipOf } from '../lib/format'
import StatusBadge from '../components/StatusBadge'
import PeerFormDialog from '../components/PeerFormDialog'
import PeerDetailDialog from '../components/PeerDetailDialog'
import InterfaceFormDialog from '../components/InterfaceFormDialog'
import ConfirmDialog from '../components/ConfirmDialog'
import type { Peer, WireGuardInterface } from '../lib/types'

export default function InterfaceDetail() {
  const { name = '' } = useParams()
  const { push } = useToast()
  const [iface, setIface] = useState<WireGuardInterface | null>(null)
  const [showPri, setShowPri] = useState(false)
  const [priKey, setPriKey] = useState('')
  const [peerForm, setPeerForm] = useState<{ open: boolean; peer: Peer | null }>({ open: false, peer: null })
  const [peerDetail, setPeerDetail] = useState<Peer | null>(null)
  const [editIface, setEditIface] = useState(false)
  const [deletePeerTarget, setDeletePeerTarget] = useState<Peer | null>(null)
  const [serverConf, setServerConf] = useState('')

  const load = useCallback(async () => {
    try {
      setIface(await api.interface(name))
    } catch (err) {
      push(String(err), 'error')
    }
  }, [name, push])

  useEffect(() => {
    void load()
    const id = setInterval(load, 4000)
    return () => clearInterval(id)
  }, [load])

  useEffect(() => {
    api
      .getText(api.serverConfigUrl(name))
      .then(setServerConf)
      .catch(() => {})
  }, [name])

  const totalRx = useMemo(
    () => (iface ? iface.peers.reduce((a, p) => a + p.transferRx, 0) : 0),
    [iface],
  )
  const totalTx = useMemo(
    () => (iface ? iface.peers.reduce((a, p) => a + p.transferTx, 0) : 0),
    [iface],
  )

  if (!iface) {
    return (
      <div className="mx-auto max-w-6xl py-24 text-center text-slate-400">
        Loading interface…
      </div>
    )
  }

  const tone = !iface.up ? 'idle' : iface.running ? 'ok' : 'err'
  const status = !iface.up ? 'Disabled' : iface.running ? 'Running' : iface.dryRun ? 'Would run (dry-run)' : 'Not running'

  const toggleUp = async () => {
    try {
      const updated = await api.setInterfaceUp(iface.name, !iface.up)
      setIface(updated)
      push(updated.up ? 'Interface enabled' : 'Interface disabled', 'success')
    } catch (err) {
      push(String(err), 'error')
    }
  }

  const revealPri = async () => {
    if (!showPri) {
      try {
        setPriKey((await api.privateKey(iface.name)).privateKey)
      } catch (err) {
        push(String(err), 'error')
        return
      }
    }
    setShowPri((v) => !v)
  }

  const copy = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text)
      push(`${label} copied`, 'success')
    } catch {
      push('Clipboard unavailable', 'error')
    }
  }

  const deletePeer = async (p: Peer) => {
    try {
      await api.deletePeer(iface.name, p.publicKey)
      setDeletePeerTarget(null)
      await load()
      push(`Peer ${p.name} removed`, 'success')
    } catch (err) {
      push(String(err), 'error')
    }
  }

  return (
    <div className="mx-auto max-w-6xl">
      <Link
        to="/"
        className="mb-4 inline-flex items-center gap-1.5 text-sm text-slate-400 transition hover:text-white"
      >
        <ArrowLeftIcon className="h-4 w-4" /> Interfaces
      </Link>

      <div className="card p-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div className="flex flex-wrap items-center gap-3">
              <h1 className="text-2xl font-semibold text-white">{iface.name}</h1>
              <StatusBadge tone={tone} label={status} />
              <label className="flex cursor-pointer items-center gap-2">
                <Switch.Root
                  checked={iface.up}
                  onCheckedChange={toggleUp}
                  className="relative h-6 w-11 rounded-full bg-[#1e2a45] transition data-[state=checked]:bg-sky-600"
                >
                  <Switch.Thumb className="block h-5 w-5 translate-x-0.5 rounded-full bg-white transition data-[state=checked]:translate-x-[22px]" />
                </Switch.Root>
                <span className="text-xs text-slate-400">
                  {iface.up ? 'enabled' : 'disabled'}
                </span>
              </label>
            </div>
            <dl className="mt-3 grid grid-cols-2 gap-x-8 gap-y-2 text-sm sm:grid-cols-3 lg:grid-cols-4">
              <KV k="Listen port" v={iface.listenPort ? String(iface.listenPort) : 'random'} />
              <KV k="Addresses" v={iface.addresses.join(', ')} mono />
              <KV k="MTU" v={String(iface.mtu)} />
              <KV k="Peers" v={`${iface.connectedPeers}/${iface.totalPeers} connected`} />
            </dl>
          </div>

          <div className="flex flex-col items-end gap-3">
            <div className="grid grid-cols-2 gap-3 text-center">
              <MiniStat label="Download" value={fmtBytes(totalRx)} />
              <MiniStat label="Upload" value={fmtBytes(totalTx)} />
            </div>
            <div className="flex gap-2">
              <button className="btn-ghost" onClick={() => setEditIface(true)}>
                <Pencil1Icon className="h-4 w-4" /> Edit
              </button>
              <button className="btn-ghost" onClick={() => openDownload(api.serverConfigUrl(iface.name))}>
                <DownloadIcon className="h-4 w-4" /> Server config
              </button>
            </div>
          </div>
        </div>

        <div className="mt-5 space-y-2 border-t border-[#1e2a45] pt-4">
          <KeyRow
            label="Public key"
            value={iface.publicKey}
            onCopy={() => copy(iface.publicKey, 'Public key')}
          />
          <KeyRow
            label="Private key"
            value={showPri ? priKey : '••••••••••••••••••••••••••••••••'}
            masked={!showPri}
            action={
              <button
                className="btn-ghost !p-1.5"
                title="Toggle private key"
                onClick={revealPri}
              >
                {showPri ? <EyeOpenIcon className="h-4 w-4" /> : <EyeClosedIcon className="h-4 w-4" />}
              </button>
            }
            onCopy={
              showPri
                ? () => copy(priKey, 'Private key')
                : () => push('Reveal the private key first', 'info')
            }
          />
        </div>
      </div>

      <Tabs.Root defaultValue="peers" className="mt-6">
        <Tabs.List className="mb-4 flex items-center justify-between gap-3">
          <div className="flex gap-1 rounded-xl border border-[#263450] bg-[#0a1426] p-1">
            <Tabs.Trigger value="peers" className={tabCls}>
              Peers ({iface.peers.length})
            </Tabs.Trigger>
            <Tabs.Trigger value="conf" className={tabCls}>
              Server config
            </Tabs.Trigger>
          </div>
          <Tabs.Content value="peers" className="hidden">
            <span />
          </Tabs.Content>
          <button className="btn-primary" onClick={() => setPeerForm({ open: true, peer: null })}>
            <PlusIcon className="h-4 w-4" /> Add peer
          </button>
        </Tabs.List>

        <Tabs.Content value="peers">
          {iface.peers.length === 0 ? (
            <div className="card flex flex-col items-center gap-3 py-16 text-center">
              <span className="text-3xl">👥</span>
              <p className="text-slate-400">
                No peers yet. Add devices like phones and laptops.
              </p>
              <button className="btn-primary" onClick={() => setPeerForm({ open: true, peer: null })}>
                <PlusIcon className="h-4 w-4" /> Add first peer
              </button>
            </div>
          ) : (
            <div className="card overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-[#1e2a45] text-xs uppercase tracking-wide text-slate-500">
                      <th className="px-4 py-3 font-medium">Peer</th>
                      <th className="px-4 py-3 font-medium">Address</th>
                      <th className="hidden px-4 py-3 font-medium lg:table-cell">Public key</th>
                      <th className="hidden px-4 py-3 font-medium md:table-cell">Status</th>
                      <th className="hidden px-4 py-3 font-medium md:table-cell">Last handshake</th>
                      <th className="hidden px-4 py-3 font-medium sm:table-cell">Transfer</th>
                      <th className="px-4 py-3 text-right font-medium">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {iface.peers.map((p) => (
                      <tr
                        key={p.publicKey}
                        className="border-b border-[#161f33] last:border-0 hover:bg-white/[0.02]"
                      >
                        <td className="px-4 py-3">
                          <button
                            className="font-medium text-slate-100 hover:text-sky-300"
                            onClick={() => setPeerDetail(p)}
                          >
                            {p.name}
                          </button>
                          <div className="text-xs text-slate-500">
                            {p.endpoint || p.presharedKey ? 'psk·on' : ''}
                            {p.endpoint ? ` · ${p.endpoint}` : ''}
                          </div>
                        </td>
                        <td className="mono px-4 py-3 text-slate-300">{ipOf(p.address)}</td>
                        <td className="hidden lg:table-cell">
                          <button
                            className="mono text-slate-400 hover:text-sky-300"
                            onClick={() => copy(p.publicKey, 'Public key')}
                            title="Copy public key"
                          >
                            {shortKey(p.publicKey, 20)}
                          </button>
                        </td>
                        <td className="hidden md:table-cell">
                          {p.connected ? (
                            <StatusBadge tone="ok" label="connected" />
                          ) : (
                            <StatusBadge tone="idle" label="idle" />
                          )}
                        </td>
                        <td className="hidden text-slate-400 md:table-cell">
                          {fmtAge(p.latestHandshake)}
                        </td>
                        <td className="mono hidden text-slate-300 sm:table-cell">
                          ↑ {fmtBytes(p.transferTx)} ↓ {fmtBytes(p.transferRx)}
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex justify-end gap-1">
                            <IconBtn title="Show config & QR" onClick={() => setPeerDetail(p)}>
                              <MobileIcon className="h-4 w-4" />
                            </IconBtn>
                            <IconBtn
                              title="Download client config"
                              onClick={() => openDownload(api.peerConfigUrl(iface.name, p.publicKey))}
                            >
                              <DownloadIcon className="h-4 w-4" />
                            </IconBtn>
                            <DropdownMenu.Root>
                              <DropdownMenu.Trigger asChild>
                                <IconBtn title="More">
                                  <DotsHorizontalIcon className="h-4 w-4" />
                                </IconBtn>
                              </DropdownMenu.Trigger>
                              <DropdownMenu.Portal>
                                <DropdownMenu.Content
                                  align="end"
                                  sideOffset={6}
                                  className="z-50 min-w-40 rounded-xl border border-[#263450] bg-[#0d1830] p-1 shadow-2xl shadow-black/50"
                                >
                                  <DropdownMenu.Item
                                    className="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-slate-300 hover:bg-white/5 hover:text-white"
                                    onSelect={() => setPeerForm({ open: true, peer: p })}
                                  >
                                    <Pencil1Icon className="h-4 w-4" /> Edit
                                  </DropdownMenu.Item>
                                  <DropdownMenu.Item
                                    className="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-red-300 hover:bg-red-950/40 hover:text-red-100"
                                    onSelect={() => setDeletePeerTarget(p)}
                                  >
                                    <TrashIcon className="h-4 w-4" /> Delete
                                  </DropdownMenu.Item>
                                </DropdownMenu.Content>
                              </DropdownMenu.Portal>
                            </DropdownMenu.Root>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </Tabs.Content>

        <Tabs.Content value="conf">
          <div className="card p-5">
            <div className="mb-3 flex items-center justify-between">
              <span className="text-sm text-slate-400">wg-quick style export</span>
              <button
                className="btn-ghost"
                onClick={() => {
                  navigator.clipboard.writeText(serverConf)
                  push('Server config copied', 'success')
                }}
              >
                <CopyIcon className="h-4 w-4" /> Copy
              </button>
            </div>
            <pre className="mono max-h-96 overflow-auto rounded-xl border border-[#1e2a45] bg-[#070d1a] p-4 text-slate-300">
              {serverConf}
            </pre>
          </div>
        </Tabs.Content>
      </Tabs.Root>

      {peerForm.open && (
        <PeerFormDialog
          open={peerForm.open}
          onOpenChange={(v) => !v && setPeerForm({ ...peerForm, open: false })}
          ifaceName={iface.name}
          peer={peerForm.peer}
          onDone={() => {
            setPeerForm({ open: false, peer: null })
            void load()
          }}
        />
      )}
      {peerDetail && (
        <PeerDetailDialog
          open={!!peerDetail}
          onOpenChange={(v) => !v && setPeerDetail(null)}
          ifaceName={iface.name}
          peer={peerDetail}
          onEdited={() => {
            setPeerDetail(null)
            setPeerForm({ open: true, peer: peerDetail })
          }}
          onDeleted={() => void load()}
        />
      )}
      {deletePeerTarget && (
        <ConfirmDialog open onOpenChange={() => setDeletePeerTarget(null)}
          title={`Delete ${deletePeerTarget.name}?`}
          danger
          confirmLabel="Delete peer"
          onConfirm={() => void deletePeer(deletePeerTarget)}
        >
          <p>This removes the peer and its client configuration will stop working.</p>
        </ConfirmDialog>
      )}
      {editIface && (
        <InterfaceFormDialog
          open={editIface}
          onOpenChange={setEditIface}
          existing={iface}
          onDone={(updated) => {
            setEditIface(false)
            setIface(updated)
          }}
        />
      )}
    </div>
  )
}

function KeyRow({ label, value, masked, onCopy, action }: {
  label: string
  value: string
  masked?: boolean
  onCopy: () => void
  action?: React.ReactNode
}) {
  return (
    <div className="flex items-center gap-2">
      <span className="w-24 shrink-0 text-xs uppercase tracking-wide text-slate-500">{label}</span>
      <code className={`mono flex-1 truncate rounded-lg border border-[#1e2a45] bg-[#0a1426] px-3 py-2 text-slate-300 ${masked ? 'tracking-widest' : ''}`}>
        {value}
      </code>
      {action}
      <button className="btn-ghost !p-1.5" title={`Copy ${label.toLowerCase()}`} onClick={onCopy}>
        <CopyIcon className="h-4 w-4" />
      </button>
    </div>
  )
}

function KV({ k, v, mono }: { k: string; v: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-[11px] uppercase tracking-wide text-slate-500">{k}</dt>
      <dd className={`truncate text-slate-200 ${mono ? 'mono' : ''}`}>{v}</dd>
    </div>
  )
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-28 rounded-xl border border-[#1e2a45] bg-[#0a1426] px-4 py-2.5">
      <div className="text-[11px] uppercase tracking-wide text-slate-500">{label}</div>
      <div className="mono text-sm text-sky-200">{value}</div>
    </div>
  )
}

function IconBtn({ title, onClick, children }: { title: string; onClick?: () => void; children: React.ReactNode }) {
  return (
    <button
      className="btn-ghost !p-1.5"
      title={title}
      onClick={onClick}
      type="button"
    >
      {children}
    </button>
  )
}

const tabCls =
  'rounded-lg px-3 py-1.5 text-sm text-slate-400 transition hover:text-white data-selected:bg-white/5 data-selected:text-white'