import { useEffect, useState } from 'react'
import { AlertCircle, CheckCircle, Info, X, ShieldOff } from 'lucide-react'
import { cn } from '../../lib/cn'

export interface ToastData {
  id: string
  message: string
  type?: 'info' | 'success' | 'warning' | 'error' | 'revoked'
  duration?: number
}

const ICON = {
  info:     Info,
  success:  CheckCircle,
  warning:  AlertCircle,
  error:    AlertCircle,
  revoked:  ShieldOff,
} as const

const COLOR = {
  info:     'border-info/40 bg-info/10 text-foreground',
  success:  'border-success/40 bg-success/10 text-foreground',
  warning:  'border-warning/40 bg-warning/10 text-foreground',
  error:    'border-destructive/40 bg-destructive/10 text-foreground',
  revoked:  'border-destructive/40 bg-destructive/10 text-foreground',
} as const

const ICON_COLOR = {
  info:     'text-info',
  success:  'text-success',
  warning:  'text-warning',
  error:    'text-destructive',
  revoked:  'text-destructive',
} as const

function ToastItem({ toast, onDismiss }: { toast: ToastData; onDismiss: (id: string) => void }) {
  const type = toast.type ?? 'info'
  const Icon = ICON[type]

  useEffect(() => {
    if (toast.duration === 0) return
    const t = setTimeout(() => onDismiss(toast.id), toast.duration ?? 6000)
    return () => clearTimeout(t)
  }, [toast.id, toast.duration, onDismiss])

  return (
    <div className={cn(
      'flex items-start gap-3 px-4 py-3 rounded-lg border shadow-lg backdrop-blur-sm max-w-sm animate-slide-up',
      COLOR[type],
    )}>
      <Icon className={cn('w-5 h-5 shrink-0 mt-0.5', ICON_COLOR[type])} />
      <p className="text-sm flex-1">{toast.message}</p>
      <button
        onClick={() => onDismiss(toast.id)}
        className="p-0.5 rounded text-muted-foreground hover:text-foreground transition-colors cursor-pointer shrink-0"
      >
        <X className="w-3.5 h-3.5" />
      </button>
    </div>
  )
}

let _nextId = 0
let _pushFn: ((toast: Omit<ToastData, 'id'>) => void) | null = null

export function toast(message: string, type: ToastData['type'] = 'info', duration?: number) {
  _pushFn?.({ message, type, duration })
}

export function ToastContainer() {
  const [toasts, setToasts] = useState<ToastData[]>([])

  useEffect(() => {
    _pushFn = (t) => setToasts(prev => [...prev, { ...t, id: String(++_nextId) }])
    return () => { _pushFn = null }
  }, [])

  const dismiss = (id: string) => setToasts(prev => prev.filter(t => t.id !== id))

  if (toasts.length === 0) return null

  return (
    <div className="fixed bottom-4 right-4 z-[100] flex flex-col gap-2 pointer-events-auto no-drag">
      {toasts.map(t => <ToastItem key={t.id} toast={t} onDismiss={dismiss} />)}
    </div>
  )
}
