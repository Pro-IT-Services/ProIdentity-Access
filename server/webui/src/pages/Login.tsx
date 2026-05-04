import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Fingerprint, Loader2, Lock, Smartphone, KeyRound } from 'lucide-react'
import { useAuthStore } from '../stores/useAuthStore'
import { api } from '../api/client'
import { startAuthentication } from '@simplewebauthn/browser'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

export default function Login() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [totpCode, setTotpCode] = useState('')
  const [needTotp, setNeedTotp] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const { setAuth } = useAuthStore()
  const navigate = useNavigate()

  const [pushEnabled, setPushEnabled] = useState(false)
  const [pushRequestId, setPushRequestId] = useState<string | null>(null)
  const [pushStatus, setPushStatus] = useState('idle')
  const [mode, setMode] = useState<'totp' | 'push'>('totp')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const usernameRef = useRef(username)
  const passwordRef = useRef(password)
  usernameRef.current = username
  passwordRef.current = password

  const stopPolling = () => {
    if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null }
  }
  useEffect(() => () => stopPolling(), [])

  const completeLogin = async (token: string) => {
    sessionStorage.setItem('wg_token', token)
    const user = await api.me()
    setAuth(token, user)
    navigate('/')
  }

  const startPolling = (reqId: string) => {
    stopPolling()
    setPushStatus('pending')
    pollRef.current = setInterval(async () => {
      try {
        const res = await api.pollPushStatus(reqId)
        if (res.status === 'approved' || res.status === 'denied' || res.status === 'expired') {
          stopPolling()
          setPushStatus(res.status)
        }
      } catch { /* keep polling */ }
    }, 2000)
  }

  // When push is approved, complete the login.
  useEffect(() => {
    if (pushStatus !== 'approved' || !pushRequestId) return
    let cancelled = false
    setLoading(true)
    api.login(usernameRef.current, passwordRef.current, undefined, pushRequestId)
      .then(res => {
        if (cancelled) return
        if (res.token) completeLogin(res.token)
        else setError('Login failed after push approval')
      })
      .catch(e => { if (!cancelled) setError(e.message) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [pushStatus, pushRequestId])

  // Initial login — sends username+password, server responds with what 2FA is needed.
  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(''); setLoading(true)
    try {
      const res = await api.login(username, password, needTotp && mode === 'totp' ? totpCode : undefined)
      if (res.require_totp) {
        setNeedTotp(true)
        if (res.push_auth_enabled && res.push_request_id) {
          setPushEnabled(true)
          setPushRequestId(res.push_request_id)
          setMode('push')
          startPolling(res.push_request_id)
        }
        setLoading(false)
        return
      }
      await completeLogin(res.token)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  // TOTP code submit — separate from initial login so it doesn't create a new push.
  const handleTotpSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(''); setLoading(true)
    try {
      const res = await api.login(username, password, totpCode)
      if (res.token) await completeLogin(res.token)
      else setError('Verification failed')
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  // Retry push — re-sends login to get a fresh push request.
  const retryPush = async () => {
    setError(''); setLoading(true)
    try {
      const res = await api.login(username, password)
      if (res.push_request_id) {
        setPushRequestId(res.push_request_id)
        startPolling(res.push_request_id)
      }
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  const handlePasskey = async () => {
    if (!username) { setError('Enter username first'); return }
    setError(''); setLoading(true)
    try {
      const options = await api.passkeyLoginBegin(username)
      const assertion = await startAuthentication(options as any)
      const res = await api.passkeyLoginFinish(username, assertion)
      await completeLogin(res.token)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-4">
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        <div className="absolute top-1/4 left-1/2 -translate-x-1/2 w-96 h-96 bg-primary/5 rounded-full blur-3xl" />
      </div>

      <div className="relative w-full max-w-sm">
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-primary/10 border border-primary/20 mb-4 text-primary">
            <svg width="28" height="28" viewBox="0 0 256 256" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round">
              <polygon points="178,128 153,171.3 103,171.3 78,128 103,84.7 153,84.7" strokeWidth="8"/>
              <line x1="178" y1="128" x2="192.2" y2="199.3" strokeWidth="6"/>
              <line x1="153" y1="171.3" x2="98.3" y2="219.3" strokeWidth="6"/>
              <line x1="103" y1="171.3" x2="34.1" y2="148" strokeWidth="6"/>
              <line x1="78" y1="128" x2="63.8" y2="56.7" strokeWidth="6"/>
              <line x1="103" y1="84.7" x2="157.7" y2="36.7" strokeWidth="6"/>
              <line x1="153" y1="84.7" x2="221.9" y2="108" strokeWidth="6"/>
              <circle cx="128" cy="128" r="11" fill="currentColor"/>
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-foreground">ProIdentity Access</h1>
          <p className="text-sm text-muted-foreground mt-1">Secure VPN control panel</p>
        </div>

        <div className="bg-card border border-border rounded-xl p-6 shadow-xl space-y-5">
          {needTotp && (
            <div className="flex items-center gap-2 text-sm font-medium text-foreground pb-1 border-b border-border">
              <Lock className="w-4 h-4 text-primary" />
              {mode === 'push' ? 'Push Verification' : 'Two-Factor Authentication'}
            </div>
          )}

          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          {/* Push mode — primary when push auth is enabled */}
          {needTotp && mode === 'push' ? (
            <div className="text-center py-4 space-y-4">
              <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-primary/10 border border-primary/20">
                {pushStatus === 'approved' || loading
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
                <Button variant="outline" className="w-full" onClick={retryPush} disabled={loading}>
                  Try again
                </Button>
              )}
              {pushStatus !== 'approved' && (
                <button
                  onClick={() => { stopPolling(); setMode('totp'); setError('') }}
                  className="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                >
                  <KeyRound className="w-3.5 h-3.5" /> Enter code manually
                </button>
              )}
            </div>

          /* Username + password (initial) or TOTP code (fallback) */
          ) : (
            <form onSubmit={needTotp ? handleTotpSubmit : handleLogin} className="space-y-4">
              {!needTotp && (
                <>
                  <div className="space-y-1.5">
                    <Label htmlFor="username" className="text-xs text-muted-foreground">Username</Label>
                    <Input id="username" value={username} onChange={e => setUsername(e.target.value)}
                      placeholder="admin" autoComplete="username"
                      className="bg-secondary/50 border-border focus-visible:ring-primary/50" required />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="password" className="text-xs text-muted-foreground">Password</Label>
                    <Input id="password" type="password" value={password} onChange={e => setPassword(e.target.value)}
                      placeholder="••••••••" autoComplete="current-password"
                      className="bg-secondary/50 border-border focus-visible:ring-primary/50" required />
                  </div>
                </>
              )}

              {needTotp && mode === 'totp' && (
                <div className="space-y-1.5">
                  <Label className="text-xs text-muted-foreground">6-digit code from your authenticator app</Label>
                  <Input
                    className="text-center tracking-[0.5em] text-xl font-mono bg-secondary/50 border-border h-12"
                    value={totpCode} onChange={e => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                    placeholder="000000" maxLength={6} autoComplete="one-time-code" autoFocus />
                </div>
              )}

              <Button type="submit" className="w-full font-semibold" disabled={loading}>
                {loading ? <Loader2 className="animate-spin" /> : null}
                {needTotp ? 'Verify Code' : 'Sign In'}
              </Button>

              {needTotp && pushEnabled && (
                <button type="button" onClick={() => { setError(''); setTotpCode(''); setMode('push'); retryPush() }}
                  className="w-full inline-flex items-center justify-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer">
                  <Smartphone className="w-3.5 h-3.5" /> Use push notification instead
                </button>
              )}
            </form>
          )}

          {!needTotp && (
            <>
              <div className="flex items-center gap-3">
                <div className="h-px flex-1 bg-border" />
                <span className="text-xs text-muted-foreground">or continue with</span>
                <div className="h-px flex-1 bg-border" />
              </div>
              <Button variant="outline" className="w-full border-border text-muted-foreground hover:text-foreground"
                onClick={handlePasskey} disabled={loading}>
                <Fingerprint className="text-primary" /> Passkey
              </Button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
