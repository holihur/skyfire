import type { Lang } from '../i18n'

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

const AGE: Record<Lang, { never: string; now: string; sAgo: string; mAgo: string; hMAgo: string; dAgo: string }> = {
  en: { never: 'never', now: 'now', sAgo: '{n}s ago', mAgo: '{n}m ago', hMAgo: '{h}h {m}m ago', dAgo: '{d}d ago' },
  zh: { never: '从未', now: '刚刚', sAgo: '{n} 秒前', mAgo: '{n} 分钟前', hMAgo: '{h} 小时 {m} 分钟前', dAgo: '{d} 天前' },
}

export function fmtAge(t: string | undefined | null, lang: Lang = 'en'): string {
  const fmt = AGE[lang]
  if (!t || t === '0001-01-01T00:00:00Z') return fmt.never
  const ms = Date.now() - new Date(t).getTime()
  if (ms < 0) return fmt.now
  const s = Math.floor(ms / 1000)
  if (s < 60) return fmt.sAgo.replace('{n}', String(s))
  const m = Math.floor(s / 60)
  if (m < 60) return fmt.mAgo.replace('{n}', String(m))
  const h = Math.floor(m / 60)
  if (h < 24) return fmt.hMAgo.replace('{h}', String(h)).replace('{m}', String(m % 60))
  const d = Math.floor(h / 24)
  return fmt.dAgo.replace('{d}', String(d))
}

export function fmtDate(t: string, lang: Lang = 'en'): string {
  const d = new Date(t)
  return d.toLocaleString(lang === 'zh' ? 'zh-CN' : undefined, { dateStyle: 'medium', timeStyle: 'short' })
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