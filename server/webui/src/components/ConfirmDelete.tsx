import { useState } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface ConfirmDeleteProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: React.ReactNode
  /** User must type this exact string to enable the confirm button. */
  confirmText: string
  /** Label for the destructive button (default: "Delete"). */
  actionLabel?: string
  onConfirm: () => Promise<void> | void
}

export function ConfirmDelete({
  open, onOpenChange, title, description, confirmText, actionLabel = 'Delete', onConfirm,
}: ConfirmDeleteProps) {
  const [typed, setTyped] = useState('')
  const [busy, setBusy] = useState(false)
  const matches = typed === confirmText

  const handleConfirm = async () => {
    if (!matches) return
    setBusy(true)
    try {
      await onConfirm()
      onOpenChange(false)
      setTyped('')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!busy) { onOpenChange(v); if (!v) setTyped('') } }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="text-destructive">{title}</DialogTitle>
        </DialogHeader>
        <div className="text-sm text-foreground/85 space-y-3">{description}</div>
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">
            Type <span className="font-mono text-foreground">{confirmText}</span> to confirm
          </Label>
          <Input
            value={typed}
            onChange={e => setTyped(e.target.value)}
            autoFocus
            placeholder={confirmText}
            className="font-mono"
          />
        </div>
        <DialogFooter>
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Cancel
          </Button>
          <Button
            type="button"
            variant="destructive"
            onClick={handleConfirm}
            disabled={!matches || busy}
          >
            {busy ? 'Working…' : actionLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
