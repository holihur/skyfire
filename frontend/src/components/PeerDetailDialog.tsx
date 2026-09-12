import { useState } from 'react'
import { Copy, Download } from 'lucide-react'
import Dialog from './Dialog'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useToast } from '@/components/ui/use-toast'
import { api, openDownload } from '../lib/api'
import { copyText } from '../lib/clipboard'
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
  const { toast } = useToast()
  const { t, lang } = useI18n()
  const [tab, setTab] = useState('qr')
  const [cmdOS, setCmdOS] = useState<'unix' | 'windows'>('unix')
  const [config, setConfig] = useState('')
  const [configError, setConfigError] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  const loadConfig = async (): Promise<string> => {
    try {
      const cfg = await api.getText(api.peerConfigUrl(ifaceName, peer.publicKey))
      setConfig(cfg)
      setConfigError(false)
      return cfg
    } catch (err) {
      setConfigError(true)
      toast({ description: String(err), variant: 'destructive' })
      return ''
    }
  }

  const copyConfig = async () => {
    const text = config || await loadConfig()
    if (!text) return
    if (await copyText(text)) {
      toast({ description: t('peerDetail.confCopied'), variant: 'success' })
    } else {
      toast({ description: t('common.clipboardUnavailable'), variant: 'destructive' })
    }
  }

  const connectString = `${window.location.origin}${api.tokenConfigUrl(peer.clientToken)}`

  // One-click connect command an admin can copy and hand to an end user. The
  // client is launched with -connect, which now brings the tunnel up
  // immediately (tray starts connected).
  const connectCommand =
    cmdOS === 'windows'
      ? `skyfire-client.exe -connect "${connectString}"`
      : `skyfire-client -connect '${connectString}'`

  const copyConnectString = async () => {
    if (await copyText(connectString)) {
      toast({ description: t('peerDetail.connectCopied'), variant: 'success' })
    } else {
      toast({ description: t('common.clipboardUnavailable'), variant: 'destructive' })
    }
  }

  const copyCommand = async () => {
    if (await copyText(connectCommand)) {
      toast({ description: t('peerDetail.commandCopied'), variant: 'success' })
    } else {
      toast({ description: t('common.clipboardUnavailable'), variant: 'destructive' })
    }
  }

  const doDelete = async () => {
    try {
      await api.deletePeer(ifaceName, peer.publicKey)
      onDeleted?.(peer.publicKey)
      onOpenChange(false)
      toast({ description: t('peerDetail.removed'), variant: 'success' })
    } catch (err) {
      toast({ description: String(err), variant: 'destructive' })
    }
  }

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
            <Button variant="outline" onClick={() => onEdited?.()}>
              {t('peerDetail.edit')}
            </Button>
            <Button variant="destructive" onClick={() => setConfirmDelete(true)}>
              {t('iface.delete')}
            </Button>
          </div>
          <Button variant="outline" onClick={copyConfig}>
            <Copy /> {t('common.copy')}
          </Button>
          <Button variant="outline" onClick={() => openDownload(api.peerConfigUrl(ifaceName, peer.publicKey))}>
            <Download /> {t('common.download')}
          </Button>
        </div>
      }
    >
      {confirmDelete && (
        <div className="mb-4 rounded-xl border border-destructive/50 bg-destructive/10 p-4">
          <p className="text-sm text-destructive">
            {t('peerDetail.removeBody', { name: peer.name })}
          </p>
          <div className="mt-3 flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={() => setConfirmDelete(false)}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" size="sm" onClick={doDelete}>
              {t('peerDetail.removeBtn')}
            </Button>
          </div>
        </div>
      )}

      <Tabs value={tab} onValueChange={setTab}>
        <TabsList className="mb-4">
          <TabsTrigger value="qr">{t('peerDetail.qr')}</TabsTrigger>
          <TabsTrigger value="conf">{t('peerDetail.conf')}</TabsTrigger>
          <TabsTrigger value="client">{t('peerDetail.client')}</TabsTrigger>
          <TabsTrigger value="info">{t('peerDetail.info')}</TabsTrigger>
        </TabsList>

        <TabsContent value="qr" className="flex flex-col items-center gap-4">
          <div className="rounded-2xl border bg-white p-4">
            <img
              src={api.peerQrUrl(ifaceName, peer.publicKey)}
              alt={`${peer.name} QR`}
              className="h-64 w-64"
              onLoad={loadConfig}
            />
          </div>
          <p className="text-center text-xs text-muted-foreground">{t('peerDetail.qrHint')}</p>
        </TabsContent>

        <TabsContent value="conf">
          {configError ? (
            <div className="flex flex-col items-center gap-3 py-8 text-center">
              <p className="text-sm text-muted-foreground">{t('peerDetail.loadFailed')}</p>
              <Button variant="outline" size="sm" onClick={() => void loadConfig()}>
                {t('peerDetail.retry')}
              </Button>
            </div>
          ) : (
            <pre className="mono max-h-80 overflow-auto rounded-xl border bg-secondary p-4">
              {config || t('peerDetail.loading')}
            </pre>
          )}
        </TabsContent>

        <TabsContent value="client">
          <div className="flex flex-col gap-5">
            <p className="text-sm text-muted-foreground">{t('peerDetail.clientHint')}</p>

            <div>
              <div className="mb-1 text-xs uppercase tracking-wide text-muted-foreground">
                {t('peerDetail.connectString')}
              </div>
              <div className="flex items-center gap-2">
                <code className="mono flex-1 break-all rounded-xl border bg-secondary px-3 py-2 text-xs">
                  {connectString}
                </code>
                <Button variant="outline" size="icon" onClick={copyConnectString} title={t('common.copy')}>
                  <Copy />
                </Button>
              </div>
            </div>

            <div>
              <div className="mb-1 flex items-center justify-between gap-2">
                <span className="text-xs uppercase tracking-wide text-muted-foreground">
                  {t('peerDetail.command')}
                </span>
                <div className="flex gap-1">
                  <Button
                    size="sm"
                    variant={cmdOS === 'unix' ? 'secondary' : 'ghost'}
                    onClick={() => setCmdOS('unix')}
                  >
                    {t('peerDetail.commandUnix')}
                  </Button>
                  <Button
                    size="sm"
                    variant={cmdOS === 'windows' ? 'secondary' : 'ghost'}
                    onClick={() => setCmdOS('windows')}
                  >
                    {t('peerDetail.commandWindows')}
                  </Button>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <code className="mono flex-1 break-all rounded-xl border bg-secondary px-3 py-2 text-xs">
                  {connectCommand}
                </code>
                <Button variant="outline" size="icon" onClick={copyCommand} title={t('common.copy')}>
                  <Copy />
                </Button>
              </div>
              <p className="mt-2 text-xs text-muted-foreground">{t('peerDetail.commandHint')}</p>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="info">
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
        </TabsContent>
      </Tabs>
    </Dialog>
  )
}

function Info({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <>
      <dt className="text-xs uppercase tracking-wide text-muted-foreground">{label}</dt>
      <dd className={`break-all text-secondary-foreground ${mono ? 'mono' : ''}`}>{value || '—'}</dd>
    </>
  )
}
