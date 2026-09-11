import { clsx } from 'clsx'

type Tone = 'ok' | 'warn' | 'err' | 'idle' | 'info'

export default function StatusBadge({ tone, label }: { tone: Tone; label: string }) {
  const styles: Record<Tone, string> = {
    ok: 'bg-ok/10 text-ok border-ok/30',
    warn: 'bg-warn/10 text-warn border-warn/30',
    err: 'bg-err/10 text-err border-err/30',
    idle: 'bg-muted/10 text-muted border-muted/30',
    info: 'bg-info/10 text-info border-info/30',
  }
  return (
    <span
      className={clsx(
        'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium',
        styles[tone],
      )}
    >
      <span className="relative flex h-1.5 w-1.5">
        {tone === 'ok' && (
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-ok opacity-60" />
        )}
        <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-current" />
      </span>
      {label}
    </span>
  )
}