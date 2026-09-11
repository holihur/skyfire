import { useEffect, useMemo, useState } from 'react'
import { Copy } from 'lucide-react'
import Dialog from './Dialog'
import { Button } from '@/components/ui/button'
import { useToast } from '@/components/ui/use-toast'
import { api } from '../lib/api'
import { useI18n } from '../i18n'
import type { Peer, WireGuardInterface } from '../lib/types'

interface Option {
  key: string
  iface: string
  peer: Peer
}

// QuickConnectDialog surfaces the one-click connect command for any peer in
// one place, so an admin does not have to open each peer to find it.
export default function QuickConnectDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
}) {
  const { toast } = useToast()
  const { t } = useI18n()
  const [ifaces, setIfaces] = useState<WireGuardInterface[]>([])
  const [loading, setLoading] = useState(false)
  const [selected, setSelected] = useState('')
  const [os, setOs] = useState<'unix' | 'windows'>('unix')

  useEffect(() => {
    if (!open) return
    setLoading(true)
    api
      .interfaces()
      .then(setIfaces)
      .catch((err) => toast({ description: String(err), variant: 'destructive' }))
      .finally(() => setLoading(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  const options = useMemo<Option[]>(() => {
    const out: Option[] = []
    for (const i of ifaces) {
      for (const p of i.peers ?? []) {
        out.push({ key: `${i.name}/${p.publicKey}`, iface: i.name, peer: p })
      }
    }
    return out
  }, [ifaces])

  useEffect(() => {
    if (options.length && !options.some((o) => o.key === selected)) {
      setSelected(options[0].key)
    }
  }, [options, selected])

  const current = options.find((o) => o.key === selected)
  const connectString = current
    ? `${window.location.origin}${api.tokenConfigUrl(current.peer.clientToken)}`
    : ''
  const command =
    os === 'windows'
      ? `skyfire-client.exe -connect "${connectString}"`
      : `skyfire-client -connect '${connectString}'`

  const copyCommand = async () => {
    try {
      await navigator.clipboard.writeText(command)
      toast({ description: t('peerDetail.commandCopied'), variant: 'success' })
    } catch {
      toast({ description: t('common.clipboardUnavailable'), variant: 'destructive' })
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('quick.title')}
      description={t('quick.desc')}
      width="max-w-xl"
    >
      {loading ? (
        <p className="py-6 text-center text-sm text-muted-foreground">{t('iface.loading')}</p>
      ) : options.length === 0 ? (
        <p className="py-6 text-center text-sm text-muted-foreground">{t('quick.none')}</p>
      ) : (
        <div className="flex flex-col gap-5">
          <div>
            <div className="mb-1 text-xs uppercase tracking-wide text-muted-foreground">
              {t('quick.peer')}
            </div>
            <select
              className="w-full cursor-pointer rounded-lg border bg-popover px-3 py-2 text-sm text-secondary-foreground outline-none transition hover:text-foreground"
              value={selected}
              onChange={(e) => setSelected(e.target.value)}
            >
              {options.map((o) => (
                <option key={o.key} value={o.key}>
                  {o.iface} / {o.peer.name} · {o.peer.address}
                </option>
              ))}
            </select>
          </div>

          <div>
            <div className="mb-1 flex items-center justify-between gap-2">
              <span className="text-xs uppercase tracking-wide text-muted-foreground">
                {t('peerDetail.command')}
              </span>
              <div className="flex gap-1">
                <Button size="sm" variant={os === 'unix' ? 'secondary' : 'ghost'} onClick={() => setOs('unix')}>
                  {t('peerDetail.commandUnix')}
                </Button>
                <Button
                  size="sm"
                  variant={os === 'windows' ? 'secondary' : 'ghost'}
                  onClick={() => setOs('windows')}
                >
                  {t('peerDetail.commandWindows')}
                </Button>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <code className="mono flex-1 break-all rounded-xl border bg-secondary px-3 py-2 text-xs">
                {command}
              </code>
              <Button variant="outline" size="icon" onClick={copyCommand} title={t('common.copy')}>
                <Copy />
              </Button>
            </div>
            <p className="mt-2 text-xs text-muted-foreground">{t('peerDetail.commandHint')}</p>
          </div>
        </div>
      )}
    </Dialog>
  )
}
