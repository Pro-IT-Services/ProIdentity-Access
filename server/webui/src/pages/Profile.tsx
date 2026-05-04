import { useEffect, useRef, useState } from 'react'
import { api, type Passkey } from '../api/client'
import { useAuthStore } from '../stores/useAuthStore'
import { Shield, Fingerprint, Trash2, Plus, CheckCircle, KeyRound, Smartphone } from 'lucide-react'
import { startRegistration } from '@simplewebauthn/browser'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Separator } from '@/components/ui/separator'

export default function Profile() {
  const user = useAuthStore(s => s.user)
  const [passkeys, setPasskeys] = useState<Passkey[]>([])
  const [totpSetup, setTotpSetup] = useState<{ secret: string; uri: string } | null>(null)
  const [totpCode, setTotpCode] = useState('')
  const [totpEnabled, setTotpEnabled] = useState(user?.totp_enabled ?? false)
  const [pushAuthEnabled, setPushAuthEnabled] = useState(false)
  const [pushStep, setPushStep] = useState<'idle' | 'pending' | 'approved' | 'denied' | 'expired'>('idle')
  const pushPollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const [passkeyName, setPasskeyName] = useState('My Device')
  const [disableCode, setDisableCode] = useState('')
  const [pwForm, setPwForm] = useState({ current: '', next: '', confirm: '' })
  const [msg, setMsg] = useState('')
  const [error, setError] = useState('')

  useEffect(() => { api.listPasskeys().then(d => setPasskeys(d ?? [])) }, [])
  useEffect(() => { setTotpEnabled(user?.totp_enabled ?? false) }, [user])
  useEffect(() => { api.serverInfo().then(i => setPushAuthEnabled(i.push_auth_enabled)).catch(() => {}) }, [])
  useEffect(() => () => { if (pushPollRef.current) clearInterval(pushPollRef.current) }, [])

  const flash = (m: string) => { setMsg(m); setTimeout(() => setMsg(''), 3000) }
  const err = (e: string) => { setError(e); setTimeout(() => setError(''), 5000) }

  const startTOTP = async () => {
    try { setTotpSetup(await api.totpSetup()) } catch (e: any) { err(e.message) }
  }

  const confirmTOTP = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.totpConfirm(totpCode)
      if (pushAuthEnabled) {
        setPushStep('pending')
        try {
          const resp = await api.createPushAuth('2FA setup verification')
          pushPollRef.current = setInterval(async () => {
            try {
              const st = await api.pollPushStatus(resp.request_id)
              if (st.status === 'approved') {
                if (pushPollRef.current) clearInterval(pushPollRef.current)
                setPushStep('approved')
                setTotpEnabled(true); setTotpSetup(null); setTotpCode('')
                flash('2FA enabled with push verification!')
              } else if (st.status === 'denied' || st.status === 'expired') {
                if (pushPollRef.current) clearInterval(pushPollRef.current)
                setPushStep(st.status as any)
              }
            } catch { /* keep polling */ }
          }, 2000)
        } catch (e: any) {
          setTotpEnabled(true); setTotpSetup(null); setTotpCode('')
          setPushStep('idle')
          flash('2FA enabled (push verification failed: ' + (e.message || e) + ')')
        }
      } else {
        setTotpEnabled(true); setTotpSetup(null); setTotpCode('')
        flash('2FA enabled!')
      }
    } catch (e: any) { err(e.message) }
  }

  const disableTOTP = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.totpDisable(disableCode)
      setTotpEnabled(false); setDisableCode('')
      flash('2FA disabled')
    } catch (e: any) { err(e.message) }
  }

  const addPasskey = async () => {
    try {
      const options = await api.passkeyRegisterBegin()
      const attestation = await startRegistration(options as any)
      const result = await api.passkeyRegisterFinish(passkeyName, attestation)
      setPasskeys(prev => [...prev, { ...result, user_id: user?.id ?? '', created_at: new Date().toISOString() }])
      flash('Passkey registered!')
    } catch (e: any) { err(e.message) }
  }

  const deletePasskey = async (id: string) => {
    await api.deletePasskey(id)
    setPasskeys(prev => prev.filter(p => p.id !== id))
  }

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (pwForm.next !== pwForm.confirm) { err('New passwords do not match'); return }
    if (pwForm.next.length < 8) { err('Password must be at least 8 characters'); return }
    try {
      await api.changePassword(pwForm.current, pwForm.next)
      setPwForm({ current: '', next: '', confirm: '' })
      flash('Password changed!')
    } catch (e: any) { err(e.message) }
  }

  return (
    <div className="p-6 max-w-2xl mx-auto space-y-4">
      <div className="mb-2">
        <h1 className="text-2xl font-semibold">Profile & Security</h1>
        <p className="text-muted-foreground text-sm">{user?.username} · {user?.email}</p>
      </div>

      {msg && (
        <Alert variant="success">
          <CheckCircle className="w-4 h-4" />
          <AlertDescription>{msg}</AlertDescription>
        </Alert>
      )}
      {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}

      {/* Change Password */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center gap-3">
            <KeyRound className="w-5 h-5 text-primary" />
            <div>
              <CardTitle className="text-sm">Change Password</CardTitle>
              <CardDescription>Update your account password</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleChangePassword} className="space-y-3">
            <div className="space-y-1.5">
              <Label>Current Password</Label>
              <Input type="password" value={pwForm.current} onChange={e => setPwForm(p => ({ ...p, current: e.target.value }))} required />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>New Password</Label>
                <Input type="password" value={pwForm.next} onChange={e => setPwForm(p => ({ ...p, next: e.target.value }))} required />
              </div>
              <div className="space-y-1.5">
                <Label>Confirm Password</Label>
                <Input type="password" value={pwForm.confirm} onChange={e => setPwForm(p => ({ ...p, confirm: e.target.value }))} required />
              </div>
            </div>
            <Button type="submit" size="sm">Update Password</Button>
          </form>
        </CardContent>
      </Card>

      {/* TOTP */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center gap-3">
            <Shield className="w-5 h-5 text-primary" />
            <div className="flex-1">
              <CardTitle className="text-sm">Two-Factor Authentication</CardTitle>
              <CardDescription>TOTP via authenticator app</CardDescription>
            </div>
            <Badge variant={totpEnabled ? 'success' : 'muted'}>{totpEnabled ? 'Enabled' : 'Disabled'}</Badge>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {!totpEnabled && !totpSetup && (
            <Button size="sm" onClick={startTOTP}>
              <Shield /> Enable 2FA
            </Button>
          )}
          {totpSetup && pushStep === 'idle' && (
            <div className="space-y-3 bg-secondary/50 rounded-lg p-4">
              <p className="text-xs text-muted-foreground">Scan this QR code with your authenticator app, then enter the 6-digit code to confirm.</p>
              <div className="flex justify-center">
                <img
                  src={`https://api.qrserver.com/v1/create-qr-code/?size=180x180&data=${encodeURIComponent(totpSetup.uri)}`}
                  alt="TOTP QR Code"
                  className="rounded-lg border border-border"
                />
              </div>
              <p className="text-xs text-center text-muted-foreground font-mono">{totpSetup.secret}</p>
              <form onSubmit={confirmTOTP} className="flex gap-2">
                <Input className="text-center tracking-widest font-mono" value={totpCode}
                  onChange={e => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  placeholder="000000" maxLength={6} />
                <Button type="submit">Verify</Button>
                <Button type="button" variant="ghost" onClick={() => setTotpSetup(null)}>Cancel</Button>
              </form>
            </div>
          )}
          {pushStep !== 'idle' && pushStep !== 'approved' && (
            <div className="space-y-3 bg-secondary/50 rounded-lg p-4 text-center">
              <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-primary/10 border border-primary/20 mx-auto">
                {pushStep === 'pending'
                  ? <Smartphone className="w-6 h-6 text-primary animate-pulse" />
                  : <Smartphone className="w-6 h-6 text-destructive" />}
              </div>
              <p className="text-sm font-medium">
                {pushStep === 'pending' && 'Approve the push notification to complete setup…'}
                {pushStep === 'denied' && 'Push denied. 2FA was enabled but push verification failed.'}
                {pushStep === 'expired' && 'Push expired. 2FA was enabled but push verification failed.'}
              </p>
              <p className="text-xs text-muted-foreground">
                {pushStep === 'pending' && 'Check your phone for a notification from ProIdentity Access.'}
                {(pushStep === 'denied' || pushStep === 'expired') && 'You can still use your authenticator app for 2FA.'}
              </p>
              {(pushStep === 'denied' || pushStep === 'expired') && (
                <Button size="sm" variant="outline" onClick={() => setPushStep('idle')}>Done</Button>
              )}
            </div>
          )}
          {totpEnabled && (
            <form onSubmit={disableTOTP} className="flex gap-2 items-end">
              <div className="flex-1 space-y-1.5">
                <Label className="text-xs text-muted-foreground">Enter current code to disable</Label>
                <Input className="text-center tracking-widest font-mono" value={disableCode}
                  onChange={e => setDisableCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  placeholder="000000" maxLength={6} />
              </div>
              <Button type="submit" variant="destructive">Disable 2FA</Button>
            </form>
          )}
        </CardContent>
      </Card>

      {/* Passkeys */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center gap-3">
            <Fingerprint className="w-5 h-5 text-primary" />
            <div className="flex-1">
              <CardTitle className="text-sm">Passkeys</CardTitle>
              <CardDescription>Biometric or hardware key authentication</CardDescription>
            </div>
            <Badge variant="muted">{passkeys.length}</Badge>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          {passkeys.map((pk, i) => (
            <div key={pk.id}>
              {i > 0 && <Separator className="mb-3" />}
              <div className="flex items-center gap-3">
                <Fingerprint className="w-4 h-4 text-muted-foreground" />
                <div className="flex-1">
                  <p className="text-sm">{pk.name}</p>
                  <p className="text-xs text-muted-foreground">Added {new Date(pk.created_at).toLocaleDateString()}</p>
                </div>
                <Button variant="ghost" size="icon" onClick={() => deletePasskey(pk.id)} className="hover:text-destructive hover:bg-destructive/10">
                  <Trash2 />
                </Button>
              </div>
            </div>
          ))}
          <Separator />
          <div className="flex gap-2 items-end">
            <div className="flex-1 space-y-1.5">
              <Label className="text-xs text-muted-foreground">Device name</Label>
              <Input value={passkeyName} onChange={e => setPasskeyName(e.target.value)} placeholder="My MacBook" />
            </div>
            <Button onClick={addPasskey}>
              <Plus /> Add Passkey
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
