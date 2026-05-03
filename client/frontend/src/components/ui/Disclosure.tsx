import { useState } from 'react'
import { ChevronRight } from 'lucide-react'
import { cn } from '../../lib/cn'

interface DisclosureProps {
  title: React.ReactNode
  defaultOpen?: boolean
  badge?: React.ReactNode
  className?: string
  children: React.ReactNode
}

export function Disclosure({ title, defaultOpen = false, badge, className, children }: DisclosureProps) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div className={cn('rounded-lg border border-border bg-card/60', className)}>
      <button
        type="button"
        onClick={() => setOpen(o => !o)}
        className="w-full flex items-center gap-2 px-4 py-2.5 text-left cursor-pointer hover:bg-secondary/40 transition-colors rounded-lg"
      >
        <ChevronRight className={cn('w-4 h-4 text-muted-foreground transition-transform', open && 'rotate-90')} />
        <span className="flex-1 text-sm font-medium">{title}</span>
        {badge}
      </button>
      {open && <div className="px-4 pb-4 pt-1">{children}</div>}
    </div>
  )
}
