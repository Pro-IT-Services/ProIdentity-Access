import { ArrowDown, ArrowUp, ExternalLink, LockKeyhole, Power, ShieldCheck, X } from 'lucide-react'
import type { ReactNode } from 'react'
import type { TunnelInfo, StatsInfo, ManagedSettings } from '../../types'
import type { ThroughputSample } from '../../stores/useTrafficHistory'
import { Sparkline } from '../ui/Sparkline'
import { formatBytes, formatHandshake } from '../../lib/format'
import { cn } from '../../lib/cn'

interface Props {
  tunnel: TunnelInfo | null
  stats?: StatsInfo
  history: ThroughputSample[]
  settings: ManagedSettings
  authState?: 'push' | 'approving' | 'required' | 'disabled'
  onConnect: () => void | Promise<void>
  onDisconnect: () => void | Promise<void>
  onOpenApp: () => void
  onClose: () => void
}

export function TrayMiniDashboard({
  tunnel,
  stats,
  history,
  settings,
  authState = 'push',
  onConnect,
  onDisconnect,
  onOpenApp,
  onClose,
}: Props) {
  const status = tunnel?.status ?? 'disconnected'
  const connected = status === 'connected'
  const approving = authState === 'approving'
  const connecting = status === 'connecting' || approving
  const errored = status === 'error'
  const graph = history.map(sample => sample.rxBps + sample.txBps)
  const totalBytes = (stats?.rx_bytes ?? 0) + (stats?.tx_bytes ?? 0)

  const connectionLabel =
    connected ? 'Connected'
    : connecting ? 'Connecting'
    : errored ? 'Needs attention'
    : 'Disconnected'

  const twoFaLabel =
    authState === 'required' ? '2FA required'
    : authState === 'approving' ? '2FA approving'
    : authState === 'disabled' ? '2FA disabled'
    : settings.logged_in ? '2FA push'
    : '2FA unavailable'

  const sessionLabel =
    authState === 'required' ? 'Expired'
    : authState === 'approving' ? 'Approving'
    : settings.logged_in ? 'Verified'
    : 'Signed out'

  const title = tunnel?.name ?? (settings.vpn_name || 'ProIdentity Access')
  const subtitle =
    tunnel?.is_managed ? (settings.vpn_name || 'Managed VPN')
    : tunnel ? 'Imported tunnel'
    : 'No active tunnel'

  return (
    <div className="h-full w-full overflow-hidden bg-background text-foreground">
      <div className="flex h-full flex-col overflow-hidden">
          <div className="tray-header drag flex items-start justify-between gap-3 border-b border-border/70 bg-background/95">
            <div className="flex min-w-0 items-center gap-3">
              <div className="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-primary/25 bg-primary/10 text-primary">
                <ShieldCheck className="h-5 w-5" />
              </div>
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold">ProIdentity Access</p>
                <p className="truncate text-[11px] text-muted-foreground">{subtitle}</p>
              </div>
            </div>
            <div className="flex shrink-0 items-start gap-2">
              <div className="grid justify-items-end gap-1.5">
                <StatusPill state={connecting && !connected ? 'connecting' : status}>{connectionLabel}</StatusPill>
                <span className={cn(
                  'inline-flex items-center gap-1.5 rounded-full border px-2 py-1 text-[11px] font-semibold',
                  authState === 'required'
                    ? 'border-destructive/35 bg-destructive/15 text-orange-200'
                    : 'border-primary/25 bg-primary/10 text-blue-200',
                )}>
                  <LockKeyhole className="h-3 w-3" />
                  {twoFaLabel}
                </span>
              </div>
              <button
                type="button"
                onClick={onClose}
                className="no-drag grid h-7 w-7 place-items-center rounded-md text-muted-foreground transition hover:bg-secondary hover:text-foreground"
                aria-label="Close tray dashboard"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          </div>

          <div className="flex-1 overflow-hidden px-5 py-4">
              <div className="mb-4 flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <h1 className="truncate text-base font-semibold">{title}</h1>
                  <p className="text-[11px] text-muted-foreground">
                  {connected ? `${formatBytes(totalBytes)} total` : approving ? 'Waiting for 2FA approval' : connecting ? 'Waiting for tunnel approval' : 'Ready when you are'}
                  </p>
                </div>
                <p className="shrink-0 text-[11px] text-muted-foreground">
                  {connected ? formatHandshake(stats?.last_handshake) : approving ? '2FA' : connecting ? 'pending' : 'idle'}
                </p>
              </div>

              <div className="mb-3 h-[106px] border-b border-border/80 px-1 py-2">
                <Sparkline
                  data={graph.length > 1 ? graph : [0, 0]}
                  width={374}
                  height={88}
                  color="hsl(var(--primary))"
                  className="h-full w-full"
                />
              </div>

              <div className="grid grid-cols-2 gap-2">
                <Metric
                  icon={<ArrowDown className="h-3.5 w-3.5" />}
                  label="Down"
                  value={formatBytes(stats?.rx_bytes ?? 0)}
                  caption="total"
                  tone="success"
                />
                <Metric
                  icon={<ArrowUp className="h-3.5 w-3.5" />}
                  label="Up"
                  value={formatBytes(stats?.tx_bytes ?? 0)}
                  caption="total"
                  tone="primary"
                />
              </div>

              <div className="mt-2 grid grid-cols-2 gap-2">
                <InfoTile label="2FA" value={twoFaLabel.replace('2FA ', '')} />
                <InfoTile label="Session" value={sessionLabel} />
              </div>

            {errored && tunnel?.error && (
              <p className="mt-3 rounded-lg border border-destructive/25 bg-destructive/10 px-3 py-2 text-xs text-orange-200">
                {tunnel.error}
              </p>
            )}

            <div className="mt-3 grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={connected ? onDisconnect : onConnect}
                disabled={approving}
                className={cn(
                  'no-drag inline-flex h-10 items-center justify-center gap-2 rounded-lg border text-sm font-semibold transition',
                  connected
                    ? 'border-destructive/35 bg-destructive/10 text-orange-100 hover:bg-destructive/20'
                    : approving
                    ? 'border-warning/35 bg-warning/15 text-amber-100'
                    : 'border-primary/45 bg-primary text-primary-foreground hover:bg-primary/90',
                )}
              >
                <Power className="h-4 w-4" />
                {connected ? 'Disconnect' : approving ? 'Waiting for 2FA' : connecting ? 'Connecting' : 'Connect'}
              </button>
              <button
                type="button"
                onClick={onOpenApp}
                className="no-drag inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-border bg-secondary text-sm font-semibold transition hover:bg-secondary/80"
              >
                <ExternalLink className="h-4 w-4" />
                Full App
              </button>
            </div>
          </div>
      </div>
    </div>
  )
}

