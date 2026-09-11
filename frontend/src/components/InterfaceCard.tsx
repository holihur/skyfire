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
  const { push } = useToast()
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

  const totalRx = useMemo(() => iface.peers.reduce((a, p) => a + p.transferRx, 0), [iface.peers])
  const totalTx = useMemo(() => iface.peers.reduce((a, p) => a + p.transferTx, 0), [iface.peers])

  const toggle = async () => {
    setApplying(true)
    try {
      const updated = await api.setInterfaceUp(iface.name, !iface.up)
      onEdited(updated)
      push(updated.up ? t('iface.enabledToast', { name: iface.name }) : t('iface.disabledToast', { name: iface.name }), 'success')
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
      push(t('iface.deleted', { name: iface.name }), 'success')
    } catch (err) {
      push(String(err), 'error')
    }
  }

  const copyPubkey = async () => {
    try {
      await navigator.clipboard.writeText(iface.publicKey)
      push(t('iface.copyPubkey'), 'success')
    } catch {
      push(t('common.clipboardUnavailable'), 'error')
    }
  }

  const copyWgQuick = async () => {
    try {
      const cfg = await api.getText(api.serverConfigUrl(iface.name))
      await navigator.clipboard.writeText(cfg)
      push(t('iface.copySrvConf'), 'success')
    } catch (err) {
      push(String(err), 'error')
    }
  }

  const itemCls =
    'flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-fg2 hover:bg-hover/10 hover:text-fg'

  return (
    <div className="card group relative overflow-hidden p-5 transition hover:border-edge2 hover:shadow-lg hover:shadow-shade/20">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <button
              onClick={() => navigate(`/interfaces/${iface.name}`)}
              className="truncate text-lg font-semibold text-fg transition group-hover:text-brand"
            >
              {iface.name}
            </button>
            <StatusBadge tone={tone} label={status} />
          </div>
          <div className="mt-1 text-xs text-muted">
            {iface.up ? `${iface.listenPort || t('iface.randomPort')} · ${iface.addresses.join(', ')}` : '—'}
          </div>
        </div>

        <DropdownMenu.Root>
          <DropdownMenu.Trigger asChild>
            <button className="btn-ghost !p-2" aria-label={t('iface.actions')}>
              <DotsHorizontalIcon className="h-4 w-4" />
            </button>
          </DropdownMenu.Trigger>
          <DropdownMenu.Portal>
            <DropdownMenu.Content
              align="end"
              sideOffset={6}
              className="z-50 min-w-44 rounded-xl border border-edge2 bg-panel2 p-1 shadow-2xl shadow-shade/50"
            >
              <DropdownMenu.Item
                className={itemCls}
                onSelect={() => navigate(`/interfaces/${iface.name}`)}
              >
                <EyeOpenIcon className="h-4 w-4" /> {t('iface.details')}
              </DropdownMenu.Item>
              <DropdownMenu.Item
                className={itemCls}
                onSelect={toggle}
                disabled={applying}
              >
                {iface.up ? <StopIcon className="h-4 w-4" /> : <PlayIcon className="h-4 w-4" />}
                {iface.up ? t('iface.disable') : t('iface.enable')}
              </DropdownMenu.Item>
              <DropdownMenu.Item
                className={itemCls}
                onSelect={copyPubkey}
              >
                <CopyIcon className="h-4 w-4" /> {t('iface.copyPublicKey')}
              </DropdownMenu.Item>
              <DropdownMenu.Item
                className={itemCls}
                onSelect={copyWgQuick}
              >
                <CopyIcon className="h-4 w-4" /> {t('iface.copyServerConf')}
              </DropdownMenu.Item>
              <DropdownMenu.Item
                className={itemCls}
                onSelect={() => openDownload(api.serverConfigUrl(iface.name))}
              >
                <DownloadIcon className="h-4 w-4" /> {t('iface.downloadConf')}
              </DropdownMenu.Item>
              <DropdownMenu.Separator className="my-1 h-px bg-edge2" />
              <DropdownMenu.Item
                className="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-err hover:bg-err/10 hover:text-err"
                onSelect={() => setConfirmDelete(true)}
              >
                <TrashIcon className="h-4 w-4" /> {t('iface.delete')}
              </DropdownMenu.Item>
            </DropdownMenu.Content>
          </DropdownMenu.Portal>
        </DropdownMenu.Root>
      </div>

      <div className="mt-4 grid grid-cols-3 gap-3 text-center">
        <Stat label={t('iface.stat.peers')} value={`${iface.connectedPeers}/${iface.totalPeers}`} sub={t('iface.stat.connTotal')} />
        <Stat label={t('iface.stat.download')} value={fmtBytes(totalRx)} sub={t('iface.stat.received')} />
        <Stat label={t('iface.stat.upload')} value={fmtBytes(totalTx)} sub={t('iface.stat.sent')} />
      </div>

      <div className="mt-4 flex items-center justify-between border-t border-edge pt-3">
        <span className="mono text-[11px] text-faint">{shortKey(iface.publicKey, 22)}</span>
        <button className="btn-ghost" onClick={() => navigate(`/interfaces/${iface.name}`)}>
          {t('iface.configure')}
        </button>
      </div>

      {confirmDelete && (
        <div className="absolute inset-0 z-10 flex items-center justify-center rounded-2xl bg-black/70 backdrop-blur-sm">
          <div className="w-64 space-y-3 rounded-xl border border-err/60 bg-err/10 p-4">
            <p className="text-sm text-err">
              {t('iface.delete.confirm', { name: iface.name })}
            </p>
            <div className="flex justify-end gap-2">
              <button className="btn-ghost" onClick={() => setConfirmDelete(false)}>
                {t('common.cancel')}
              </button>
              <button className="btn-danger" onClick={del}>
                {t('iface.delete')}
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
    <div className="rounded-xl border border-edge bg-inset px-2 py-2.5">
      <div className="text-[11px] uppercase tracking-wide text-faint">{label}</div>
      <div className="mt-0.5 text-base font-semibold text-fg">{value}</div>
      <div className="text-[11px] text-faint">{sub}</div>
    </div>
  )
}