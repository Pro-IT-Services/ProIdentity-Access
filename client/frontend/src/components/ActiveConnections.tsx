import { useEffect, useState } from 'react'
import { ArrowDown, ArrowUp, PowerOff, Loader2 } from 'lucide-react'
import { useActiveConnections } from '../lib/activeConnections'
import { formatBytes, formatSince } from '../lib/format'
import { StatusDot } from './ui/StatusDot'
import { cn } from '../lib/cn'

interface Props {
  /** The connection shown in the big status card, highlighted here. */
  focusedId?: string
  onDisconnect: (id: string) => Promise<void> | void
}

const MAX_CHIPS = 4

/**
 * Every live VPN on this computer (WireGuard and OpenVPN), with the networks
 * each one routes. Two connections can't share a network: the service refuses
 * the second one and says which connection is in the way.
 */
export function ActiveConnections({ focusedId, onDisconnect }: Props) {
  const conns = useActiveConnections()
  const [busy, setBusy] = useState<string | null>(null)

  // Keep the durations ticking.
  const [, tick] = useState(0)
  useEffect(() => {
    if (!conns.length) return
    const t = setInterval(() => tick(x => x + 1), 1000)
    return () => clearInterval(t)
  }, [conns.length])

  if (!conns.length) return null

  const disconnect = async (id: string) => {
    setBusy(id)
    try { await onDisconnect(id) } finally { setBusy(null) }
  }

  return (
    <aside aria-label="Connected now" className="relative w-72 shrink-0 border-l border-border bg-background/70 backdrop-blur flex flex-col min-h-0">
      <p className="px-4 pt-4 pb-2 text-[10px] uppercase tracking-[0.18em] font-semibold text-muted-foreground">
        Connected now · {conns.length}
      </p>
      <ul className="flex-1 overflow-y-auto px-3 pb-3 space-y-2">
        {conns.map(c => {
          const extra = c.networks.length - MAX_CHIPS
          return (
            <li
              key={c.id}
              className={cn(
                'rounded-lg border px-3 py-2.5',
                c.id === focusedId ? 'border-primary/40 bg-primary/5' : 'border-border bg-secondary/30',
              )}
            >
              <div className="flex items-center gap-2 min-w-0">
                <StatusDot status={c.status} pulse={c.status === 'connecting'} className="shrink-0" />
                <span className="text-sm font-medium truncate flex-1">{c.name}</span>
                <button
                  onClick={() => disconnect(c.id)}
                  disabled={busy === c.id}
                  aria-label={`Disconnect ${c.name}`}
                  title="Disconnect"
                  className="no-drag shrink-0 -mr-1 p-1.5 rounded-md text-destructive hover:bg-destructive/10 disabled:opacity-50 cursor-pointer"
                >
                  {busy === c.id ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <PowerOff className="w-3.5 h-3.5" />}
                </button>
              </div>
              <div className="flex items-center gap-2 mt-1 text-[11px]">
                <span className="text-[10px] px-1.5 py-px rounded border border-border text-muted-foreground">{c.kind}</span>
                {c.ip && <span className="font-mono text-foreground/80 truncate">{c.ip}</span>}
              </div>
              <div className="flex flex-wrap items-center gap-1 mt-1.5">
                {c.networks.slice(0, MAX_CHIPS).map(n => (
                  <span key={n} className="font-mono text-[10px] px-1.5 py-px rounded bg-secondary text-foreground/85">{n}</span>
                ))}
                {extra > 0 && (
                  <span className="text-[10px] text-muted-foreground" title={c.networks.slice(MAX_CHIPS).join(', ')}>
                    +{extra} more
                  </span>
                )}
                {!c.networks.length && c.status === 'connecting' && (
                  <span className="text-[10px] text-muted-foreground">waiting for the server's networks</span>
                )}
              </div>
              <div className="mt-1.5 text-[11px] text-muted-foreground tabular-nums space-y-0.5">
                {c.server && <p className="truncate">via <span className="font-mono">{c.server}</span>{c.status === 'connected' && c.since > 0 && <> · {formatSince(c.since)}</>}</p>}
                {c.status === 'connected' ? (
                  <p className="inline-flex items-center gap-1">
                    <ArrowDown className="w-3 h-3" />{formatBytes(c.rx)}
                    <ArrowUp className="w-3 h-3 ml-1.5" />{formatBytes(c.tx)}
                  </p>
                ) : (
                  <p className="text-warning">connecting…</p>
                )}
              </div>
            </li>
          )
        })}
      </ul>
    </aside>
  )
}
