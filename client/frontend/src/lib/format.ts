/** "1.21 GB" / "543 KB" / "12 B" */
export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return '—'
  if (n < 1024) return `${n} B`
  if (n < 1024 ** 2) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 ** 3) return `${(n / 1024 ** 2).toFixed(1)} MB`
  if (n < 1024 ** 4) return `${(n / 1024 ** 3).toFixed(2)} GB`
  return `${(n / 1024 ** 4).toFixed(2)} TB`
}

/** "12 KB/s" — for live rate */
export function formatRate(bps: number): string {
  return formatBytes(bps) + '/s'
}

/** "14s ago", "3m ago", "—" if zero/null */
export function formatHandshake(unixSec: number | null | undefined): string {
  if (!unixSec) return 'never'
  const diff = Math.floor(Date.now() / 1000 - unixSec)
  if (diff < 0) return 'just now'
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

/** Cap a string with mid-truncation for keys: "abc...xyz=". Safe for nullish input. */
export function midTruncate(s: string | null | undefined, head = 8, tail = 6): string {
  if (s == null || s === '') return '—'
  if (s.length <= head + tail + 1) return s
  return `${s.slice(0, head)}…${s.slice(-tail)}`
}
