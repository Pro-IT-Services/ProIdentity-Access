import { useEffect, useState } from 'react'
import { Shield, Server, Monitor, LogIn, Eye, EyeOff, ChevronRight, Wifi, AlertCircle, X } from 'lucide-react'
import { useSetupStore } from '../stores/useSetupStore'
import { useManagedStore } from '../stores/useManagedStore'

export default function SetupWizard({ onClose }: { onClose?: () => void }) {
  const {
    step, mode, serverURL, deviceName, loading, error,
    chooseMode, saveServerURL, persistServerURL, setDeviceName, ensureDeviceName, registerDevice, completeSetup, clearError,
  } = useSetupStore()
  const { login } = useManagedStore()

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-bg-base">
      <div className="w-full max-w-md mx-4">
        {/* Header */}
        <div className="text-center mb-8 relative">
          {onClose && (
            <button
              onClick={onClose}
              className="absolute right-0 top-0 p-1.5 rounded-md text-text-muted hover:text-text-primary hover:bg-bg-border transition-colors"
              title="Close"
            >
              <X className="w-4 h-4" />
            </button>
          )}
          <div className="w-14 h-14 rounded-2xl bg-accent/10 border border-accent/20 flex items-center justify-center mx-auto mb-4">
            <Shield className="w-7 h-7 text-accent" />
          </div>
          <h1 className="text-xl font-bold text-text-primary">ProIdentity</h1>
          <p className="text-sm text-text-secondary mt-1">Set up your VPN experience</p>
        </div>

        {/* Step card */}
        <div className="bg-bg-surface border border-bg-border rounded-2xl shadow-xl overflow-hidden">
          {step === 'mode' && <StepMode onChoose={chooseMode} loading={loading} error={error} onClearError={clearError} />}
          {step === 'server' && (
            <StepServer
              serverURL={serverURL}
              onURLChange={saveServerURL}
              onNext={async () => {
                await persistServerURL()
                useSetupStore.setState({ step: 'register' })
              }}
              error={error}
              onClearError={clearError}
            />
          )}
          {step === 'register' && (
            <StepRegister
              deviceName={deviceName}
              onNameChange={setDeviceName}
              onLoadDefaultName={ensureDeviceName}
              onRegister={registerDevice}
              loading={loading}
              error={error}
              onClearError={clearError}
            />
          )}
          {step === 'login' && (
            <StepLogin
              login={login}
              onComplete={completeSetup}
              loading={loading}
              error={error}
              onClearError={clearError}
            />
          )}
        </div>

        {/* Step indicator */}
        {mode === 'managed' && (
          <div className="flex items-center justify-center gap-2 mt-6">
            {(['server', 'register', 'login'] as const).map((s, i) => (
              <div key={s} className="flex items-center gap-2">
                <div className={`w-2 h-2 rounded-full transition-colors ${
                  step === s ? 'bg-accent' :
                  ['server', 'register', 'login'].indexOf(step) > i ? 'bg-accent/40' : 'bg-bg-border'
                }`} />
                {i < 2 && <div className="w-6 h-px bg-bg-border" />}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

// --- Step components ---

function StepMode({ onChoose, loading, error, onClearError }: {
  onChoose: (mode: 'standalone' | 'managed') => Promise<void>
  loading: boolean
  error: string | null
  onClearError: () => void
}) {
  return (
    <div className="p-6">
      <h2 className="text-base font-semibold text-text-primary mb-1">Choose your setup</h2>
      <p className="text-sm text-text-secondary mb-5">How would you like to use this app?</p>

      {error && <ErrorBox message={error} onDismiss={onClearError} />}

      <div className="space-y-3">
        <ModeCard
          icon={<Wifi className="w-5 h-5 text-accent" />}
          title="Standalone"
          description="Manage WireGuard configs manually. Import .conf files and connect."
          onClick={() => onChoose('standalone')}
          disabled={loading}
        />
        <ModeCard
          icon={<Server className="w-5 h-5 text-accent" />}
          title="Managed"
          description="Connect to a WG Manager server to automatically sync and manage configs."
          onClick={() => onChoose('managed')}
          disabled={loading}
        />
      </div>
    </div>
  )
}

function ModeCard({ icon, title, description, onClick, disabled }: {
  icon: React.ReactNode
  title: string
  description: string
  onClick: () => void
  disabled: boolean
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className="w-full flex items-start gap-3 p-4 bg-bg-base hover:bg-bg-border border border-bg-border rounded-xl text-left transition-colors group disabled:opacity-50 disabled:cursor-not-allowed"
    >
      <div className="w-9 h-9 rounded-lg bg-accent/10 border border-accent/20 flex items-center justify-center shrink-0 mt-0.5">
        {icon}
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-text-primary">{title}</p>
        <p className="text-xs text-text-secondary mt-0.5 leading-relaxed">{description}</p>
      </div>
      <ChevronRight className="w-4 h-4 text-text-muted group-hover:text-text-secondary mt-1 shrink-0 transition-colors" />
    </button>
  )
}

function StepServer({ serverURL, onURLChange, onNext, error, onClearError }: {
  serverURL: string
  onURLChange: (url: string) => void
  onNext: () => Promise<void>
  error: string | null
  onClearError: () => void
}) {
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!serverURL.trim()) return
    onClearError()
    await onNext()
  }

  return (
    <form onSubmit={handleSubmit} className="p-6">
      <div className="flex items-center gap-2.5 mb-4">
        <Server className="w-5 h-5 text-accent" />
        <h2 className="text-base font-semibold text-text-primary">Server URL</h2>
      </div>

      {error && <ErrorBox message={error} onDismiss={onClearError} />}

      <div className="mb-5">
        <label className="block text-xs font-medium text-text-secondary mb-1.5">Management server URL</label>
        <input
          className="w-full px-3 py-2 bg-bg-base border border-bg-border rounded-lg text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors"
          placeholder="https://vpn.example.com"
          value={serverURL}
          onChange={e => onURLChange(e.target.value)}
          required
          autoFocus
          type="url"
        />
        <p className="text-xs text-text-muted mt-1.5">The URL of your WG Manager instance</p>
      </div>

      <button
        type="submit"
        disabled={!serverURL.trim()}
        className="w-full px-4 py-2 text-sm font-medium text-white bg-accent hover:bg-accent-hover disabled:opacity-50 disabled:cursor-not-allowed rounded-lg transition-colors"
      >
        Continue
      </button>
    </form>
  )
}

function StepRegister({ deviceName, onNameChange, onLoadDefaultName, onRegister, loading, error, onClearError }: {
  deviceName: string
  onNameChange: (name: string) => void
  onLoadDefaultName: () => Promise<void>
  onRegister: () => Promise<void>
  loading: boolean
  error: string | null
  onClearError: () => void
}) {
  useEffect(() => {
    if (!deviceName.trim()) {
      void onLoadDefaultName()
    }
  }, [deviceName, onLoadDefaultName])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    onClearError()
    try {
      await onRegister()
    } catch {
      // error is in store
    }
  }

  return (
    <form onSubmit={handleSubmit} className="p-6">
      <div className="flex items-center gap-2.5 mb-4">
        <Monitor className="w-5 h-5 text-accent" />
        <h2 className="text-base font-semibold text-text-primary">Register Device</h2>
      </div>
      <p className="text-sm text-text-secondary mb-4">
        Give this device a name. A unique encryption key pair will be generated and registered with the server.
      </p>

      {error && <ErrorBox message={error} onDismiss={onClearError} />}

      <div className="mb-5">
        <label className="block text-xs font-medium text-text-secondary mb-1.5">Device name</label>
        <input
          className="w-full px-3 py-2 bg-bg-base border border-bg-border rounded-lg text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors"
          placeholder="My Android"
          value={deviceName}
          onChange={e => onNameChange(e.target.value)}
          required
          autoFocus
        />
      </div>

      <button
        type="submit"
        disabled={loading || !deviceName.trim()}
        className="w-full px-4 py-2 text-sm font-medium text-white bg-accent hover:bg-accent-hover disabled:opacity-50 disabled:cursor-not-allowed rounded-lg transition-colors"
      >
        {loading ? 'Registering…' : 'Register Device'}
      </button>
    </form>
  )
}

function StepLogin({ login, onComplete, loading, error, onClearError }: {
  login: (username: string, password: string, totpCode: string) => Promise<{ requireTOTP: boolean }>
  onComplete: () => Promise<void>
  loading: boolean
  error: string | null
  onClearError: () => void
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [totpCode, setTotpCode] = useState('')
  const [showTOTP, setShowTOTP] = useState(false)
  const [showPw, setShowPw] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    onClearError()
    try {
      const result = await login(username, password, showTOTP ? totpCode : '')
      if (result.requireTOTP) {
        setShowTOTP(true)
        return
      }
      await onComplete()
    } catch {
      // error is in store
    }
  }

  return (
    <form onSubmit={handleSubmit} className="p-6">
      <div className="flex items-center gap-2.5 mb-4">
        <LogIn className="w-5 h-5 text-accent" />
        <h2 className="text-base font-semibold text-text-primary">Sign In</h2>
      </div>
      <p className="text-sm text-text-secondary mb-4">
        Sign in with your VPN account credentials.
      </p>

      {error && <ErrorBox message={error} onDismiss={onClearError} />}

      <div className="space-y-4 mb-5">
        <div>
          <label className="block text-xs font-medium text-text-secondary mb-1.5">Username</label>
          <input
            className="w-full px-3 py-2 bg-bg-base border border-bg-border rounded-lg text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors"
            placeholder="admin"
            value={username}
            onChange={e => setUsername(e.target.value)}
            required
            autoFocus={!showTOTP}
            disabled={showTOTP}
          />
        </div>

        <div>
          <label className="block text-xs font-medium text-text-secondary mb-1.5">Password</label>
          <div className="relative">
            <input
              className="w-full px-3 py-2 pr-9 bg-bg-base border border-bg-border rounded-lg text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors"
              type={showPw ? 'text' : 'password'}
              placeholder="••••••••"
              value={password}
              onChange={e => setPassword(e.target.value)}
              required
              disabled={showTOTP}
            />
            <button
              type="button"
              onClick={() => setShowPw(v => !v)}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-secondary transition-colors"
            >
              {showPw ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </button>
          </div>
        </div>

        {showTOTP && (
          <div>
            <label className="block text-xs font-medium text-text-secondary mb-1.5">2FA Code</label>
            <input
              className="w-full px-3 py-2 bg-bg-base border border-bg-border rounded-lg text-sm text-text-primary text-center tracking-widest font-mono placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors"
              placeholder="000000"
              value={totpCode}
              onChange={e => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
              maxLength={6}
              autoFocus
              required
            />
            <p className="text-xs text-text-muted mt-1.5">Enter the 6-digit code from your authenticator app</p>
          </div>
        )}
      </div>

      <button
        type="submit"
        disabled={loading}
        className="w-full px-4 py-2 text-sm font-medium text-white bg-accent hover:bg-accent-hover disabled:opacity-50 disabled:cursor-not-allowed rounded-lg transition-colors"
      >
        {loading ? 'Signing in…' : showTOTP ? 'Verify' : 'Sign In'}
      </button>
    </form>
  )
}

function ErrorBox({ message, onDismiss }: { message: string; onDismiss: () => void }) {
  return (
    <div className="flex items-start gap-2 px-3 py-2.5 bg-danger/10 border border-danger/20 rounded-lg mb-4">
      <AlertCircle className="w-4 h-4 text-danger shrink-0 mt-0.5" />
      <p className="text-sm text-danger flex-1">{message}</p>
      <button
        type="button"
        onClick={onDismiss}
        className="text-danger/60 hover:text-danger text-xs shrink-0"
      >
        ✕
      </button>
    </div>
  )
}
