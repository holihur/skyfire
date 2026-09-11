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
import { useI18n } from '../i18n'
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
  const { t, lang } = useI18n()
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
      <div className="mx-auto max-w-6xl py-24 text-center text-muted">
        {t('iface.loading')}
      </div>
    )
  }

  const tone = !iface.up ? 'idle' : iface.running ? 'ok' : 'err'
  const status = !iface.up
    ? t('iface.disabled')
    : iface.running
      ? t('iface.running')
      : iface.dryRun
        ? t('iface.wouldRun')
        : t('iface.notRunning')

  const toggleUp = async () => {
    try {
      const updated = await api.setInterfaceUp(iface.name, !iface.up)
      setIface(updated)
      push(updated.up ? t('iface.update.on') : t('iface.update.off'), 'success')
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
      push(t('common.copied', { label }), 'success')
    } catch {
      push(t('common.clipboardUnavailable'), 'error')
    }
  }

  const deletePeer = async (p: Peer) => {
    try {
      await api.deletePeer(iface.name, p.publicKey)
      setDeletePeerTarget(null)
      await load()
      push(t('iface.peer.removeToast', { name: p.name }), 'success')
    } catch (err) {
      push(String(err), 'error')
    }
  }

  const menuItem =
    'flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-fg2 hover:bg-hover/10 hover:text-fg'

  return (
    <div className="mx-auto max-w-6xl">
      <Link
        to="/"
        className="mb-4 inline-flex items-center gap-1.5 text-sm text-muted transition hover:text-fg"
      >
        <ArrowLeftIcon className="h-4 w-4" /> {t('iface.back')}
      </Link>

      <div className="card p-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div className="flex flex-wrap items-center gap-3">
              <h1 className="text-2xl font-semibold text-fg">{iface.name}</h1>
              <StatusBadge tone={tone} label={status} />
              <label className="flex cursor-pointer items-center gap-2">
                <Switch.Root
                  checked={iface.up}
                  onCheckedChange={toggleUp}
                  className="relative h-6 w-11 rounded-full bg-edge2 transition data-[state=checked]:bg-sky-600"
                >
                  <Switch.Thumb className="block h-5 w-5 translate-x-0.5 rounded-full bg-white transition data-[state=checked]:translate-x-[22px]" />
                </Switch.Root>
                <span className="text-xs text-muted">{iface.up ? t('iface.enabled') : t('iface.disabled')}</span>
              </label>
            </div>
            <dl className="mt-3 grid grid-cols-2 gap-x-8 gap-y-2 text-sm sm:grid-cols-3 lg:grid-cols-4">
              <KV k={t('iface.listenPort')} v={iface.listenPort ? String(iface.listenPort) : t('iface.randomPort')} />
              <KV k={t('iface.addresses')} v={iface.addresses.join(', ')} mono />
              <KV k="MTU" v={String(iface.mtu)} />
              <KV k={t('iface.stat.peers')} v={t('iface.peersCount', { conn: iface.connectedPeers, total: iface.totalPeers })} />
            </dl>
          </div>

          <div className="flex flex-col items-end gap-3">
            <div className="grid grid-cols-2 gap-3 text-center">
              <MiniStat label={t('iface.stat.download')} value={fmtBytes(totalRx)} />
              <MiniStat label={t('iface.stat.upload')} value={fmtBytes(totalTx)} />
            </div>
            <div className="flex gap-2">
              <button className="btn-ghost" onClick={() => setEditIface(true)}>
                <Pencil1Icon className="h-4 w-4" /> {t('iface.edit')}
              </button>
              <button className="btn-ghost" onClick={() => openDownload(api.serverConfigUrl(iface.name))}>
                <DownloadIcon className="h-4 w-4" /> {t('iface.srvConf')}
              </button>
            </div>
          </div>
        </div>

        <div className="mt-5 space-y-2 border-t border-edge pt-4">
          <KeyRow
            label={t('iface.pubkey')}
            value={iface.publicKey}
            onCopy={() => copy(iface.publicKey, t('iface.pubkey'))}
          />
          <KeyRow
            label={t('iface.prikey')}
            value={showPri ? priKey : '••••••••••••••••••••••••••••••••'}
            masked={!showPri}
            action={
              <button
                className="btn-ghost !p-1.5"
                title={t('iface.revealPriFirst')}
                onClick={revealPri}
              >
                {showPri ? <EyeOpenIcon className="h-4 w-4" /> : <EyeClosedIcon className="h-4 w-4" />}
              </button>
            }
            onCopy={
              showPri
                ? () => copy(priKey, t('iface.prikey'))
                : () => push(t('iface.revealPriFirst'), 'info')
            }
          />
        </div>
      </div>

      <Tabs.Root defaultValue="peers" className="mt-6">
        <Tabs.List className="mb-4 flex items-center justify-between gap-3">
          <div className="flex gap-1 rounded-xl border border-edge2 bg-inset p-1">
            <Tabs.Trigger value="peers" className={tabCls}>
              {t('iface.stat.peers')} ({iface.peers.length})
            </Tabs.Trigger>
            <Tabs.Trigger value="conf" className={tabCls}>
              {t('iface.srvConf')}
            </Tabs.Trigger>
          </div>
          <Tabs.Content value="peers" className="hidden">
            <span />
          </Tabs.Content>
          <button className="btn-primary" onClick={() => setPeerForm({ open: true, peer: null })}>
            <PlusIcon className="h-4 w-4" /> {t('iface.addPeer')}
          </button>
        </Tabs.List>

        <Tabs.Content value="peers">
          {iface.peers.length === 0 ? (
            <div className="card flex flex-col items-center gap-3 py-16 text-center">
              <span className="text-3xl">👥</span>
              <p className="text-muted">{t('iface.peer.noPeers')}</p>
              <button className="btn-primary" onClick={() => setPeerForm({ open: true, peer: null })}>
                <PlusIcon className="h-4 w-4" /> {t('iface.addfirstPeer')}
              </button>
            </div>
          ) : (
            <div className="card overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-edge text-xs uppercase tracking-wide text-faint">
                      <th className="px-4 py-3 font-medium">{t('iface.peerTable.name')}</th>
                      <th className="px-4 py-3 font-medium">{t('iface.peerTable.address')}</th>
                      <th className="hidden px-4 py-3 font-medium lg:table-cell">{t('iface.peerTable.pubkey')}</th>
                      <th className="hidden px-4 py-3 font-medium md:table-cell">{t('iface.peerTable.status')}</th>
                      <th className="hidden px-4 py-3 font-medium md:table-cell">{t('iface.peerTable.handshake')}</th>
                      <th className="hidden px-4 py-3 font-medium sm:table-cell">{t('iface.peerTable.transfer')}</th>
                      <th className="px-4 py-3 text-right font-medium">{t('iface.actions')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {iface.peers.map((p) => (
                      <tr
                        key={p.publicKey}
                        className="border-b border-edge3 last:border-0 hover:bg-hover/5"
                      >
                        <td className="px-4 py-3">
                          <button
                            className="font-medium text-fg hover:text-brand"
                            onClick={() => setPeerDetail(p)}
                          >
                            {p.name}
                          </button>
                          <div className="text-xs text-faint">
                            {p.endpoint || p.presharedKey ? 'psk·on' : ''}
                            {p.endpoint ? ` · ${p.endpoint}` : ''}
                          </div>
                        </td>
                        <td className="mono px-4 py-3 text-fg2">{ipOf(p.address)}</td>
                        <td className="hidden lg:table-cell">
                          <button
                            className="mono text-muted hover:text-brand"
                            onClick={() => copy(p.publicKey, t('iface.pubkey'))}
                            title={t('iface.peer.copyPubkey')}
                          >
                            {shortKey(p.publicKey, 20)}
                          </button>
                        </td>
                        <td className="hidden md:table-cell">
                          {p.connected ? (
                            <StatusBadge tone="ok" label={t('iface.peer.connected')} />
                          ) : (
                            <StatusBadge tone="idle" label={t('iface.peer.idle')} />
                          )}
                        </td>
                        <td className="hidden text-muted md:table-cell">
                          {fmtAge(p.latestHandshake, lang)}
                        </td>
                        <td className="mono hidden text-fg2 sm:table-cell">
                          ↑ {fmtBytes(p.transferTx)} ↓ {fmtBytes(p.transferRx)}
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex justify-end gap-1">
                            <IconBtn title={t('iface.peer.showQR')} onClick={() => setPeerDetail(p)}>
                              <MobileIcon className="h-4 w-4" />
                            </IconBtn>
                            <IconBtn
                              title={t('iface.peer.downloadClient')}
                              onClick={() => openDownload(api.peerConfigUrl(iface.name, p.publicKey))}
                            >
                              <DownloadIcon className="h-4 w-4" />
                            </IconBtn>
                            <DropdownMenu.Root>
                              <DropdownMenu.Trigger asChild>
                                <IconBtn title={t('iface.peer.more')}>
                                  <DotsHorizontalIcon className="h-4 w-4" />
                                </IconBtn>
                              </DropdownMenu.Trigger>
                              <DropdownMenu.Portal>
                                <DropdownMenu.Content
                                  align="end"
                                  sideOffset={6}
                                  className="z-50 min-w-40 rounded-xl border border-edge2 bg-panel2 p-1 shadow-2xl shadow-shade/50"
                                >
                                  <DropdownMenu.Item
                                    className={menuItem}
                                    onSelect={() => setPeerForm({ open: true, peer: p })}
                                  >
                                    <Pencil1Icon className="h-4 w-4" /> {t('iface.editPeer')}
                                  </DropdownMenu.Item>
                                  <DropdownMenu.Item
                                    className="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-err hover:bg-err/10 hover:text-err"
                                    onSelect={() => setDeletePeerTarget(p)}
                                  >
                                    <TrashIcon className="h-4 w-4" /> {t('iface.delete')}
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
              <span className="text-sm text-muted">{t('iface.conf.wgQuick')}</span>
              <button
                className="btn-ghost"
                onClick={() => {
                  navigator.clipboard.writeText(serverConf)
                  push(t('iface.copySrvConf'), 'success')
                }}
              >
                <CopyIcon className="h-4 w-4" /> {t('iface.conf.copy')}
              </button>
            </div>
            <pre className="mono max-h-96 overflow-auto rounded-xl border border-edge bg-page p-4 text-fg2">
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
        <ConfirmDialog
          open
          onOpenChange={() => setDeletePeerTarget(null)}
          title={t('iface.peer.removeTitle', { name: deletePeerTarget.name })}
          danger
          confirmLabel={t('iface.peer.deletePeer')}
          onConfirm={() => void deletePeer(deletePeerTarget)}
        >
          <p>{t('iface.peer.removeBody')}</p>
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
      <span className="w-24 shrink-0 text-xs uppercase tracking-wide text-faint">{label}</span>
      <code className={`mono flex-1 truncate rounded-lg border border-edge bg-inset px-3 py-2 text-fg2 ${masked ? 'tracking-widest' : ''}`}>
        {value}
      </code>
      {action}
      <button className="btn-ghost !p-1.5" title={label} onClick={onCopy}>
        <CopyIcon className="h-4 w-4" />
      </button>
    </div>
  )
}

function KV({ k, v, mono }: { k: string; v: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-[11px] uppercase tracking-wide text-faint">{k}</dt>
      <dd className={`truncate text-fg2 ${mono ? 'mono' : ''}`}>{v}</dd>
    </div>
  )
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-28 rounded-xl border border-edge bg-inset px-4 py-2.5">
      <div className="text-[11px] uppercase tracking-wide text-faint">{label}</div>
      <div className="mono text-sm text-info">{value}</div>
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
  'rounded-lg px-3 py-1.5 text-sm text-muted transition hover:text-fg data-selected:bg-hover/10 data-selected:text-fg'