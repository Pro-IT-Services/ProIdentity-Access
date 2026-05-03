import { useEffect, useRef, useState } from 'react'
import { Search } from 'lucide-react'
import { cn } from '@/lib/utils'

export interface ComboboxOption {
  value: string
  label: string
  hint?: string
}

interface ComboboxProps {
  options: ComboboxOption[]
  onSelect: (value: string) => void
  placeholder?: React.ReactNode
  emptyText?: string
  className?: string
  buttonClassName?: string
}

/** Simple typeahead — opens a popover, filters as you type, selects on click. */
export function Combobox({
  options, onSelect, placeholder = 'Add…', emptyText = 'No matches',
  className, buttonClassName,
}: ComboboxProps) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [highlight, setHighlight] = useState(0)
  const wrapRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const filtered = options.filter(o =>
    o.label.toLowerCase().includes(query.toLowerCase()) ||
    (o.hint?.toLowerCase().includes(query.toLowerCase()) ?? false)
  )

  useEffect(() => {
    if (!open) return
    const onClick = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onClick)
    return () => document.removeEventListener('mousedown', onClick)
  }, [open])

  useEffect(() => { if (open) setTimeout(() => inputRef.current?.focus(), 10) }, [open])
  useEffect(() => { setHighlight(0) }, [query])

  const choose = (value: string) => {
    onSelect(value)
    setQuery('')
    setOpen(false)
  }

  return (
    <div ref={wrapRef} className={cn('relative inline-block', className)}>
      <button
        type="button"
        onClick={() => setOpen(o => !o)}
        className={cn(
          'inline-flex items-center gap-2 px-3 h-9 rounded-md border border-dashed border-border',
          'text-xs text-muted-foreground hover:border-primary/50 hover:text-foreground transition-colors cursor-pointer',
          buttonClassName
        )}
      >
        <Search className="w-3.5 h-3.5" /> {placeholder}
      </button>

      {open && (
        <div className="absolute z-40 mt-1 w-72 rounded-md border border-border bg-popover shadow-xl overflow-hidden">
          <div className="px-3 py-2 border-b border-border">
            <input
              ref={inputRef}
              value={query}
              onChange={e => setQuery(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'ArrowDown') { e.preventDefault(); setHighlight(h => Math.min(h + 1, filtered.length - 1)) }
                else if (e.key === 'ArrowUp') { e.preventDefault(); setHighlight(h => Math.max(h - 1, 0)) }
                else if (e.key === 'Enter' && filtered[highlight]) { e.preventDefault(); choose(filtered[highlight].value) }
                else if (e.key === 'Escape') setOpen(false)
              }}
              placeholder="Search…"
              className="w-full bg-transparent text-sm text-foreground placeholder:text-muted-foreground outline-none"
            />
          </div>
          <div className="max-h-64 overflow-y-auto scrollbar-thin">
            {filtered.length === 0 ? (
              <p className="px-3 py-3 text-xs text-muted-foreground text-center">{emptyText}</p>
            ) : (
              filtered.map((opt, i) => (
                <button
                  key={opt.value}
                  type="button"
                  onClick={() => choose(opt.value)}
                  onMouseEnter={() => setHighlight(i)}
                  className={cn(
                    'w-full text-left px-3 py-2 text-sm flex items-baseline gap-2 cursor-pointer transition-colors',
                    i === highlight ? 'bg-primary/10 text-foreground' : 'text-foreground/85 hover:bg-secondary'
                  )}
                >
                  <span className="truncate">{opt.label}</span>
                  {opt.hint && <span className="text-[11px] text-muted-foreground font-mono truncate">{opt.hint}</span>}
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
