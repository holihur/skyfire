import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  ArrowLeft,
  Copy,
  Download,
  Eye,
  EyeOff,
  MoreHorizontal,
  Pencil,
  Plus,
  Smartphone,
  Trash2,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useToast } from '@/components/ui/use-toast'
import { api, openDownload } from '../lib/api'
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
  const { toast } = useToast()
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
      toast({ description: String(err), variant: 'destructive' })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [name])

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
    () => (iface ? (iface.peers ?? []).reduce((a, p) => a + p.transferRx, 0) : 0),
    [iface],
  )
  const totalTx = useMemo(
    () => (iface ? (iface.peers ?? []).reduce((a, p) => a + p.transferTx, 0) : 0),
    [iface],
  )

  if (!iface) {
    return (
      <div className="mx-auto max-w-6xl py-24 text-center text-muted-foreground">
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
      toast({
        description: updated.up ? t('iface.update.on') : t('iface.update.off'),
        variant: 'success',
      })
    } catch (err) {
      toast({ description: String(err), variant: 'destructive' })
    }
  }

  const revealPri = async () => {
    if (!showPri) {
      try {
        setPriKey((await api.privateKey(iface.name)).privateKey)
      } catch (err) {
        toast({ description: String(err), variant: 'destructive' })
        return
      }
    }
    setShowPri((v) => !v)
  }

  const copy = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text)
      toast({ description: t('common.copied', { label }), variant: 'success' })
    } catch {
      toast({ description: t('common.clipboardUnavailable'), variant: 'destructive' })
    }
  }

  const deletePeer = async (p: Peer) => {
    try {
      await api.deletePeer(iface.name, p.publicKey)
      setDeletePeerTarget(null)
      await load()
      toast({
        description: t('iface.peer.removeToast', { name: p.name }),
        variant: 'success',
      })
    } catch (err) {
      toast({ description: String(err), variant: 'destructive' })
    }
  }

  return (
    <div className="mx-auto max-w-6xl">
      <Link
        to="/"
        className="mb-4 inline-flex items-center gap-1.5 text-sm text-muted-foreground transition hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" /> {t('iface.back')}
      </Link>

      <Card className="p-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div className="flex flex-wrap items-center gap-3">
              <h1 className="text-2xl font-semibold text-foreground">{iface.name}</h1>
              <StatusBadge tone={tone} label={status} />
              <label className="flex cursor-pointer items-center gap-2">
                <Switch checked={iface.up} onCheckedChange={toggleUp} />
                <span className="text-xs text-muted-foreground">
                  {iface.up ? t('iface.enabled') : t('iface.disabled')}
                </span>
              </label>
            </div>
            <dl className="mt-3 grid grid-cols-2 gap-x-8 gap-y-2 text-sm sm:grid-cols-3 lg:grid-cols-4">
              <KV k={t('iface.listenPort')} v={iface.listenPort ? String(iface.listenPort) : t('iface.randomPort')} />
              <KV k={t('iface.addresses')} v={(iface.addresses ?? []).join(', ')} mono />
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
              <Button variant="outline" size="sm" onClick={() => setEditIface(true)}>
                <Pencil /> {t('iface.edit')}
              </Button>
              <Button variant="outline" size="sm" onClick={() => openDownload(api.serverConfigUrl(iface.name))}>
                <Download /> {t('iface.srvConf')}
              </Button>
            </div>
          </div>
        </div>

        <div className="mt-5 space-y-2 border-t pt-4">
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
              <Button
                variant="ghost"
                size="icon"
                title={t('iface.revealPriFirst')}
                onClick={revealPri}
              >
                {showPri ? <Eye /> : <EyeOff />}
              </Button>
            }
            onCopy={
              showPri
                ? () => copy(priKey, t('iface.prikey'))
                : () => toast({ description: t('iface.revealPriFirst') })
            }
          />
        </div>
      </Card>

      <Tabs defaultValue="peers" className="mt-6">
        <div className="mb-4 flex items-center justify-between gap-3">
          <TabsList>
            <TabsTrigger value="peers">
              {t('iface.stat.peers')} ({(iface.peers ?? []).length})
            </TabsTrigger>
            <TabsTrigger value="conf">{t('iface.srvConf')}</TabsTrigger>
          </TabsList>
          <Button onClick={() => setPeerForm({ open: true, peer: null })}>
            <Plus /> {t('iface.addPeer')}
          </Button>
        </div>

        <TabsContent value="peers">
          {(iface.peers ?? []).length === 0 ? (
            <Card className="flex flex-col items-center gap-3 py-16 text-center">
              <span className="text-3xl">👥</span>
              <p className="text-muted-foreground">{t('iface.peer.noPeers')}</p>
              <Button onClick={() => setPeerForm({ open: true, peer: null })}>
                <Plus /> {t('iface.addfirstPeer')}
              </Button>
            </Card>
          ) : (
            <Card className="overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('iface.peerTable.name')}</TableHead>
                    <TableHead>{t('iface.peerTable.address')}</TableHead>
                    <TableHead className="hidden lg:table-cell">{t('iface.peerTable.pubkey')}</TableHead>
                    <TableHead className="hidden md:table-cell">{t('iface.peerTable.status')}</TableHead>
                    <TableHead className="hidden md:table-cell">{t('iface.peerTable.handshake')}</TableHead>
                    <TableHead className="hidden sm:table-cell">{t('iface.peerTable.transfer')}</TableHead>
                    <TableHead className="text-right">{t('iface.actions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(iface.peers ?? []).map((p) => (
                    <TableRow key={p.publicKey}>
                      <TableCell>
                        <button
                          className="font-medium text-foreground hover:text-primary"
                          onClick={() => setPeerDetail(p)}
                        >
                          {p.name}
                        </button>
                        <div className="text-xs text-muted-foreground/70">
                          {p.presharedKey ? 'psk·on' : ''}
                          {p.endpoint ? ` · ${p.endpoint}` : ''}
                        </div>
                      </TableCell>
                      <TableCell className="mono text-secondary-foreground">{ipOf(p.address)}</TableCell>
                      <TableCell className="hidden lg:table-cell">
                        <button
                          className="mono text-muted-foreground hover:text-primary"
                          onClick={() => copy(p.publicKey, t('iface.pubkey'))}
                          title={t('iface.peer.copyPubkey')}
                        >
                          {shortKey(p.publicKey, 20)}
                        </button>
                      </TableCell>
                      <TableCell className="hidden md:table-cell">
                        {p.connected ? (
                          <StatusBadge tone="ok" label={t('iface.peer.connected')} />
                        ) : (
                          <StatusBadge tone="idle" label={t('iface.peer.idle')} />
                        )}
                      </TableCell>
                      <TableCell className="hidden text-muted-foreground md:table-cell">
                        {fmtAge(p.latestHandshake, lang)}
                      </TableCell>
                      <TableCell className="mono hidden text-secondary-foreground sm:table-cell">
                        ↑ {fmtBytes(p.transferTx)} ↓ {fmtBytes(p.transferRx)}
                      </TableCell>
                      <TableCell>
                        <div className="flex justify-end gap-1">
                          <Button variant="ghost" size="icon" title={t('iface.peer.showQR')} onClick={() => setPeerDetail(p)}>
                            <Smartphone />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            title={t('iface.peer.downloadClient')}
                            onClick={() => openDownload(api.peerConfigUrl(iface.name, p.publicKey))}
                          >
                            <Download />
                          </Button>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button variant="ghost" size="icon" title={t('iface.peer.more')}>
                                <MoreHorizontal />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end" sideOffset={6} className="min-w-40">
                              <DropdownMenuItem onSelect={() => setPeerForm({ open: true, peer: p })}>
                                <Pencil /> {t('iface.editPeer')}
                              </DropdownMenuItem>
                              <DropdownMenuItem variant="destructive" onSelect={() => setDeletePeerTarget(p)}>
                                <Trash2 /> {t('iface.delete')}
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Card>
          )}
        </TabsContent>

        <TabsContent value="conf">
          <Card className="p-5">
            <div className="mb-3 flex items-center justify-between">
              <span className="text-sm text-muted-foreground">{t('iface.conf.wgQuick')}</span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void copy(serverConf, t('iface.srvConf'))}
              >
                <Copy /> {t('iface.conf.copy')}
              </Button>
            </div>
            <pre className="mono max-h-96 overflow-auto rounded-xl border bg-secondary p-4">
              {serverConf}
            </pre>
          </Card>
        </TabsContent>
      </Tabs>

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
      <span className="w-24 shrink-0 text-xs uppercase tracking-wide text-muted-foreground/70">{label}</span>
      <code className={`mono flex-1 truncate rounded-lg border bg-secondary px-3 py-2 text-secondary-foreground ${masked ? 'tracking-widest' : ''}`}>
        {value}
      </code>
      {action}
      <Button variant="ghost" size="icon" title={label} onClick={onCopy}>
        <Copy />
      </Button>
    </div>
  )
}

function KV({ k, v, mono }: { k: string; v: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-[11px] uppercase tracking-wide text-muted-foreground/70">{k}</dt>
      <dd className={`truncate text-secondary-foreground ${mono ? 'mono' : ''}`}>{v}</dd>
    </div>
  )
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-28 rounded-xl border bg-secondary px-4 py-2.5">
      <div className="text-[11px] uppercase tracking-wide text-muted-foreground/70">{label}</div>
      <div className="mono text-sm text-info">{value}</div>
    </div>
  )
}
