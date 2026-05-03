import { useEffect, useMemo, useState } from 'react'
import type { TunnelInfo } from '../../types'
import { Power, PowerOff, RefreshCw, ArrowDown, ArrowUp } from 'lucide-react'
import { LogoMark } from '../brand/LogoMark'
import { useTunnelStore } from '../../stores/useTunnelStore'
import { useTrafficHistory, type ThroughputSample } from '../../stores/useTrafficHistory'
import { Button } from '../ui/Button'
import { Sparkline } from '../ui/Sparkline'
import type { Status } from '../ui/StatusDot'
import { StatusOrb } from './StatusOrb'
import { EndpointLine } from './EndpointLine'
import { GridBackground } from './GridBackground'
import { formatBytes, formatRate, formatHandshake } from '../../lib/format'
import { cn } from '../../lib/cn'

interface Props {
  tunnel: TunnelInfo | null
  onOpenConnections?: () => void
  onImport?: () => void
  /** When provided, primary action defers to host (App handles synth vs real). */
  onConnect?: () => void | Promise<void>
  onDisconnect?: () => void | Promise<void>
}

// Stable reference for the empty case — selectors that return a fresh `[]`
// every render cause zustand to think state changed and infinite-loop.
const EMPTY_HISTORY: ThroughputSample[] = []

const STATE_LABEL: Record<Status, string> = {
  connected:    'connected',
  connecting:   'connecting…',
  reconnecting: 'reconnecting…',
  disconnected: 'disconnected',
  error:        'error',
}

