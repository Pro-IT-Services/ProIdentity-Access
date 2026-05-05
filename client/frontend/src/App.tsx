import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTunnelStore } from './stores/useTunnelStore'
import { useManagedStore } from './stores/useManagedStore'
import { useSetupStore } from './stores/useSetupStore'
import { useTrafficHistory } from './stores/useTrafficHistory'
import { Topbar } from './components/Topbar'
import { MissionControl } from './components/mission/MissionControl'
import { ConnectionsList } from './components/ConnectionsList'
import { ConfigDisclosure } from './components/ConfigDisclosure'
import { ImportSheet } from './components/ImportSheet'
import { LoginSheet } from './components/LoginSheet'
import { SettingsSheet } from './components/SettingsSheet'
import { TotpPromptSheet } from './components/TotpPromptSheet'
import { TrayMiniDashboard } from './components/tray/TrayMiniDashboard'
import { Sheet } from './components/ui/Sheet'
import SetupWizard from './components/SetupWizard'
import ErrorBoundary from './components/ErrorBoundary'
import { ToastContainer, toast } from './components/ui/Toast'
import {
  checkForUpdate,
  managedConnectServerPush,
  managedCreatePushAuth,
  managedDisconnectByTunnelID,
  managedPollPushAuth,
} from './wailsbridge'
import type { ServerInfo, StatsInfo, TunnelInfo } from './types'
import {
  ScreenGetAll,
  WindowCenter,
  WindowHide,
  WindowSetAlwaysOnTop,
  WindowSetMaxSize,
  WindowSetMinSize,
  WindowSetPosition,
  WindowSetSize,
  WindowShow,
  WindowUnminimise,
} from '../wailsjs/runtime/runtime'

const LAST_SERVER_KEY = 'proidentity:lastServerId'
const CONNECT_TIMEOUT_MS = 15_000

function withTimeout<T>(p: Promise<T>, ms: number, label: string): Promise<T> {
  return Promise.race([
    p,
    new Promise<never>((_, reject) =>
      setTimeout(() => reject(new Error(`${label}: timed out after ${ms / 1000}s`)), ms),
    ),
  ])
}

const delay = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

/**
 * Build a TunnelInfo-shaped placeholder from a managed server. Used so the
 * Mission Control orb can show "connecting to TESTER" or "TESTER (last)" even
 * before/after a real tunnel exists.
 *
 * The synthesized id has a `__synth_` prefix so callers can detect it and
 * route the action through connectServer instead of connectTunnel.
 */
function synthFromServer(srv: ServerInfo, status: TunnelInfo['status']): TunnelInfo {
  return {
    id: `__synth_${srv.id}`,
    name: srv.name,
    status,
    addresses: [],
    dns: srv.dns ? [srv.dns] : [],
    mtu: 0,
    listen_port: 0,
    private_key: '',
    peers: [{
      public_key: '',
      endpoint: `${srv.endpoint}:${srv.port}`,
      allowed_ips: [],
      persistent_keepalive: 0,
    }],
    is_managed: true,
  }
}

function isSynth(t: TunnelInfo | null): boolean {
  return !!t && t.id.startsWith('__synth_')
}

