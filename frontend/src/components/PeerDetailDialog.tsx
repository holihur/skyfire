import { useState } from 'react'
import * as Tabs from '@radix-ui/react-tabs'
import { CopyIcon, DownloadIcon } from '@radix-ui/react-icons'
import Dialog from './Dialog'
import { api, openDownload } from '../lib/api'
import { useToast } from './Toast'
import { useI18n } from '../i18n'
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
  const { t, lang } = useI18n()
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
      push(t('peerDetail.confCopied'), 'success')
    } catch {
      push(t('common.clipboardUnavailable'), 'error')
    }
  }

  const doDelete = async () => {
    try {
      await api.deletePeer(ifaceName, peer.publicKey)
      onDeleted?.(peer.publicKey)
      onOpenChange(false)
      push(t('peerDetail.removed'), 'success')
    } catch (err) {
      push(String(err), 'error')
    }
  }

  const tabTrigger =
    'rounded-lg px-3 py-1.5 text-sm text-muted transition hover:text-fg data-selected:bg-hover/10 data-selected:text-fg'

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={peer.name || t('iface.editPeer')}
      description={shortKey(peer.publicKey, 24)}
      width="max-w-xl"
      footer={
        <div className="flex items-center gap-3">
          <div className="mr-auto flex gap-2">
            <button className="btn-ghost" onClick={() => onEdited?.()}>
              {t('peerDetail.edit')}
            </button>
            <button className="btn-danger" onClick={() => setConfirmDelete(true)}>
              {t('iface.delete')}
            </button>
          </div>
          <button className="btn-ghost" onClick={copyConfig}>
            <CopyIcon className="h-4 w-4" /> {t('common.copy')}
          </button>
          <button
            className="btn-ghost"
            onClick={() => openDownload(api.peerConfigUrl(ifaceName, peer.publicKey))}
          >
            <DownloadIcon className="h-4 w-4" /> {t('common.download')}
          </button>
        </div>
      }
    >
      {confirmDelete && (
        <div className="mb-4 rounded-xl border border-err/60 bg-err/10 p-4">
          <p className="text-sm text-err">
            {t('peerDetail.removeBody', { name: peer.name })}
          </p>
          <div className="mt-3 flex justify-end gap-2">
            <button className="btn-ghost" onClick={() => setConfirmDelete(false)}>
              {t('common.cancel')}
            </button>
            <button className="btn-danger" onClick={doDelete}>
              {t('peerDetail.removeBtn')}
            </button>
          </div>
        </div>
      )}

      <Tabs.Root value={tab} onValueChange={setTab}>
        <Tabs.List className="mb-4 flex gap-1 rounded-xl border border-edge2 bg-inset p-1">
          <Tabs.Trigger value="qr" className={tabTrigger}>
            {t('peerDetail.qr')}
          </Tabs.Trigger>
          <Tabs.Trigger value="conf" className={tabTrigger}>
            {t('peerDetail.conf')}
          </Tabs.Trigger>
          <Tabs.Trigger value="info" className={tabTrigger}>
            {t('peerDetail.info')}
          </Tabs.Trigger>
        </Tabs.List>

        <Tabs.Content value="qr" className="flex flex-col items-center gap-4">
          <div className="rounded-2xl border border-edge2 bg-white p-4">
            <img
              src={api.peerQrUrl(ifaceName, peer.publicKey)}
              alt={`${peer.name} QR`}
              className="h-64 w-64"
              onLoad={loadConfig}
            />
          </div>
          <p className="text-center text-xs text-muted">{t('peerDetail.qrHint')}</p>
        </Tabs.Content>

        <Tabs.Content value="conf">
          <pre className="mono max-h-80 overflow-auto rounded-xl border border-edge bg-page p-4 text-fg2">
            {config || t('peerDetail.loading')}
          </pre>
        </Tabs.Content>

        <Tabs.Content value="info">
          <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
            <Info label={t('peerDetail.info.name')} value={peer.name} />
            <Info label={t('peerDetail.info.address')} value={peer.address} />
            <Info label={t('peerDetail.info.pubkey')} value={shortKey(peer.publicKey, 32)} mono />
            {peer.presharedKey && (
              <Info label={t('peerDetail.info.psk')} value={t('peerDetail.info.pskPresent')} />
            )}
            <Info label={t('peerDetail.info.endpoint')} value={peer.endpoint || '—'} mono />
            <Info
              label={t('peerDetail.info.status')}
              value={peer.connected ? t('peerDetail.info.connected') : t('peerDetail.info.idle')}
            />
            <Info label={t('peerDetail.info.handshake')} value={fmtAge(peer.latestHandshake, lang)} />
            <Info label={t('peerDetail.info.transfer')} value={`↑ ${fmtBytes(peer.transferTx)} · ↓ ${fmtBytes(peer.transferRx)}`} />
            <Info label={t('peerDetail.info.clientRoutes')} value={(peer.clientRoutes ?? []).join(', ')} mono />
            <Info
              label={t('peerDetail.info.keepalive')}
              value={peer.persistentKeepalive ? t('peerDetail.info.keepaliveS', { value: peer.persistentKeepalive }) : t('peerDetail.info.keepaliveOff')}
            />
          </dl>
        </Tabs.Content>
      </Tabs.Root>
    </Dialog>
  )
}

function Info({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <>
      <dt className="text-xs uppercase tracking-wide text-faint">{label}</dt>
      <dd className={`break-all text-fg2 ${mono ? 'mono' : ''}`}>{value || '—'}</dd>
    </>
  )
}