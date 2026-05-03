import { useEffect, useState } from 'react'
import { api, type TrafficSummary, type TrafficTopRow } from '../api/client'
import { ArrowDown, ArrowUp, Activity as ActivityIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`
  if (n < 1024 * 1024 * 1024 * 1024) return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`
  return `${(n / 1024 / 1024 / 1024 / 1024).toFixed(2)} TB`
}

interface ActivityProps {
  /** Filter parameter (one of) — drives both summary and top list */
  userId?: string
  resourceId?: string
  serverId?: string
  /** What dimension to group the top list by */
  topBy: 'user' | 'resource' | 'destination' | 'port'
  /** Hours back to summarize (default 24) */
  hours?: number
  /** Top N (default 5) */
  topN?: number
  /** Heading text for the top list */
  topTitle?: string
  /** Substitute label for the top list when empty */
  emptyLabel?: string
}

export function Activity({
  userId, resourceId, serverId, topBy, hours = 24, topN = 5, topTitle, emptyLabel,
}: ActivityProps) {
  const [summary, setSummary] = useState<TrafficSummary | null>(null)
  const [top, setTop] = useState<TrafficTopRow[]>([])
  const since = new Date(Date.now() - hours * 60 * 60 * 1000).toISOString()

  useEffect(() => {
    const filter = { user_id: userId, resource_id: resourceId, server_id: serverId, since }
    api.trafficSummary(filter).then(setSummary).catch(() => {})
    api.trafficTop({ by: topBy, ...filter, limit: topN }).then(setTop).catch(() => {})
  }, [userId, resourceId, serverId, topBy, hours, topN])

  const total = (summary?.bytes_tx ?? 0) + (summary?.bytes_rx ?? 0)
  const max = top.reduce((m, r) => Math.max(m, r.bytes_tx + r.bytes_rx), 0)

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-3 gap-3">
        <Stat label={`Total · ${hours}h`}     value={formatBytes(total)}                      icon={ActivityIcon} />
        <Stat label="Sent (TX)"               value={formatBytes(summary?.bytes_tx ?? 0)}     icon={ArrowUp} />
        <Stat label="Received (RX)"           value={formatBytes(summary?.bytes_rx ?? 0)}     icon={ArrowDown} />
      </div>

      <div>
        <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
          {topTitle ?? `Top ${topBy}s`}
        </p>
        {top.length === 0 ? (
          <p className="text-xs text-muted-foreground italic">{emptyLabel ?? 'No activity in this window.'}</p>
        ) : (
          <div className="space-y-1">
            {top.map(r => {
              const t = r.bytes_tx + r.bytes_rx
              const pct = max > 0 ? (t / max) * 100 : 0
              return (
                <div key={r.key} className="grid grid-cols-[1fr_auto] gap-2 items-center py-1">
                  <div className="min-w-0">
                    <div className="flex items-baseline justify-between gap-2 mb-0.5">
                      <span className="text-sm truncate">{r.label}</span>
                      <span className="text-[11px] text-muted-foreground tabular-nums whitespace-nowrap">{formatBytes(t)}</span>
                    </div>
                    <div className="h-1.5 rounded-full bg-secondary overflow-hidden">
                      <div className="h-full bg-primary/60 transition-all" style={{ width: `${pct}%` }} />
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}

function Stat({ label, value, icon: Icon }: { label: string; value: string; icon: React.ElementType }) {
  return (
    <div className={cn('rounded-md border border-border bg-card/60 px-3 py-2.5')}>
      <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-wider text-muted-foreground font-semibold">
        <Icon className="w-3 h-3" /> {label}
      </div>
      <p className="text-base font-semibold tabular-nums mt-0.5">{value}</p>
    </div>
  )
}
