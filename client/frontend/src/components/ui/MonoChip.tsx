import { useState } from 'react'
import { Copy, Check } from 'lucide-react'
import { cn } from '../../lib/cn'

interface MonoChipProps {
  value: string
  display?: string
  copy?: boolean
  truncate?: boolean
  bare?: boolean
  className?: string
}

/** Monospace value (IP, key, port…) with optional copy-to-clipboard. */
export function MonoChip({ value, display, copy = true, truncate, bare, className }: MonoChipProps) {
  const [copied, setCopied] = useState(false)
  const text = display ?? value

  const onCopy = async (e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 1200)
    } catch { /* clipboard might be blocked in webview */ }
  }

  if (bare) {
    return (
      <span className={cn('inline-flex items-center gap-1 font-mono text-xs', className)}>
        <span className={truncate ? 'truncate' : ''}>{text}</span>
        {copy && (
          <button onClick={onCopy} aria-label="Copy" className="text-muted-foreground/60 hover:text-foreground transition-colors cursor-pointer no-drag">
            {copied ? <Check className="w-3 h-3 text-success" /> : <Copy className="w-3 h-3" />}
          </button>
        )}
      </span>
    )
  }

  return (
    <span className={cn(
      'inline-flex items-center gap-1.5 px-2 py-0.5 rounded font-mono text-xs',
      'bg-secondary/60 text-foreground/90 border border-border',
      truncate && 'max-w-full',
      className,
    )}>
      <span className={truncate ? 'truncate' : ''}>{text}</span>
      {copy && (
        <button onClick={onCopy} aria-label="Copy" className="text-muted-foreground hover:text-foreground transition-colors cursor-pointer shrink-0 no-drag">
          {copied ? <Check className="w-3 h-3 text-success" /> : <Copy className="w-3 h-3" />}
        </button>
      )}
    </span>
  )
}
