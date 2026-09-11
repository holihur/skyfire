import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Copy,
  Download,
  Eye,
  MoreHorizontal,
  Play,
  Square,
  Trash2,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useToast } from '@/components/ui/use-toast'
import { api, openDownload } from '../lib/api'
import { useI18n } from '../i18n'
import { fmtBytes, shortKey } from '../lib/format'
import StatusBadge from './StatusBadge'
import type { WireGuardInterface } from '../lib/types'

interface Props {
  iface: WireGuardInterface
  onDeleted: (name: string) => void
  onEdited: (iface: WireGuardInterface) => void
}

export default function InterfaceCard({ iface, onDeleted, onEdited }: Props) {
  const { toast } = useToast()
  const { t } = useI18n()
  const navigate = useNavigate()
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [applying, setApplying] = useState(false)

  const tone = !iface.up ? 'idle' : iface.running ? 'ok' : 'err'
  const status =
    !iface.up
      ? t('iface.disabled')
      : iface.running
        ? t('iface.running')
        : iface.dryRun
          ? t('iface.wouldRun')
          : t('iface.notRunning')

  const peers = iface.peers ?? []
  const totalRx = useMemo(() => peers.reduce((a, p) => a + p.transferRx, 0), [peers])
  const totalTx = useMemo(() => peers.reduce((a, p) => a + p.transferTx, 0), [peers])

  const toggle = async () => {
    setApplying(true)
    try {
      const updated = await api.setInterfaceUp(iface.name, !iface.up)
      onEdited(updated)
      toast({
        description: updated.up
          ? t('iface.enabledToast', { name: iface.name })
          : t('iface.disabledToast', { name: iface.name }),
        variant: 'success',
      })
    } catch (err) {
      toast({ description: String(err), variant: 'destructive' })
    } finally {
      setApplying(false)
    }
  }

  const del = async () => {
    try {
      await api.deleteInterface(iface.name)
      onDeleted(iface.name)
      toast({ description: t('iface.deleted', { name: iface.name }), variant: 'success' })
    } catch (err) {
      toast({ description: String(err), variant: 'destructive' })
    }
  }

  const copyPubkey = async () => {
    try {
      await navigator.clipboard.writeText(iface.publicKey)
      toast({ description: t('iface.copyPubkey'), variant: 'success' })
    } catch {
      toast({ description: t('common.clipboardUnavailable'), variant: 'destructive' })
    }
  }

  const copyWgQuick = async () => {
    try {
      const cfg = await api.getText(api.serverConfigUrl(iface.name))
      await navigator.clipboard.writeText(cfg)
      toast({ description: t('iface.copySrvConf'), variant: 'success' })
    } catch (err) {
      toast({ description: String(err), variant: 'destructive' })
    }
  }

  return (
    <Card className="group relative overflow-hidden p-5 transition hover:border-ring/50 hover:shadow-lg hover:shadow-black/20">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <button
              onClick={() => navigate(`/interfaces/${iface.name}`)}
              className="truncate text-lg font-semibold text-foreground transition group-hover:text-primary"
            >
              {iface.name}
            </button>
            <StatusBadge tone={tone} label={status} />
          </div>
          <div className="mt-1 text-xs text-muted-foreground">
            {iface.up ? `${iface.listenPort || t('iface.randomPort')} · ${iface.addresses.join(', ')}` : '—'}
          </div>
        </div>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="icon" aria-label={t('iface.actions')}>
              <MoreHorizontal />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" sideOffset={6} className="min-w-44">
            <DropdownMenuItem onSelect={() => navigate(`/interfaces/${iface.name}`)}>
              <Eye /> {t('iface.details')}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={toggle} disabled={applying}>
              {iface.up ? <Square /> : <Play />}
              {iface.up ? t('iface.disable') : t('iface.enable')}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={copyPubkey}>
              <Copy /> {t('iface.copyPublicKey')}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={copyWgQuick}>
              <Copy /> {t('iface.copyServerConf')}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => openDownload(api.serverConfigUrl(iface.name))}>
              <Download /> {t('iface.downloadConf')}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={() => setConfirmDelete(true)}>
              <Trash2 /> {t('iface.delete')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <div className="mt-4 grid grid-cols-3 gap-3 text-center">
        <Stat label={t('iface.stat.peers')} value={`${iface.connectedPeers}/${iface.totalPeers}`} sub={t('iface.stat.connTotal')} />
        <Stat label={t('iface.stat.download')} value={fmtBytes(totalRx)} sub={t('iface.stat.received')} />
        <Stat label={t('iface.stat.upload')} value={fmtBytes(totalTx)} sub={t('iface.stat.sent')} />
      </div>

      <div className="mt-4 flex items-center justify-between border-t pt-3">
        <span className="mono text-[11px] text-muted-foreground/70">{shortKey(iface.publicKey, 22)}</span>
        <Button variant="outline" size="sm" onClick={() => navigate(`/interfaces/${iface.name}`)}>
          {t('iface.configure')}
        </Button>
      </div>

      {confirmDelete && (
        <div className="absolute inset-0 z-10 flex items-center justify-center rounded-2xl bg-black/70 backdrop-blur-sm">
          <div className="w-64 space-y-3 rounded-xl border border-destructive/60 bg-destructive/10 p-4">
            <p className="text-sm text-destructive">
              {t('iface.delete.confirm', { name: iface.name })}
            </p>
            <div className="flex justify-end gap-2">
              <Button variant="outline" size="sm" onClick={() => setConfirmDelete(false)}>
                {t('common.cancel')}
              </Button>
              <Button variant="destructive" size="sm" onClick={del}>
                {t('iface.delete')}
              </Button>
            </div>
          </div>
        </div>
      )}
    </Card>
  )
}

function Stat({ label, value, sub }: { label: string; value: string; sub: string }) {
  return (
    <div className="rounded-xl border bg-secondary px-2 py-2.5">
      <div className="text-[11px] uppercase tracking-wide text-muted-foreground/70">{label}</div>
      <div className="mt-0.5 text-base font-semibold text-foreground">{value}</div>
      <div className="text-[11px] text-muted-foreground/70">{sub}</div>
    </div>
  )
}
