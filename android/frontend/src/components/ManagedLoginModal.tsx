import { useState } from 'react'
import { Shield, X, Eye, EyeOff } from 'lucide-react'
import { useManagedStore } from '../stores/useManagedStore'

interface Props {
  onClose: () => void
}

export default function ManagedLoginModal({ onClose }: Props) {
  const { login, loading, error, clearError } = useManagedStore()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [totpCode, setTotpCode] = useState('')
  const [showTOTP, setShowTOTP] = useState(false)
  const [showPw, setShowPw] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    clearError()
    try {
      const result = await login(username, password, showTOTP ? totpCode : '')
      if (result.requireTOTP) {
        setShowTOTP(true)
        return
      }
      onClose()
    } catch {
      // error is set in store
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-bg-surface border border-bg-border rounded-2xl shadow-2xl w-full max-w-sm mx-4 overflow-hidden">
        <div className="flex items-center justify-between px-5 py-4 border-b border-bg-border">
          <div className="flex items-center gap-2.5">
            <Shield className="w-5 h-5 text-accent" />
            <span className="font-semibold text-text-primary">Sign in to Server</span>
          </div>
          <button onClick={onClose} className="p-1 rounded hover:bg-bg-border transition-colors text-text-muted hover:text-text-primary">
            <X className="w-4 h-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-5 space-y-4">
          {error && (
            <div className="px-3 py-2 bg-danger/10 border border-danger/20 rounded-lg text-sm text-danger">
              {error}
            </div>
          )}

          <div>
            <label className="block text-xs font-medium text-text-secondary mb-1.5">Username</label>
            <input
              className="w-full px-3 py-2 bg-bg-base border border-bg-border rounded-lg text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors"
              placeholder="admin"
              value={username}
              onChange={e => setUsername(e.target.value)}
              required
              autoFocus
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

          <div className="flex gap-2 pt-1">
            <button type="button" onClick={onClose}
              className="flex-1 px-4 py-2 text-sm text-text-secondary hover:text-text-primary bg-bg-base hover:bg-bg-border border border-bg-border rounded-lg transition-colors">
              Cancel
            </button>
            <button type="submit" disabled={loading}
              className="flex-1 px-4 py-2 text-sm font-medium text-white bg-accent hover:bg-accent-hover disabled:opacity-50 disabled:cursor-not-allowed rounded-lg transition-colors">
              {loading ? 'Signing in…' : showTOTP ? 'Verify' : 'Sign In'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
