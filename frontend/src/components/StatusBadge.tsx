import { clsx } from 'clsx'

type Tone = 'ok' | 'warn' | 'err' | 'idle' | 'info'

export default function StatusBadge({ tone, label }: { tone: Tone; label: string }) {
  const styles: Record<Tone, string> = {
    ok: 'bg-emerald-500/10 text-emerald-300 border-emerald-500/30',
    warn: 'bg-amber-500/10 text-amber-300 border-amber-500/30',
    err: 'bg-red-500/10 text-red-300 border-red-500/30',
    idle: 'bg-slate-500/10 text-slate-400 border-slate-500/30',
    info: 'bg-sky-500/10 text-sky-300 border-sky-500/30',
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
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60" />
        )}
        <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-current" />
      </span>
      {label}
    </span>
  )
}