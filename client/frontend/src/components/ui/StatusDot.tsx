import { cn } from '../../lib/cn'

export type Status = 'connected' | 'connecting' | 'reconnecting' | 'disconnected' | 'error'

const COLOR: Record<Status, string> = {
  connected:    'bg-success',
  connecting:   'bg-warning',
  reconnecting: 'bg-warning',
  disconnected: 'bg-muted-foreground/60',
  error:        'bg-destructive',
}

export function StatusDot({
  status, pulse, className,
}: { status: Status; pulse?: boolean; className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={cn('inline-block w-2 h-2 rounded-full', COLOR[status], pulse && 'animate-pulse-slow', className)}
    />
  )
}

export function StatusPill({ status, label, className }: { status: Status; label?: string; className?: string }) {
  const text = label ?? status
  const tone =
    status === 'connected'    ? 'text-success ring-success/20 bg-success/10' :
    status === 'connecting'   ? 'text-warning ring-warning/25 bg-warning/10' :
    status === 'reconnecting' ? 'text-warning ring-warning/25 bg-warning/10' :
    status === 'error'        ? 'text-destructive ring-destructive/25 bg-destructive/10' :
                                'text-muted-foreground ring-border bg-muted/40'
  return (
    <span className={cn('inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-medium ring-1', tone, className)}>
      <StatusDot status={status} pulse={status === 'connected' || status === 'connecting' || status === 'reconnecting'} />
      <span className="capitalize">{text}</span>
    </span>
  )
}
