import { useEffect, useRef, useState } from 'react'
import { AlertCircle, Loader2, ShieldCheck, Smartphone, KeyRound } from 'lucide-react'
import { Sheet } from './ui/Sheet'
import { Button } from './ui/Button'
import { Input } from './ui/Input'
import { managedCreatePushAuth, managedPollPushAuth } from '../wailsbridge'

interface Props {
  open: boolean
  serverName?: string
  pushAuthEnabled?: boolean
  onCancel: () => void
  onSubmit: (code: string) => Promise<void>
  onPushApproved?: (requestId: string) => Promise<void>
}

export function TotpPromptSheet({ open, serverName, pushAuthEnabled, onCancel, onSubmit, onPushApproved }: Props) {
  const [mode, setMode] = useState<'push' | 'totp'>('totp')
  const [code, setCode] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [pushStatus, setPushStatus] = useState<string>('pending')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const onPushRef = useRef(onPushApproved)
  onPushRef.current = onPushApproved

  const stopPolling = () => {
    if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null }
  }

  useEffect(() => {
    if (!open) return
    setCode(''); setError(''); setLoading(false)
    setPushStatus('pending')
    stopPolling()
    setMode(pushAuthEnabled ? 'push' : 'totp')
  }, [open, pushAuthEnabled])

  useEffect(() => {
    if (!open || mode !== 'push') return
    let cancelled = false

    const startPush = async () => {
      try {
        const resp = await managedCreatePushAuth(serverName ? `Connect to ${serverName}` : 'VPN connection')
        if (cancelled) return
        setPushStatus('pending')

        pollRef.current = setInterval(async () => {
          try {
            const status = await managedPollPushAuth(resp.request_id)
            if (cancelled) return
            setPushStatus(status)
            if (status === 'approved') {
              stopPolling()
              setLoading(true)
              try {
                await onPushRef.current?.(resp.request_id)
              } catch (e: any) {
                setError(String(e?.message ?? e))
              } finally { setLoading(false) }
            } else if (status === 'denied' || status === 'expired') {
              stopPolling()
              setError(status === 'denied' ? 'Request denied.' : 'Request expired. Try again.')
            }
          } catch { /* keep polling */ }
        }, 2000)
      } catch (e: any) {
        if (!cancelled) setError(e?.message ?? 'Failed to create push request')
      }
    }
    startPush()
    return () => { cancelled = true; stopPolling() }
  }, [open, mode])

  const submitTotp = async () => {
    if (code.length !== 6) return
    setLoading(true); setError('')
    try {
      await onSubmit(code)
    } catch (e: any) {
      const msg = String(e?.message ?? e)
      setError(msg.includes('totp') || msg.includes('2FA') ? 'Invalid code, try again' : msg)
      setCode('')
    } finally { setLoading(false) }
  }

  return (
    <Sheet
      open={open}
      onClose={() => { if (!loading) { stopPolling(); onCancel() } }}
      title="Verification required"
      description={serverName ? `Connect to ${serverName}` : 'Verify your identity to continue.'}
      widthPx={400}
      footer={mode === 'totp' ? (
        <>
          <Button variant="ghost" onClick={() => { stopPolling(); onCancel() }} disabled={loading}>Cancel</Button>
          <Button onClick={submitTotp} disabled={loading || code.length !== 6}>
            {loading
              ? <><Loader2 className="w-3.5 h-3.5 animate-spin" /> Verifying…</>
              : <><ShieldCheck className="w-3.5 h-3.5" /> Connect</>}
          </Button>
        </>
      ) : (
        <Button variant="ghost" onClick={() => { stopPolling(); onCancel() }} disabled={loading}>Cancel</Button>
      )}
    >
      <div className="space-y-4">
        {error && (
          <div className="flex items-start gap-2 px-3 py-2.5 bg-destructive/10 border border-destructive/30 rounded-md">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5 text-destructive" />
            <span className="text-sm text-destructive break-words">{error}</span>
          </div>
        )}

        {mode === 'push' ? (
          <div className="text-center py-6 space-y-4">
            <div className="w-16 h-16 rounded-2xl bg-primary/10 border border-primary/20 flex items-center justify-center mx-auto">
              {pushStatus === 'approved'
                ? <ShieldCheck className="w-8 h-8 text-success" />
                : <Smartphone className="w-8 h-8 text-primary animate-pulse" />}
            </div>
            <div>
              <p className="text-sm font-medium">
                {pushStatus === 'pending' && 'Waiting for approval…'}
                {pushStatus === 'approved' && 'Approved — connecting…'}
                {pushStatus === 'denied' && 'Denied'}
                {pushStatus === 'expired' && 'Expired'}
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                {pushStatus === 'pending' && 'Check your phone for a push notification from ProIdentity Access.'}
                {pushStatus === 'denied' && 'The request was denied. Try again or use a code.'}
                {pushStatus === 'expired' && 'The request expired. Try again or use a code.'}
              </p>
            </div>
            {(pushStatus === 'pending' || pushStatus === 'denied' || pushStatus === 'expired') && (
              <button
                onClick={() => { stopPolling(); setError(''); setMode('totp') }}
                className="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
              >
                <KeyRound className="w-3.5 h-3.5" /> Enter code manually
              </button>
            )}
          </div>
        ) : (
          <div className="space-y-3">
            <Input
              value={code}
              onChange={e => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
              onKeyDown={e => { if (e.key === 'Enter' && code.length === 6) submitTotp() }}
              placeholder="000000"
              autoFocus
              inputMode="numeric"
              maxLength={6}
              disabled={loading}
              className="text-center font-mono tracking-[0.45em] text-lg h-12"
            />
            <p className="text-xs text-muted-foreground text-center">
              Enter the 6-digit code from your authenticator app.
            </p>
            {pushAuthEnabled && (
              <button
                onClick={() => { setError(''); setCode(''); setMode('push') }}
                className="w-full inline-flex items-center justify-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer mt-2"
              >
                <Smartphone className="w-3.5 h-3.5" /> Use push notification instead
              </button>
            )}
          </div>
        )}
      </div>
    </Sheet>
  )
}
