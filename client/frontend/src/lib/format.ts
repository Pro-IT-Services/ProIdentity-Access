import { fmtDecimal, t, tp } from '../i18n'

/** "1.21 GB" / "543 KB" / "12 B" (decimal comma in Slovak) */
export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return '—'
  if (n < 1024) return `${n} B`
  if (n < 1024 ** 2) return `${fmtDecimal(n / 1024, 1)} KB`
  if (n < 1024 ** 3) return `${fmtDecimal(n / 1024 ** 2, 1)} MB`
  if (n < 1024 ** 4) return `${fmtDecimal(n / 1024 ** 3, 2)} GB`
  return `${fmtDecimal(n / 1024 ** 4, 2)} TB`
}

/** "12 KB/s" — for live rate */
export function formatRate(bps: number): string {
  return formatBytes(bps) + '/s'
}

/** "14s ago", "3m ago" ("pred 14 s", "pred 3 min"), "never" if zero/null */
export function formatHandshake(unixSec: number | null | undefined): string {
  if (!unixSec) return t('time.never')
  const diff = Math.floor(Date.now() / 1000 - unixSec)
  if (diff < 0) return t('time.justNow')
  if (diff < 60) return t('time.secondsAgo', { n: diff })
  if (diff < 3600) return t('time.minutesAgo', { n: Math.floor(diff / 60) })
  if (diff < 86400) return t('time.hoursAgo', { n: Math.floor(diff / 3600) })
  return tp('time.daysAgo', Math.floor(diff / 86400))
}

/** "42s", "5m", "2h 05m", "3d 4h" since unixSec; "" if zero/null */
export function formatSince(unixSec: number | null | undefined): string {
  if (!unixSec) return ''
  const d = Math.max(0, Math.floor(Date.now() / 1000 - unixSec))
  if (d < 60) return t('time.seconds', { n: d })
  if (d < 3600) return t('time.minutes', { n: Math.floor(d / 60) })
  if (d < 86400) return t('time.hoursMinutes', { h: Math.floor(d / 3600), m: String(Math.floor((d % 3600) / 60)).padStart(2, '0') })
  return t('time.daysHours', { d: Math.floor(d / 86400), h: Math.floor((d % 86400) / 3600) })
}

/** Cap a string with mid-truncation for keys: "abc...xyz=". Safe for nullish input. */
export function midTruncate(s: string | null | undefined, head = 8, tail = 6): string {
  if (s == null || s === '') return '—'
  if (s.length <= head + tail + 1) return s
  return `${s.slice(0, head)}…${s.slice(-tail)}`
}
