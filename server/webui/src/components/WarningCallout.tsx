import { AlertTriangle, AlertCircle, Info, CheckCircle2 } from 'lucide-react'
import { cn } from '@/lib/utils'

type Tone = 'warn' | 'error' | 'info' | 'success'

const TONE: Record<Tone, { icon: React.ElementType; ring: string; text: string; bg: string; iconColor: string }> = {
  warn:    { icon: AlertTriangle, ring: 'ring-warning/30', text: 'text-warning', bg: 'bg-warning/5', iconColor: 'text-warning' },
  error:   { icon: AlertCircle,   ring: 'ring-destructive/30', text: 'text-destructive', bg: 'bg-destructive/5', iconColor: 'text-destructive' },
  info:    { icon: Info,          ring: 'ring-primary/30', text: 'text-primary', bg: 'bg-primary/5', iconColor: 'text-primary' },
  success: { icon: CheckCircle2,  ring: 'ring-success/30', text: 'text-success', bg: 'bg-success/5', iconColor: 'text-success' },
}

interface WarningCalloutProps {
  tone?: Tone
  title: string
  description?: string
  action?: React.ReactNode
  className?: string
}

export function WarningCallout({ tone = 'warn', title, description, action, className }: WarningCalloutProps) {
  const t = TONE[tone]
  const Icon = t.icon
  return (
    <div className={cn('flex gap-3 p-3 rounded-lg ring-1', t.ring, t.bg, className)}>
      <Icon className={cn('w-4 h-4 mt-0.5 shrink-0', t.iconColor)} />
      <div className="flex-1 min-w-0">
        <p className={cn('text-sm font-medium', t.text)}>{title}</p>
        {description && <p className="text-xs text-foreground/80 mt-0.5">{description}</p>}
        {action && <div className="mt-2">{action}</div>}
      </div>
    </div>
  )
}
