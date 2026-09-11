import { Badge } from '@/components/ui/badge'

type Tone = 'ok' | 'warn' | 'err' | 'idle' | 'info'

const tones: Record<Tone, 'success' | 'warning' | 'destructive' | 'muted' | 'info'> = {
  ok: 'success',
  warn: 'warning',
  err: 'destructive',
  idle: 'muted',
  info: 'info',
}

export default function StatusBadge({ tone, label }: { tone: Tone; label: string }) {
  return (
    <Badge variant={tones[tone]}>
      <span className="relative flex h-1.5 w-1.5">
        {tone === 'ok' && (
          <span className="absolute inline-flex h-full w-full animate-ping-soft rounded-full bg-current opacity-60" />
        )}
        <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-current" />
      </span>
      {label}
    </Badge>
  )
}
