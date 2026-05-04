import { useEffect } from 'react'
import { X } from 'lucide-react'
import { cn } from '../../lib/cn'

interface SheetProps {
  open: boolean
  onClose: () => void
  title?: React.ReactNode
  description?: React.ReactNode
  side?: 'right' | 'left'
  /** Pixel width of the drawer panel. Default 420. Inline-styled to avoid
   *  Tailwind JIT class-scan misses with arbitrary values. */
  widthPx?: number
  children: React.ReactNode
  footer?: React.ReactNode
}

/**
 * Right-side drawer. Esc closes; clicking the overlay closes.
 * Uses explicit absolute positioning + explicit z-indexes so the panel can
 * never end up behind the overlay.
 */
export function Sheet({
  open, onClose, title, description, side = 'right', widthPx = 420, children, footer,
}: SheetProps) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50" aria-modal="true" role="dialog">
      {/* Overlay — absolute, lower z. */}
      <button
        type="button"
        aria-label="Close"
        onClick={onClose}
        className="absolute inset-0 z-0 bg-black/55 backdrop-blur-sm cursor-default no-drag animate-fade-in"
      />
      {/* Panel — absolute pinned to side, higher z so the overlay can't bury it. */}
      <div
        style={{ width: widthPx }}
        className={cn(
          'absolute z-10 top-0 bottom-0 bg-card border-border shadow-2xl flex flex-col no-drag',
          side === 'right' ? 'right-0 border-l animate-slide-in' : 'left-0 border-r',
        )}
      >
        <div className="flex items-start justify-between gap-3 px-5 py-4 border-b border-border">
          <div className="min-w-0">
            {title && <h2 className="text-base font-semibold leading-tight">{title}</h2>}
            {description && <p className="text-xs text-muted-foreground mt-0.5">{description}</p>}
          </div>
          <button
            onClick={onClose}
            aria-label="Close"
            className="p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors cursor-pointer"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>

        {footer && (
          <div className="px-5 py-3 border-t border-border flex items-center justify-end gap-2">{footer}</div>
        )}
      </div>
    </div>
  )
}
