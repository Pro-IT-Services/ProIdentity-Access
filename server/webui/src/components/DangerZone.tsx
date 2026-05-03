import { AlertTriangle } from 'lucide-react'
import { cn } from '@/lib/utils'

interface DangerZoneProps {
  title?: string
  className?: string
  children: React.ReactNode
}

export function DangerZone({ title = 'Danger zone', className, children }: DangerZoneProps) {
  return (
    <div className={cn('rounded-lg border border-destructive/40 bg-destructive/5', className)}>
      <div className="px-4 py-3 border-b border-destructive/30 flex items-center gap-2">
        <AlertTriangle className="w-4 h-4 text-destructive" />
        <p className="text-sm font-semibold text-destructive">{title}</p>
      </div>
      <div className="p-4 space-y-3">{children}</div>
    </div>
  )
}

interface DangerActionProps {
  title: string
  description: string
  action: React.ReactNode
}

export function DangerAction({ title, description, action }: DangerActionProps) {
  return (
    <div className="flex items-start justify-between gap-4 py-2">
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium">{title}</p>
        <p className="text-xs text-muted-foreground mt-0.5">{description}</p>
      </div>
      <div className="shrink-0">{action}</div>
    </div>
  )
}
