import { X } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ChipProps {
  label: string
  hint?: string
  onRemove?: () => void
  tone?: 'neutral' | 'primary' | 'success' | 'warning'
  className?: string
}

const TONE = {
  neutral: 'bg-secondary text-foreground/85 border-border',
  primary: 'bg-primary/10 text-primary border-primary/25',
  success: 'bg-success/10 text-success border-success/25',
  warning: 'bg-warning/10 text-warning border-warning/25',
}

export function Chip({ label, hint, onRemove, tone = 'neutral', className }: ChipProps) {
  return (
    <span className={cn(
      'inline-flex items-center gap-1.5 pl-2 pr-1.5 py-0.5 rounded-md text-xs font-medium border',
      TONE[tone],
      className,
    )}>
      <span>{label}</span>
      {hint && <span className="font-mono text-[10px] opacity-70">{hint}</span>}
      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          aria-label={`Remove ${label}`}
          className="ml-0.5 p-0.5 rounded hover:bg-foreground/10 transition-colors cursor-pointer"
        >
          <X className="w-3 h-3" />
        </button>
      )}
    </span>
  )
}