function StatusPill({ state, children }: { state: TunnelInfo['status'] | 'disconnected'; children: string }) {
  const cls =
    state === 'connected' ? 'border-success/30 bg-success/10 text-green-200'
    : state === 'connecting' ? 'border-warning/30 bg-warning/10 text-amber-200'
    : state === 'error' ? 'border-destructive/35 bg-destructive/15 text-orange-200'
    : 'border-border bg-secondary text-muted-foreground'
  return (
    <span className={cn('inline-flex items-center gap-1.5 rounded-full border px-2 py-1 text-[11px] font-semibold', cls)}>
      <span className="h-1.5 w-1.5 rounded-full bg-current shadow-[0_0_10px_currentColor]" />
      {children}
    </span>
  )
}

function Metric({
  icon,
  label,
  value,
  caption,
  tone,
}: {
  icon: ReactNode
  label: string
  value: string
  caption: string
  tone: 'success' | 'primary'
}) {
  return (
    <div className="rounded-lg border border-border bg-card/70 p-2.5">
      <p className={cn(
        'mb-1 flex items-center gap-1.5 text-[11px] text-muted-foreground',
        tone === 'success' ? '[&>svg]:text-success' : '[&>svg]:text-primary',
      )}>
        {icon}
        {label}
      </p>
      <p className="truncate text-base font-semibold">{value}</p>
      <p className="truncate text-[11px] text-muted-foreground">{caption}</p>
    </div>
  )
}

function InfoTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-card/55 p-2.5">
      <p className="mb-1 text-[10px] font-bold uppercase tracking-[0.12em] text-muted-foreground">{label}</p>
      <p className="truncate text-xs font-semibold">{value}</p>
    </div>
  )
}
