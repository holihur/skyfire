export function fmtBytes(n: number | bigint): string {
  let v = Number(n)
  if (Number.isNaN(v) || v < 0) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

export function fmtAge(t: string | undefined | null): string {
  if (!t || t === '0001-01-01T00:00:00Z') return 'never'
  const ms = Date.now() - new Date(t).getTime()
  if (ms < 0) return 'now'
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s}s ago`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ${m % 60}m ago`
  const d = Math.floor(h / 24)
  return `${d}d ago`
}

export function fmtDate(t: string): string {
  const d = new Date(t)
  return d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}

export function shortKey(k: string, n = 16): string {
  if (!k) return ''
  return k.length > n ? `${k.slice(0, n)}…` : k
}

export function ipOf(cidr: string): string {
  return cidr.split('/')[0]
}

export function parseList(s: string): string[] {
  return s
    .split(/[\n,]+/)
    .map((x) => x.trim())
    .filter(Boolean)
}

export function joinList(list: string[] | undefined | null): string {
  return (list ?? []).join('\n')
}