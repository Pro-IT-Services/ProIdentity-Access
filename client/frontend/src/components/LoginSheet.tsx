import { useEffect, useRef, useState } from 'react'
import { AlertCircle, Eye, EyeOff, Loader2, LogIn, Smartphone, KeyRound } from 'lucide-react'
import { useManagedStore } from '../stores/useManagedStore'
import { managedPollPushAuth } from '../wailsbridge'
import { Sheet } from './ui/Sheet'
import { Button } from './ui/Button'
import { Input } from './ui/Input'

interface Props {
  open: boolean
  onClose: () => void
  onLoggedIn?: () => void
}

export function LoginSheet({ open, onClose, onLoggedIn }: Props) {
  const { settings, login, loginWithPush, loading, error, clearError } = useManagedStore()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [totp, setTotp] = useState('')
  const [needTotp, setNeedTotp] = useState(false)
  const [showPw, setShowPw] = useState(false)

  const [pushEnabled, setPushEnabled] = useState(false)
  const [pushRequestId, setPushRequestId] = useState<string | null>(null)
  const [pushStatus, setPushStatus] = useState('idle')
  const [mode, setMode] = useState<'totp' | 'push'>('totp')
  const [pushLoading, setPushLoading] = useState(false)
  const [pushError, setPushError] = useState('')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const usernameRef = useRef(username)
  const passwordRef = useRef(password)
  usernameRef.current = username
  passwordRef.current = password

  const stopPolling = () => {
    if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null }
  }

  useEffect(() => {
    if (!open) return
    setUsername(''); setPassword(''); setTotp(''); setNeedTotp(false); setShowPw(false)
    setPushEnabled(false); setPushRequestId(null); setPushStatus('idle'); setMode('totp')
    setPushLoading(false); setPushError('')
    stopPolling(); clearError()
  }, [open, clearError])

  useEffect(() => () => stopPolling(), [])

  // When push approved, complete login.
  useEffect(() => {
    if (pushStatus !== 'approved' || !pushRequestId) return
    let cancelled = false
    setPushLoading(true)
    loginWithPush(usernameRef.current, passwordRef.current, pushRequestId)
      .then(() => {
        if (cancelled) return
        onLoggedIn?.()
        onClose()
      })
      .catch(e => { if (!cancelled) setPushError(String(e?.message ?? e)) })
      .finally(() => { if (!cancelled) setPushLoading(false) })
    return () => { cancelled = true }
  }, [pushStatus, pushRequestId])

  const startPolling = (reqId: string) => {
    stopPolling()
    setPushStatus('pending')
    pollRef.current = setInterval(async () => {
      try {
        const status = await managedPollPushAuth(reqId)
        if (status === 'approved' || status === 'denied' || status === 'expired') {
          stopPolling()
          setPushStatus(status)
        }
      } catch { /* keep polling */ }
    }, 2000)
  }

  const handleInitialLogin = async (e?: React.FormEvent) => {
    e?.preventDefault()
    clearError()
    try {
      const r = await login(username.trim(), password, needTotp && mode === 'totp' ? totp : '')
      if (r.requireTOTP) {
        setNeedTotp(true)
        if (r.pushAuthEnabled && r.pushRequestId) {
          setPushEnabled(true)
          setPushRequestId(r.pushRequestId)
          setMode('push')
          startPolling(r.pushRequestId)
        }
        return
      }
      onLoggedIn?.()
      onClose()
    } catch {
      // store sets error
    }
  }

  const handleTotpSubmit = async (e?: React.FormEvent) => {
    e?.preventDefault()
    clearError()
    try {
      const r = await login(username.trim(), password, totp)
      if (!r.requireTOTP) {
        onLoggedIn?.()
        onClose()
      }
    } catch {
      // store sets error
    }
  }

  const retryPush = async () => {
    clearError(); setPushError('')
    try {
      const r = await login(username.trim(), password, '')
      if (r.pushRequestId) {
        setPushRequestId(r.pushRequestId)
        startPolling(r.pushRequestId)
      }
    } catch { /* error in store */ }
  }

  const displayError = error || pushError

  return (
    <Sheet
      open={open}
      onClose={() => { if (!loading && !pushLoading) { stopPolling(); onClose() } }}
      title="Sign in"
      description={settings.server_url || 'Sign in to your managed server.'}
      footer={needTotp && mode === 'push' ? (
        <Button variant="ghost" onClick={() => { stopPolling(); onClose() }} disabled={loading || pushLoading}>Cancel</Button>
      ) : (
        <>
          <Button variant="ghost" onClick={() => { stopPolling(); onClose() }} disabled={loading}>Cancel</Button>
          <Button onClick={() => needTotp ? handleTotpSubmit() : handleInitialLogin()} disabled={loading || (!needTotp ? !username || !password : totp.length !== 6)}>
            {loading
              ? <><Loader2 className="w-3.5 h-3.5 animate-spin" /> Signing in…</>
              : needTotp
                ? <><LogIn className="w-3.5 h-3.5" /> Verify</>
                : <><LogIn className="w-3.5 h-3.5" /> Sign in</>}
          </Button>
        </>
      )}
    >
      <div className="space-y-4">
        {displayError && (
          <div className="flex items-start gap-2 px-3 py-2.5 bg-destructive/10 border border-destructive/30 rounded-md">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5 text-destructive" />
            <span className="text-sm text-destructive break-words">{displayError}</span>
          </div>
        )}

        {needTotp && mode === 'push' ? (
          <div className="text-center py-6 space-y-4">
            <div className="w-16 h-16 rounded-2xl bg-primary/10 border border-primary/20 flex items-center justify-center mx-auto">
              {pushStatus === 'approved' || pushLoading
                ? <Loader2 className="w-8 h-8 text-primary animate-spin" />
                : <Smartphone className="w-8 h-8 text-primary animate-pulse" />}
            </div>
            <div>
              <p className="text-sm font-medium">
                {pushStatus === 'pending' && 'Waiting for approval…'}
                {pushStatus === 'approved' && 'Approved — signing in…'}
                {pushStatus === 'denied' && 'Denied'}
                {pushStatus === 'expired' && 'Expired'}
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                {pushStatus === 'pending' && 'Check your phone for a push notification.'}
                {pushStatus === 'denied' && 'The request was denied.'}
                {pushStatus === 'expired' && 'The request expired.'}
              </p>
            </div>
            {(pushStatus === 'denied' || pushStatus === 'expired') && (
              <Button variant="outline" size="sm" onClick={retryPush} disabled={loading}>Try again</Button>
            )}
            {pushStatus !== 'approved' && (
              <button
                onClick={() => { stopPolling(); setPushError(''); setMode('totp') }}
                className="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
              >
                <KeyRound className="w-3.5 h-3.5" /> Enter code manually
              </button>
            )}
          </div>
        ) : !needTotp ? (
          <form onSubmit={handleInitialLogin} className="space-y-4">
            <div className="space-y-1.5">
              <label className="block text-xs text-muted-foreground">Username</label>
              <Input value={username} onChange={e => setUsername(e.target.value)} placeholder="admin" autoFocus disabled={loading} autoComplete="username" />
            </div>
            <div className="space-y-1.5">
              <label className="block text-xs text-muted-foreground">Password</label>
              <div className="relative">
                <Input type={showPw ? 'text' : 'password'} value={password} onChange={e => setPassword(e.target.value)} placeholder="••••••••" disabled={loading} autoComplete="current-password" className="pr-9" />
                <button type="button" onClick={() => setShowPw(v => !v)} className="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-muted-foreground hover:text-foreground transition-colors cursor-pointer" tabIndex={-1}>
                  {showPw ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>
          </form>
        ) : (
          <div className="space-y-3">
            <div className="space-y-1.5">
              <label className="block text-xs text-muted-foreground">Two-factor code</label>
              <Input
                value={totp}
                onChange={e => setTotp(e.target.value.replace(/\D/g, '').slice(0, 6))}
                onKeyDown={e => { if (e.key === 'Enter' && totp.length === 6) handleTotpSubmit() }}
                placeholder="000000" autoFocus disabled={loading} inputMode="numeric" maxLength={6}
                className="text-center font-mono tracking-[0.4em] text-base"
              />
              <p className="text-xs text-muted-foreground">Enter the 6-digit code from your authenticator app.</p>
            </div>
            {pushEnabled && (
              <button
                onClick={() => { setPushError(''); setTotp(''); setMode('push'); retryPush() }}
                className="w-full inline-flex items-center justify-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
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
