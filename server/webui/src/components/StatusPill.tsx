import { cn } from '@/lib/utils'

export type StatusKind = 'ok' | 'warn' | 'down' | 'idle' | 'info'

const KIND: Record<StatusKind, { dot: string; text: string; bg: string; ring: string }> = {
  ok:   { dot: 'bg-success', text: 'text-success', bg: 'bg-success/10', ring: 'ring-success/20' },
  warn: { dot: 'bg-warning', text: 'text-warning', bg: 'bg-warning/10', ring: 'ring-warning/20' },
  down: { dot: 'bg-destructive', text: 'text-destructive', bg: 'bg-destructive/10', ring: 'ring-destructive/20' },
  idle: { dot: 'bg-muted-foreground/60', text: 'text-muted-foreground', bg: 'bg-muted/40', ring: 'ring-border' },
  info: { dot: 'bg-primary', text: 'text-primary', bg: 'bg-primary/10', ring: 'ring-primary/20' },
}

export function StatusPill({
  kind, label, pulse = false, className,
}: { kind: StatusKind; label: string; pulse?: boolean; className?: string }) {
  const k = KIND[kind]
  return (
    <span className={cn(
      'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-medium ring-1',
      k.bg, k.text, k.ring, className,
    )}>
      <span className={cn('w-1.5 h-1.5 rounded-full', k.dot, pulse && 'animate-pulse')} />
      {label}
    </span>
  )
}

export function StatusDot({ kind, pulse, className }: { kind: StatusKind; pulse?: boolean; className?: string }) {
  return <span className={cn('inline-block w-2 h-2 rounded-full', KIND[kind].dot, pulse && 'animate-pulse', className)} />
}
