import { Loader2, Power } from 'lucide-react'
import { Button } from './ui/Button'
import { StatusDot, type Status } from './ui/StatusDot'
import { cn } from '../lib/cn'
import { t } from '../i18n'

interface Props {
  name: string
  icon: React.ElementType
  /** Effective status (already "connecting" while busy). */
  status: Status
  /** Mono detail after the status, e.g. a subnet or address. */
  sub?: string
  /** Small chips after the status line, e.g. TAP / TOTP. */
  tags?: React.ReactNode
  busy?: boolean
  /** Connected or connecting: the button disconnects. */
  live: boolean
  selected?: boolean
  onSelect?: () => void
  onConnect: () => void
  onDisconnect: () => void
  /** The connect button cancels instead (an inline connect form is open). */
  cancelMode?: boolean
  /** Replaces the status label, e.g. "In use by Jana on LAPTOP-7". */
  statusText?: string
  /** Connecting isn't possible right now; the reason is the button's tooltip. */
  connectBlocked?: string
  /** Extra controls left of the main button (e.g. delete). */
  actions?: React.ReactNode
  /** Shown under the row: errors, details, inline forms. */
  children?: React.ReactNode
}

function statusLabel(s: Status): string {
  switch (s) {
    case 'connected':    return t('status.connected')
    case 'connecting':   return t('status.connecting')
    case 'reconnecting': return t('status.reconnecting')
    case 'error':        return t('status.error')
    case 'disconnected': return t('status.available')
  }
}

/** One connection in the connections sheet — the same layout for every kind. */
export function ConnRow({
  name, icon: Icon, status, sub, tags, busy, live, selected, onSelect, onConnect, onDisconnect, cancelMode, statusText, connectBlocked, actions, children,
}: Props) {
  const connected = status === 'connected'
  const pending = busy || status === 'connecting' || status === 'reconnecting'
  return (
    <div
      onClick={onSelect}
      className={cn(
        'relative px-4 py-3 border-b border-border last:border-0 transition-colors',
        onSelect && 'cursor-pointer hover:bg-secondary/30',
        selected && 'bg-secondary/40',
        busy && 'bg-warning/5',
      )}
    >
      {busy && (
        <span aria-hidden="true" className="absolute left-0 right-0 bottom-0 h-0.5 overflow-hidden">
          <span className="absolute inset-y-0 w-1/3 bg-warning conn-row-progress" />
        </span>
      )}

      <div className="grid grid-cols-[auto_minmax(0,1fr)_auto] gap-3 items-center">
        <span className={cn(
          'w-8 h-8 rounded-lg flex items-center justify-center border transition-colors',
          connected ? 'bg-success/10 border-success/30 text-success'
            : pending ? 'bg-warning/10 border-warning/30 text-warning'
            : 'bg-secondary/40 border-border text-muted-foreground',
        )}>
          {busy ? <Loader2 className="w-4 h-4 animate-spin" /> : <Icon className="w-4 h-4" />}
        </span>

        <div className="min-w-0">
          <p className="text-sm font-semibold leading-snug break-words line-clamp-2" title={name}>{name}</p>
          <div className="flex items-center gap-1.5 mt-1 min-w-0 text-[11px]">
            <StatusDot status={status} pulse={pending} className="shrink-0" />
            <span className={cn(
              statusText && !busy ? 'min-w-0 truncate' : 'shrink-0',
              connected ? 'text-success font-medium' : pending ? 'text-warning font-medium' : status === 'error' ? 'text-destructive'
                : connectBlocked ? 'text-warning' : 'text-muted-foreground',
            )} title={statusText}>
              {busy ? (live ? t('common.disconnectingEllipsis') : t('common.connectingEllipsis')) : statusText ?? statusLabel(status)}
            </span>
            {sub && <span className="min-w-0 truncate font-mono text-muted-foreground" title={sub}>· {sub}</span>}
            {tags}
          </div>
        </div>

        <div className="flex items-center gap-1 shrink-0">
          {actions}
          {live ? (
            <Button
              variant="outline" size="sm" disabled={busy}
              onClick={e => { e.stopPropagation(); onDisconnect() }}
              className="min-w-[96px] text-destructive border-destructive/40 hover:bg-destructive/10"
            >
              <Power className="w-3.5 h-3.5" /> {t('common.disconnect')}
            </Button>
          ) : cancelMode ? (
            <Button
              variant="secondary" size="sm"
              onClick={e => { e.stopPropagation(); onConnect() }}
              className="min-w-[96px]"
            >
              {t('common.cancel')}
            </Button>
          ) : (
            <Button
              variant="primary" size="sm" disabled={busy || !!connectBlocked}
              title={connectBlocked}
              onClick={e => { e.stopPropagation(); onConnect() }}
              className="min-w-[96px]"
            >
              <Power className="w-3.5 h-3.5" /> {t('common.connect')}
            </Button>
          )}
        </div>
      </div>

      {children && <div className="pl-11 min-w-0" onClick={e => e.stopPropagation()}>{children}</div>}
    </div>
  )
}

/** Small chip on a row's status line (TAP, TOTP). */
export function RowTag({ children, tone }: { children: React.ReactNode; tone?: 'primary' }) {
  return (
    <span className={cn(
      'shrink-0 inline-flex items-center gap-0.5 uppercase text-[10px] font-medium px-1 rounded border',
      tone === 'primary' ? 'text-primary border-primary/30' : 'text-muted-foreground border-border',
    )}>
      {children}
    </span>
  )
}

/** Card holding one group of rows under a small protocol header. */
export function ConnGroupCard({ children }: { children: React.ReactNode }) {
  return <div className="rounded-xl border border-border bg-card overflow-hidden">{children}</div>
}

export function ProtocolHeader({ label, count }: { label: string; count: number }) {
  return (
    <div className="px-4 py-1.5 border-b border-border bg-secondary/30 flex items-center justify-between">
      <span className="text-[11px] font-semibold text-muted-foreground">{label}</span>
      <span className="text-[11px] text-muted-foreground tabular-nums">{count}</span>
    </div>
  )
}