export function MissionControl({ tunnel, onOpenConnections, onImport, onConnect, onDisconnect }: Props) {
  // ALL hooks must run before any early return. React errors if the hook count
  // changes between renders (this is what crashed earlier).
  const { stats } = useTunnelStore()
  const history = useTrafficHistory(s => tunnel ? (s.history[tunnel.id] ?? EMPTY_HISTORY) : EMPTY_HISTORY)

  // Tick once per second so "handshake 14s ago" stays accurate.
  const [, force] = useState(0)
  useEffect(() => {
    if (tunnel?.status !== 'connected') return
    const t = setInterval(() => force(x => x + 1), 1000)
    return () => clearInterval(t)
  }, [tunnel?.status])

  const lastRx = history.length ? history[history.length - 1].rxBps : 0
  const lastTx = history.length ? history[history.length - 1].txBps : 0
  // Map total bps logarithmically into 0..1 for the orb halo intensity.
  const intensity = useMemo(() => {
    const total = lastRx + lastTx
    if (total <= 0) return 0
    return Math.min(1, Math.log10(total + 10) / 7)
  }, [lastRx, lastTx])

  // Empty state — no tunnel selected at all. Now with real CTAs.
  if (!tunnel) {
    return (
      <div className="relative flex-1 flex items-center justify-center">
        <GridBackground />
        <div className="relative text-center max-w-sm px-6">
          <StatusOrb status="disconnected" intensity={0} size={200} />
          <p className="mt-6 text-base font-semibold text-foreground">Nothing connected</p>
          <p className="mt-1 text-xs text-muted-foreground mb-5">
            Pick a server, or import a WireGuard config.
          </p>
          <div className="flex items-center justify-center gap-2">
            {onOpenConnections && (
              <Button variant="primary" size="md" onClick={onOpenConnections}>
                <LogoMark size={16} /> Show connections
              </Button>
            )}
            {onImport && (
              <Button variant="outline" size="md" onClick={onImport}>
                Import config
              </Button>
            )}
          </div>
        </div>
      </div>
    )
  }

  const status: Status =
    tunnel.status === 'connected' ? 'connected'
    : tunnel.status === 'connecting' ? 'connecting'
    : tunnel.status === 'error' ? 'error'
    : 'disconnected'

  const tStats = stats[tunnel.id]
  const isConnected  = status === 'connected'
  const isConnecting = status === 'connecting'
  const isError      = status === 'error'

  const handleToggle = async () => {
    if (isConnected) {
      await onDisconnect?.()
    } else {
      await onConnect?.()
    }
  }

  const endpoint = tunnel.peers[0]?.endpoint ?? ''
  const endpointHost = endpoint.split(':')[0] || endpoint
  const addr = tunnel.addresses[0] ?? '—'

  return (
    <div className="relative flex-1 flex flex-col">
      <GridBackground />

      {/* Vertical stack — every row independently centered. */}
      <div className="relative flex-1 flex flex-col items-center justify-center px-6 py-6 gap-5">
        <StatusOrb status={status} intensity={intensity} size={210} />

        {/* State + name */}
        <div className="text-center">
          <p className={cn(
            'text-[11px] uppercase tracking-[0.2em] font-semibold mb-1',
            isConnected ? 'text-success'
            : isConnecting ? 'text-warning'
            : isError ? 'text-destructive'
            : 'text-muted-foreground',
          )}>
            {STATE_LABEL[status]}
          </p>
          <h1 className="text-3xl sm:text-4xl font-semibold tracking-tight">{tunnel.name}</h1>
          <p className="text-xs text-muted-foreground font-mono mt-1">{addr}</p>
        </div>

        {/* Endpoint line — below the name, where it has space and stays centered. */}
        {endpoint && (
          <div className="flex items-center gap-3 text-[11px] text-muted-foreground">
            <span className="font-mono">you</span>
            <EndpointLine
              active={isConnected}
              length={120}
              color={
                isConnected ? 'hsl(var(--success))'
                : isConnecting ? 'hsl(var(--warning))'
                : isError ? 'hsl(var(--destructive))'
                : 'hsl(var(--muted-foreground))'
              }
            />
            <span className="font-mono text-foreground/85">{endpointHost}</span>
          </div>
        )}

        {/* Throughput pair — only when connected */}
        {isConnected && (
          <div className="grid grid-cols-2 gap-8 w-full max-w-sm">
            <ThroughputCell
              icon={<ArrowDown className="w-3.5 h-3.5" />}
              label="Received"
              rate={lastRx}
              data={history.map(h => h.rxBps)}
              total={tStats?.rx_bytes ?? 0}
              color="hsl(var(--success))"
            />
            <ThroughputCell
              icon={<ArrowUp className="w-3.5 h-3.5" />}
              label="Sent"
              rate={lastTx}
              data={history.map(h => h.txBps)}
              total={tStats?.tx_bytes ?? 0}
              color="hsl(var(--primary))"
            />
          </div>
        )}

        {/* Error message */}
        {isError && tunnel.error && (
          <div className="max-w-md text-center text-sm text-destructive">
            {tunnel.error}
          </div>
        )}

        {/* Primary action */}
        {isConnecting ? (
          <Button variant="ghost" size="lg" disabled className="min-w-[200px]">
            <RefreshCw className="w-4 h-4 animate-spin" />
            Connecting…
          </Button>
        ) : isConnected ? (
          <Button variant="destructive" size="lg" onClick={handleToggle} className="min-w-[200px]">
            <PowerOff className="w-4 h-4" />
            Disconnect
          </Button>
        ) : (
          <Button variant="primary" size="lg" onClick={handleToggle} className="min-w-[200px]">
            <LogoMark size={18} />
            {isError ? 'Retry' : 'Connect'}
          </Button>
        )}

        {/* Footnote */}
        {isConnected && tStats && (
          <p className="text-[11px] text-muted-foreground tabular-nums">
            handshake {formatHandshake(tStats.last_handshake)}
            {(tStats.rx_bytes + tStats.tx_bytes > 0) && (
              <> · {formatBytes(tStats.rx_bytes + tStats.tx_bytes)} total</>
            )}
          </p>
        )}
      </div>
    </div>
  )
}

function ThroughputCell({
  icon, label, rate, data, total, color,
}: { icon: React.ReactNode; label: string; rate: number; data: number[]; total: number; color: string }) {
  return (
    <div className="text-center">
      <div className="flex items-center justify-center gap-1 text-[10px] uppercase tracking-wider text-muted-foreground font-semibold mb-1.5">
        <span style={{ color }}>{icon}</span> {label}
      </div>
      <p className="text-xl font-semibold tabular-nums" style={{ color }}>{formatRate(rate)}</p>
      <div className="flex justify-center mt-1.5" style={{ color }}>
        <Sparkline data={data} width={140} height={20} color={color} />
      </div>
      <p className="text-[10px] text-muted-foreground mt-1 tabular-nums">{formatBytes(total)} total</p>
    </div>
  )
}
