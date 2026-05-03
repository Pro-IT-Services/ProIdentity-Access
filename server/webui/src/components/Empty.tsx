import { cn } from '@/lib/utils'

interface EmptyProps {
  icon?: React.ElementType
  title: string
  hint?: string
  action?: React.ReactNode
  className?: string
}

export function Empty({ icon: Icon, title, hint, action, className }: EmptyProps) {
  return (
    <div className={cn(
      'flex flex-col items-center justify-center text-center px-6 py-12 rounded-xl border border-dashed border-border bg-card/30',
      className,
    )}>
      {Icon && (
        <div className="w-12 h-12 rounded-xl bg-secondary/60 flex items-center justify-center mb-3">
          <Icon className="w-5 h-5 text-muted-foreground" />
        </div>
      )}
      <p className="text-sm font-medium text-foreground">{title}</p>
      {hint && <p className="text-xs text-muted-foreground mt-1 max-w-sm">{hint}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}
