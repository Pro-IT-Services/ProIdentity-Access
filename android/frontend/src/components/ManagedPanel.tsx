import { useEffect, useRef, useState } from 'react'
import { Globe, Power, LogIn, LogOut, AlertCircle, RefreshCw, Lock } from 'lucide-react'
import { useManagedStore } from '../stores/useManagedStore'
import { useTunnelStore } from '../stores/useTunnelStore'
import ManagedLoginModal from './ManagedLoginModal'

export default function ManagedPanel() {
  const { settings, servers, error, loadServers, connectServer, disconnectServer, logout, clearError } = useManagedStore()
  const { refresh } = useTunnelStore()
  const [showLogin, setShowLogin] = useState(false)
  const [loadingServers, setLoadingServers] = useState(false)
  // TOTP prompt state
  const [totpServerID, setTotpServerID] = useState<string | null>(null)
  const [totpCode, setTotpCode] = useState('')
  const [totpError, setTotpError] = useState('')
  const [totpLoading, setTotpLoading] = useState(false)
  const totpInputRef = useRef<HTMLInputElement>(null)

  // Initial load when logged in
  useEffect(() => {
    if (settings.logged_in) {
      handleLoadServers()
    }
  }, [settings.logged_in])

  // Poll for server list changes every 10 seconds
  useEffect(() => {
    if (!settings.logged_in) return
    const interval = setInterval(() => {
      loadServers()
    }, 10000)
    return () => clearInterval(interval)
  }, [settings.logged_in])

  // Focus TOTP input when modal opens
  useEffect(() => {
    if (totpServerID) {
      setTotpCode('')
      setTotpError('')
      setTimeout(() => totpInputRef.current?.focus(), 50)
    }
  }, [totpServerID])

  if (!settings.server_url) return null

  const handleLoadServers = async () => {
    setLoadingServers(true)
    try {
      await loadServers()
    } finally {
      setLoadingServers(false)
    }
  }

  const handleConnect = async (serverID: string) => {
    clearError()
    const server = servers.find(s => s.server.id === serverID)?.server
    if (!server) return

    if (settings.totp_enabled) {
      setTotpServerID(serverID)
      return
    }

    try {
      await connectServer(server, '')
      await refresh()
    } catch (e: unknown) {
      if (String(e).includes('require_totp')) {
        setTotpServerID(serverID)
      }
      // other errors shown per-server
    }
  }

  const handleTotpConnect = async () => {
    if (!totpServerID || totpCode.length !== 6) return
    const server = servers.find(s => s.server.id === totpServerID)?.server
    if (!server) return

    setTotpLoading(true)
    setTotpError('')
    try {
      await connectServer(server, totpCode)
      await refresh()
      setTotpServerID(null)
    } catch (e: unknown) {
      const msg = String(e)
      if (msg.includes('invalid 2FA') || msg.includes('totp')) {
        setTotpError('Invalid code, try again')
      } else {
        setTotpError(msg)
        setTotpServerID(null)
      }
    } finally {
      setTotpLoading(false)
    }
  }

  const handleDisconnect = async (serverID: string) => {
    clearError()
    try {
      await disconnectServer(serverID)
      await refresh()
    } catch {
      // error shown per-server
    }
  }

  const totpServer = totpServerID ? servers.find(s => s.server.id === totpServerID) : null

  return (
    <>
      <div className="border-b border-bg-border px-3 py-3">
        <p className="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">
          {settings.vpn_name || 'Managed VPN'}
        </p>

        <div className="bg-bg-card border border-bg-border rounded-lg p-3.5 space-y-2.5">
          {/* Server URL */}
          <div className="flex items-center gap-2 min-w-0">
            <Globe className="w-4 h-4 text-text-muted flex-shrink-0" />
            <span className="text-xs text-text-secondary truncate">
              {settings.server_url.replace(/^https?:\/\//, '')}
            </span>
          </div>

          {/* Global error */}
          {error && (
            <div className="flex items-start gap-1.5 text-xs text-danger">
              <AlertCircle className="w-3.5 h-3.5 flex-shrink-0 mt-0.5" />
              <span className="break-words">{error}</span>
            </div>
          )}

          {!settings.logged_in ? (
            <button
              onClick={() => setShowLogin(true)}
              className="w-full flex items-center justify-center gap-2 px-3 py-3 bg-accent hover:bg-accent-hover text-white text-sm font-medium rounded-lg transition-colors"
            >
              <LogIn className="w-4 h-4" />
              Sign In
            </button>
          ) : (
            <div className="space-y-2.5">
              {/* User row */}
              <div className="flex items-center justify-between">
                <span className="text-xs text-text-secondary">
                  {settings.username}
                  {settings.is_admin && (
                    <span className="ml-1 text-warning text-[10px]">admin</span>
                  )}
                </span>
                <div className="flex items-center gap-1">
                  <button
                    onClick={handleLoadServers}
                    disabled={loadingServers}
                    className="p-1.5 rounded text-text-muted hover:text-text-primary hover:bg-bg-border transition-colors disabled:opacity-50"
                    title="Refresh servers"
                  >
                    <RefreshCw className={`w-3.5 h-3.5 ${loadingServers ? 'animate-spin' : ''}`} />
                  </button>
                  <button
                    onClick={logout}
                    className="p-1.5 rounded text-text-muted hover:text-danger hover:bg-danger/10 transition-colors"
                    title="Sign out"
                  >
                    <LogOut className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>

              {/* Server list */}
              {servers.length === 0 && !loadingServers && (
                <p className="text-xs text-text-muted text-center py-1">No servers available</p>
              )}

              {servers.map(ss => (
                <div key={ss.server.id} className="space-y-1">
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-text-primary truncate">{ss.server.name}</p>
                    </div>
                    {ss.connected ? (
                      <button
                        onClick={() => handleDisconnect(ss.server.id)}
                        disabled={ss.connecting}
                        className="flex-shrink-0 flex items-center gap-1.5 px-3 py-2 bg-success/10 hover:bg-success/20 text-success border border-success/20 text-sm font-medium rounded-lg transition-colors disabled:opacity-50"
                      >
                        <Power className="w-4 h-4" />
                        {ss.connecting ? '…' : 'Disconnect'}
                      </button>
                    ) : (
                      <button
                        onClick={() => handleConnect(ss.server.id)}
                        disabled={ss.connecting}
                        className="flex-shrink-0 flex items-center gap-1.5 px-3 py-2 bg-accent hover:bg-accent-hover text-white text-sm font-medium rounded-lg transition-colors disabled:opacity-50"
                      >
                        <Power className="w-4 h-4" />
                        {ss.connecting ? '…' : 'Connect'}
                      </button>
                    )}
                  </div>
                  {ss.error && !ss.error.includes('require_totp') && (
                    <p className="text-xs text-danger">{ss.error}</p>
                  )}
                  {ss.error && ss.error.includes('require_totp') && (
                    <button
                      onClick={() => setTotpServerID(ss.server.id)}
                      className="text-xs text-accent underline"
                    >
                      2FA required — tap to enter code
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {showLogin && <ManagedLoginModal onClose={() => setShowLogin(false)} />}

      {/* TOTP prompt modal */}
      {totpServerID && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-bg-surface border border-bg-border rounded-xl p-5 w-72 space-y-4 shadow-xl">
            <div className="flex items-center gap-2">
              <Lock className="w-4 h-4 text-accent flex-shrink-0" />
              <p className="text-sm font-medium text-text-primary">
                2FA required — {totpServer?.server.name}
              </p>
            </div>

            <p className="text-xs text-text-muted">
              Enter the 6-digit code from your authenticator app.
            </p>

            <input
              ref={totpInputRef}
              type="text"
              inputMode="numeric"
              maxLength={6}
              value={totpCode}
              onChange={e => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
              onKeyDown={e => e.key === 'Enter' && handleTotpConnect()}
              placeholder="000000"
              className="w-full text-center tracking-[0.5em] text-lg font-mono bg-bg-card border border-bg-border rounded-md px-3 py-2 text-text-primary outline-none focus:border-accent"
            />

            {totpError && (
              <p className="text-xs text-danger">{totpError}</p>
            )}

            <div className="flex gap-2">
              <button
                onClick={() => setTotpServerID(null)}
                className="flex-1 px-3 py-2 text-xs text-text-muted border border-bg-border rounded-md hover:bg-bg-border transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleTotpConnect}
                disabled={totpCode.length !== 6 || totpLoading}
                className="flex-1 px-3 py-2 text-xs text-white bg-accent hover:bg-accent-hover rounded-md transition-colors disabled:opacity-50"
              >
                {totpLoading ? '…' : 'Connect'}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