export default function App() {
  const { tunnels, refresh, updateStats, updateTunnel, selectedId, setSelected, connect, disconnect } = useTunnelStore()
  const { servers, loadSettings, loadServers, connectServer, settings, logout } = useManagedStore()
  const { setupDone, checkSetup } = useSetupStore()
  const pushSample = useTrafficHistory(s => s.pushSample)
  const resetHistory = useTrafficHistory(s => s.reset)

  const [showImport, setShowImport]       = useState(false)
  const [showConns, setShowConns]         = useState(false)
  const [showConfig, setShowConfig]       = useState(false)
  const [showLogin, setShowLogin]         = useState(false)
  const [showSettings, setShowSettings]   = useState(false)
  const [setupChecked, setSetupChecked]   = useState(false)
  const [wizardFromSettings, setWizardFromSettings] = useState(false)
  const [trayPopoverOpen, setTrayPopoverOpen] = useState(false)

  // TOTP prompt for connect-time 2FA. Stores the server pending verification.
  const [totpForServer, setTotpForServer] = useState<ServerInfo | null>(null)
  const [pushApprovingServer, setPushApprovingServer] = useState<ServerInfo | null>(null)
  const [pushAuthAvailable, setPushAuthAvailable] = useState(false)
  const pushConnectRef = useRef<{ serverId: string; cancelled: boolean } | null>(null)

  // Optimistic "we just clicked Connect on a server" — surfaces immediately in
  // the orb, regardless of how long the daemon takes to ack.
  const [pendingServerId, setPendingServerId] = useState<string | null>(null)

  // Last successfully-connected managed server — survives reload via localStorage.
  const [lastServerId, setLastServerIdState] = useState<string | null>(() => {
    try { return localStorage.getItem(LAST_SERVER_KEY) } catch { return null }
  })
  const rememberLastServer = useCallback((id: string) => {
    setLastServerIdState(id)
    try { localStorage.setItem(LAST_SERVER_KEY, id) } catch { /* ignore */ }
  }, [])

  useEffect(() => {
    const isMac = /Macintosh|MacIntel|MacPPC|Mac68K/.test(navigator.userAgent)
    document.documentElement.style.setProperty('--titlebar-height', isMac ? '80px' : '16px')
    document.documentElement.style.setProperty('--tray-safe-top', isMac ? '44px' : '0px')
    document.documentElement.style.setProperty('--tray-safe-left', '16px')
  }, [])

  useEffect(() => { checkSetup().finally(() => setSetupChecked(true)) }, [])

  useEffect(() => {
    refresh()
    loadSettings()
    const t = setInterval(refresh, 5000)
    return () => clearInterval(t)
  }, [])

  // Poll managed servers every 10s when logged in.
  useEffect(() => {
    if (!settings.logged_in) return
    loadServers()
    const t = setInterval(loadServers, 10_000)
    return () => clearInterval(t)
  }, [settings.logged_in])

  useEffect(() => {
    if (!settings.server_url || !settings.logged_in) return
    let cancelled = false
    const t = setTimeout(() => {
      checkForUpdate()
        .then(info => {
          if (!cancelled && info.available) {
            toast(`ProIdentity ${info.latest_version} is available. Open Settings to install it.`, 'info', 12_000)
          }
        })
        .catch(() => {})
    }, 5000)
    return () => { cancelled = true; clearTimeout(t) }
  }, [settings.server_url, settings.logged_in])

  const openTrayPopover = useCallback(() => {
    const isMac = /Macintosh|MacIntel|MacPPC|Mac68K/.test(navigator.userAgent)
    const width = 430
    const height = isMac ? 540 : 580
    setTrayPopoverOpen(true)
    if (settings.logged_in) {
      loadServers().catch(() => {})
    }
    WindowSetMinSize(width, height)
    WindowSetMaxSize(width, height)
    WindowSetSize(width, height)
    ScreenGetAll()
      .then(screens => {
        const screen = screens.find(s => s.isCurrent || s.isPrimary) ?? screens[0]
        if (!screen) {
          WindowCenter()
          return
        }
        WindowSetPosition(
          Math.max(16, screen.width - width - 24),
          Math.max(16, screen.height - height - 72),
        )
      })
      .catch(() => WindowCenter())
    WindowSetAlwaysOnTop(true)
    WindowShow()
    WindowUnminimise()
  }, [loadServers, settings.logged_in])

  useEffect(() => {
    const rt = (window as any).runtime
    if (!rt?.EventsOn) return
    rt.EventsOn('tunnel.changed', (...args: unknown[]) => {
      const info = args[0] as TunnelInfo
      if (info) updateTunnel(info)
    })
    rt.EventsOn('stats.update', (...args: unknown[]) => {
      const stats = args[0] as StatsInfo
      if (!stats) return
      updateStats(stats)
      pushSample(stats.tunnel_id, Date.now(), stats.rx_bytes, stats.tx_bytes)
    })
    rt.EventsOn('servers.changed', () => {
      if (useManagedStore.getState().settings.logged_in) loadServers()
    })
    rt.EventsOn('session.revoked', () => {
      toast('Your VPN session was revoked by the server.', 'revoked', 10_000)
      loadServers()
      refresh()
    })
    rt.EventsOn('auth.expired', () => {
      toast('Your login expired or was revoked. Please set up again.', 'warning', 10_000)
      useTunnelStore.setState({ tunnels: [], selectedId: null, stats: {}, loading: false })
      useManagedStore.setState({
        servers: [],
        settings: { server_url: '', username: '', is_admin: false, logged_in: false, vpn_name: '', totp_enabled: false },
        loading: false, error: null,
      })
      useSetupStore.setState({ setupDone: false, step: 'mode', mode: null, serverURL: '', deviceName: '', loading: false, error: null })
      try { localStorage.removeItem(LAST_SERVER_KEY) } catch { /* ignore */ }
      setLastServerIdState(null)
      setShowLogin(false)
    })
    rt.EventsOn('installation_revoked', () => {
      toast('Your login expired or device registration was revoked. Please set up again.', 'revoked', 0)
      useTunnelStore.setState({ tunnels: [], selectedId: null, stats: {}, loading: false })
      useManagedStore.setState({
        servers: [],
        settings: { server_url: '', username: '', is_admin: false, logged_in: false, vpn_name: '', totp_enabled: false },
        loading: false, error: null,
      })
      useSetupStore.setState({ setupDone: false, step: 'mode', mode: null, serverURL: '', deviceName: '', loading: false, error: null })
      try { localStorage.removeItem(LAST_SERVER_KEY) } catch { /* ignore */ }
      setLastServerIdState(null)
    })
    rt.EventsOn('tray:popover', openTrayPopover)
    rt.EventsOn('tray:show-main', () => setTrayPopoverOpen(false))
  }, [openTrayPopover])

  // Garbage-collect throughput history for tunnels that have been removed.
  useEffect(() => {
    const ids = new Set(tunnels.map(t => t.id))
    Object.keys(useTrafficHistory.getState().history).forEach(id => {
      if (!ids.has(id)) resetHistory(id)
    })
  }, [tunnels])

  // Resolve focused tunnel — real first, then synthesized from pending or
  // last-known server so the main UI is never empty unless you've truly never
  // connected.
  const preferredManagedServer = useMemo<ServerInfo | null>(() => {
    const last = lastServerId ? servers.find(s => s.server.id === lastServerId)?.server : null
    return last ?? servers[0]?.server ?? null
  }, [servers, lastServerId])

  const focusedTunnel = useMemo<TunnelInfo | null>(() => {
    // 1. Live tunnel takes precedence.
    const live = tunnels.find(t => ['connected', 'connecting', 'reconnecting', 'error'].includes(t.status as any))
    if (live) return live

    // 2. Optimistic pending — set synchronously by the Connect button.
    if (pendingServerId) {
      const srv = servers.find(s => s.server.id === pendingServerId)?.server
      if (srv) return synthFromServer(srv, 'connecting')
    }

    // 3. The user previously selected one; honor that.
    if (selectedId) {
      const sel = tunnels.find(t => t.id === selectedId)
      if (sel) return sel
    }

    // 4. Any imported tunnel (these persist across disconnects, so this brings
    //    back the last imported tunnel in disconnected state automatically).
    if (tunnels[0]) return tunnels[0]

    // 5. Last-known managed server — synthesized in disconnected state.
    if (preferredManagedServer) {
      const connectingServer = servers.find(s => s.server.id === preferredManagedServer.id)?.connecting
      return synthFromServer(preferredManagedServer, connectingServer ? 'connecting' : 'disconnected')
    }

    return null
  }, [tunnels, servers, selectedId, pendingServerId, preferredManagedServer])

  // When a real tunnel for the pending server materializes (or the connect
  // succeeded and the server now has a tunnelId), clear the optimistic flag.
  // Also clear if no tunnel is live and the server isn't connecting — prevents
  // stale "connecting" state after disconnect.
  useEffect(() => {
    if (!pendingServerId) return
    const srv = servers.find(s => s.server.id === pendingServerId)
    if (srv?.tunnelId && tunnels.some(t => t.id === srv.tunnelId)) {
      setPendingServerId(null)
      return
    }
    if (totpForServer?.id === pendingServerId || pushApprovingServer?.id === pendingServerId) {
      return
    }
    if (!srv?.connecting && !tunnels.some(t => t.status === 'connecting')) {
      const timeout = setTimeout(() => setPendingServerId(null), 3000)
      return () => clearTimeout(timeout)
    }
  }, [tunnels, servers, pendingServerId, totpForServer?.id, pushApprovingServer?.id])

  // Persist the most recently connected managed server.
  useEffect(() => {
    if (!focusedTunnel || isSynth(focusedTunnel)) return
    if (focusedTunnel.is_managed && focusedTunnel.status === 'connected') {
      const owner = servers.find(s => s.tunnelId === focusedTunnel.id)
      if (owner) rememberLastServer(owner.server.id)
    }
  }, [focusedTunnel, servers, rememberLastServer])

  // Sync selected id with focused (so the connections list highlights right).
  useEffect(() => {
    if (focusedTunnel && !isSynth(focusedTunnel) && focusedTunnel.id !== selectedId) {
      setSelected(focusedTunnel.id)
    }
  }, [focusedTunnel?.id])

  // Connect a managed server. Always attempts the connect first so the server
  // can tell us whether push auth or classic TOTP is required.
  const connectManaged = useCallback(async (srv: ServerInfo) => {
    if (pushConnectRef.current?.serverId === srv.id && !pushConnectRef.current.cancelled) {
      return
    }
    setPendingServerId(srv.id)
    try {
      await withTimeout(connectServer(srv), CONNECT_TIMEOUT_MS, 'Connect')
      rememberLastServer(srv.id)
    } catch (e: any) {
      const msg = String(e?.message ?? e)
      if (msg.includes('require_push_auth')) {
        pushConnectRef.current = { ...(pushConnectRef.current ?? { serverId: srv.id, cancelled: false }), cancelled: true }
        const run = { serverId: srv.id, cancelled: false }
        pushConnectRef.current = run
        setPushAuthAvailable(false)
        setPushApprovingServer(srv)
        setTotpForServer(null)
        setPendingServerId(srv.id)
        try {
          const push = await managedCreatePushAuth(`Connect to ${srv.name}`)
          for (let attempt = 0; attempt < 45; attempt += 1) {
            if (run.cancelled) return
            await delay(2000)
            if (run.cancelled) return
            const status = await managedPollPushAuth(push.request_id)
            if (status === 'approved') {
              const tunnel = await withTimeout(
                managedConnectServerPush(srv.id, srv.name, push.request_id),
                CONNECT_TIMEOUT_MS,
                'Connect',
              )
              useManagedStore.setState(s => ({
                servers: s.servers.map(ss =>
                  ss.server.id === srv.id
                    ? { ...ss, connected: true, connecting: false, tunnelId: tunnel.id, error: null }
                    : ss
                ),
              }))
              rememberLastServer(srv.id)
              setTotpForServer(null)
              setPushApprovingServer(null)
              setPushAuthAvailable(false)
              await loadServers()
              return
            }
            if (status === 'denied' || status === 'expired') {
              throw new Error(status === 'denied' ? 'Push request denied.' : 'Push request expired.')
            }
          }
          throw new Error('Push request timed out.')
        } catch (pushErr: any) {
          if (!run.cancelled) {
            toast(String(pushErr?.message ?? pushErr), 'warning', 8000)
            setPushApprovingServer(null)
            setPushAuthAvailable(false)
            setTotpForServer(srv)
            setPendingServerId(null)
          }
        } finally {
          if (pushConnectRef.current === run) pushConnectRef.current = null
        }
      } else if (msg.includes('require_totp') || msg.includes('totp')) {
        setPendingServerId(null)
        setPushApprovingServer(null)
        setPushAuthAvailable(false)
        setTotpForServer(srv)
      } else {
        setPendingServerId(null)
        setPushApprovingServer(null)
        console.warn('connect failed', e)
      }
    }
  }, [connectServer, loadServers, rememberLastServer])

  // Primary action — routes based on synthetic vs real, managed vs imported.
  const handleConnect = useCallback(async () => {
    const t = focusedTunnel
    if (!t) {
      if (preferredManagedServer) await connectManaged(preferredManagedServer)
      return
    }
    if (isSynth(t)) {
      const serverId = t.id.replace('__synth_', '')
      const srv = servers.find(s => s.server.id === serverId)?.server ?? preferredManagedServer
      if (!srv) return
      await connectManaged(srv)
    } else {
      try {
        await withTimeout(connect(t.id), CONNECT_TIMEOUT_MS, 'Connect')
      } catch (e: any) {
        console.warn('connect failed', e)
      }
    }
  }, [focusedTunnel, servers, preferredManagedServer, connectManaged, connect])

  const handleDisconnect = useCallback(async () => {
    const t = focusedTunnel
    if (!t || isSynth(t)) return
    setPendingServerId(null)
    if (t.is_managed) {
      await managedDisconnectByTunnelID(t.id)
      await loadServers()
    } else {
      await disconnect(t.id)
    }
  }, [focusedTunnel, disconnect, loadServers])

  // Called by the connections list the instant Connect is clicked, before
  // the awaited API call returns.
  const handleConnectIntent = useCallback((srv: ServerInfo) => {
    setPendingServerId(srv.id)
  }, [])

  // Re-open the first-run setup wizard from Settings.
  const handleReRunSetup = useCallback(() => {
    setShowSettings(false)
    setWizardFromSettings(true)
    useSetupStore.setState({ setupDone: false, step: 'mode', mode: null, error: null })
  }, [])

  const restoreMainWindow = useCallback(() => {
    setTrayPopoverOpen(false)
    WindowSetAlwaysOnTop(false)
    WindowSetMaxSize(0, 0)
    WindowSetMinSize(800, 580)
    WindowSetSize(960, 660)
    WindowCenter()
  }, [])

  const closeTrayPopover = useCallback(() => {
    setTrayPopoverOpen(false)
    WindowSetAlwaysOnTop(false)
    WindowHide()
  }, [])

  if (!setupChecked) return null
  if (!setupDone) {
    const handleWizardClose = wizardFromSettings
      ? () => { setWizardFromSettings(false); useSetupStore.setState({ setupDone: true }) }
      : undefined
    return <SetupWizard onClose={handleWizardClose} />
  }

  const focusedStats = focusedTunnel && !isSynth(focusedTunnel)
    ? useTunnelStore.getState().stats[focusedTunnel.id]
    : undefined
  const focusedHistory = focusedTunnel && !isSynth(focusedTunnel)
    ? (useTrafficHistory.getState().history[focusedTunnel.id] ?? [])
    : []

  if (trayPopoverOpen) {
    const authState =
      pushApprovingServer ? 'approving'
      : totpForServer ? (pushAuthAvailable ? 'approving' : 'required')
      : settings.logged_in ? 'push'
      : 'disabled'
    return (
      <ErrorBoundary>
        <TrayMiniDashboard
          tunnel={focusedTunnel}
          stats={focusedStats}
          history={focusedHistory}
          settings={settings}
          authState={authState}
          onConnect={handleConnect}
          onDisconnect={handleDisconnect}
          onOpenApp={restoreMainWindow}
          onClose={closeTrayPopover}
        />
        <ToastContainer />
      </ErrorBoundary>
    )
  }

  return (
    <div className="flex flex-col h-full bg-background">
      <Topbar
        onImport={() => setShowImport(true)}
        onSettings={() => setShowSettings(true)}
        onSignIn={!settings.logged_in ? () => setShowLogin(true) : undefined}
        onSignOut={settings.logged_in ? () => logout() : undefined}
        onOpenConnections={() => setShowConns(true)}
        onOpenConfig={() => setShowConfig(true)}
        configEnabled={!!focusedTunnel && !isSynth(focusedTunnel)}
      />

      <main className="relative flex-1 overflow-hidden flex flex-col">
        <ErrorBoundary>
          <MissionControl
            tunnel={focusedTunnel}
            onOpenConnections={() => setShowConns(true)}
            onImport={() => setShowImport(true)}
            onConnect={handleConnect}
            onDisconnect={handleDisconnect}
          />
        </ErrorBoundary>
      </main>

      <ImportSheet open={showImport} onClose={() => setShowImport(false)} />

      <Sheet
        open={showConns}
        onClose={() => setShowConns(false)}
        title="Connections"
        description="Pick a server to switch, or sign in to load more."
      >
        <ErrorBoundary>
          <ConnectionsList
            onConnected={() => setShowConns(false)}
            onConnectIntent={handleConnectIntent}
            onConnectManaged={connectManaged}
            onSignIn={() => { setShowConns(false); setShowLogin(true) }}
          />
        </ErrorBoundary>
      </Sheet>

      <Sheet
        open={showConfig}
        onClose={() => setShowConfig(false)}
        title="Configuration"
        description={focusedTunnel?.name}
        widthPx={460}
      >
        <ErrorBoundary>
          {focusedTunnel && !isSynth(focusedTunnel)
            ? <ConfigDisclosure tunnel={focusedTunnel} />
            : <p className="text-sm text-muted-foreground">No tunnel selected.</p>}
        </ErrorBoundary>
      </Sheet>

      <SettingsSheet
        open={showSettings}
        onClose={() => setShowSettings(false)}
        onReRunSetup={handleReRunSetup}
        onSignIn={() => { setShowSettings(false); setShowLogin(true) }}
      />

      <LoginSheet
        open={showLogin}
        onClose={() => setShowLogin(false)}
        onLoggedIn={() => { setShowLogin(false); loadServers() }}
      />

      <TotpPromptSheet
        open={!!totpForServer}
        serverName={totpForServer?.name}
        pushAuthEnabled={pushAuthAvailable}
        onCancel={() => { setTotpForServer(null); setPendingServerId(null) }}
        onSubmit={async (code) => {
          if (!totpForServer) return
          setPendingServerId(totpForServer.id)
          try {
            await withTimeout(connectServer(totpForServer, code), CONNECT_TIMEOUT_MS, 'Connect')
            setTotpForServer(null)
          } catch (e) {
            setPendingServerId(null)
            throw e
          }
        }}
        onPushApproved={async (requestId) => {
          if (!totpForServer) return
          setPendingServerId(totpForServer.id)
          try {
            await withTimeout(
              managedConnectServerPush(totpForServer.id, totpForServer.name, requestId),
              CONNECT_TIMEOUT_MS,
              'Connect',
            )
            setTotpForServer(null)
          } catch (e) {
            setPendingServerId(null)
            throw e
          }
        }}
      />

      <ToastContainer />
    </div>
  )
}
