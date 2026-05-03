/**
 * Wails bridge — uses the auto-generated wailsjs bindings.
 * In dev mode (browser without Wails), all calls return mock data.
 */

import type { TunnelInfo, StatsInfo, ManagedSettings, ManagedLoginResult, ServerInfo } from './types'

// Check if we're running inside Wails
const isWails = (): boolean =>
  typeof window !== 'undefined' && typeof (window as any).go !== 'undefined'

// Lazy-import the generated bindings only when inside Wails
async function App() {
  const mod = await import('../wailsjs/go/main/App')
  return mod
}

// --- Standard tunnel API ---

export async function isDaemonRunning(): Promise<boolean> {
  if (!isWails()) return false
  try {
    return await (await App()).IsDaemonRunning()
  } catch {
    return false
  }
}

export async function listTunnels(): Promise<TunnelInfo[]> {
  if (!isWails()) return MOCK_TUNNELS
  const result = await (await App()).ListTunnels()
  return (result as TunnelInfo[]) ?? []
}

export async function importTunnel(name: string, configContent: string): Promise<TunnelInfo> {
  if (!isWails()) {
    const mock: TunnelInfo = {
      id: Math.random().toString(36).slice(2),
      name: name || 'My Tunnel',
      status: 'disconnected',
      addresses: ['10.0.0.2/24'],
      dns: ['1.1.1.1'],
      mtu: 1420,
      listen_port: 0,
      private_key: '',
      peers: [],
    }
    MOCK_TUNNELS.push(mock)
    return mock
  }
  const result = await (await App()).ImportTunnel(name, configContent)
  return result as TunnelInfo
}

export async function deleteTunnel(id: string): Promise<void> {
  if (!isWails()) {
    const idx = MOCK_TUNNELS.findIndex(t => t.id === id)
    if (idx !== -1) MOCK_TUNNELS.splice(idx, 1)
    return
  }
  await (await App()).DeleteTunnel(id)
}

export async function connectTunnel(id: string): Promise<void> {
  if (!isWails()) {
    const t = MOCK_TUNNELS.find(t => t.id === id)
    if (t) t.status = 'connected'
    return
  }
  await (await App()).ConnectTunnel(id)
}

export async function disconnectTunnel(id: string): Promise<void> {
  if (!isWails()) {
    const t = MOCK_TUNNELS.find(t => t.id === id)
    if (t) t.status = 'disconnected'
    return
  }
  await (await App()).DisconnectTunnel(id)
}

export async function getStats(id: string): Promise<StatsInfo> {
  if (!isWails()) {
    return { tunnel_id: id, rx_bytes: 1234567, tx_bytes: 987654, last_handshake: Date.now() / 1000 - 30 }
  }
  return await (await App()).GetStats(id) as StatsInfo
}

// --- Managed mode API ---

export async function managedGetSettings(): Promise<ManagedSettings> {
  if (!isWails()) return { server_url: '', username: '', is_admin: false, logged_in: false, vpn_name: '', totp_enabled: false }
  return await (await App()).ManagedGetSettings() as ManagedSettings
}

export async function managedSaveServerURL(serverURL: string): Promise<void> {
  if (!isWails()) return
  await (await App()).ManagedSaveServerURL(serverURL)
}

export async function managedLogin(username: string, password: string, totpCode: string): Promise<ManagedLoginResult> {
  if (!isWails()) {
    // Mock: simulate successful login
    MOCK_MANAGED_SETTINGS.username = username
    MOCK_MANAGED_SETTINGS.logged_in = true
    return { username, is_admin: false, require_totp: false, totp_enabled: false }
  }
  return await (await App()).ManagedLogin(username, password, totpCode) as ManagedLoginResult
}

export async function managedLoginWithPush(username: string, password: string, pushAuthID: string): Promise<ManagedLoginResult> {
  return await (await App()).ManagedLoginWithPush(username, password, pushAuthID) as ManagedLoginResult
}

export async function managedLogout(): Promise<void> {
  if (!isWails()) {
    MOCK_MANAGED_SETTINGS.username = ''
    MOCK_MANAGED_SETTINGS.logged_in = false
    return
  }
  await (await App()).ManagedLogout()
}

export async function managedListServers(): Promise<ServerInfo[]> {
  if (!isWails()) return MOCK_SERVERS
  return await (await App()).ManagedListServers() as ServerInfo[]
}

export async function managedConnectServer(serverID: string, serverName: string, totpCode: string = ''): Promise<TunnelInfo> {
  if (!isWails()) {
    const mock: TunnelInfo = {
      id: 'managed-' + serverID,
      name: serverName,
      status: 'connected',
      addresses: ['10.8.0.5/32'],
      dns: ['1.1.1.1'],
      mtu: 1420,
      listen_port: 0,
      private_key: '',
      peers: [{ public_key: '', endpoint: 'vpn.example.com:51820', allowed_ips: ['0.0.0.0/0'], persistent_keepalive: 25 }],
    }
    MOCK_TUNNELS.push(mock)
    return mock
  }
  return await (await App()).ManagedConnectServer(serverID, serverName, totpCode) as TunnelInfo
}

export async function managedConnectServerPush(serverID: string, serverName: string, pushAuthID: string): Promise<TunnelInfo> {
  return await (await App()).ManagedConnectServerPush(serverID, serverName, pushAuthID) as TunnelInfo
}

export async function managedCreatePushAuth(context: string): Promise<{ request_id: string; status: string; expires_at: number }> {
  return await (await App()).ManagedCreatePushAuth(context) as any
}

export async function managedPollPushAuth(requestID: string): Promise<string> {
  return await (await App()).ManagedPollPushAuth(requestID) as string
}

export async function managedDisconnectServer(serverID: string): Promise<void> {
  if (!isWails()) {
    const idx = MOCK_TUNNELS.findIndex(t => t.id === 'managed-' + serverID)
    if (idx !== -1) MOCK_TUNNELS.splice(idx, 1)
    return
  }
  await (await App()).ManagedDisconnectServer(serverID)
}

export async function managedActiveServers(): Promise<string[]> {
  if (!isWails()) return []
  return await (await App()).ManagedActiveServers() as string[]
}

export async function managedCheckSetup(): Promise<boolean> {
  if (!isWails()) return false
  return await (await App()).ManagedCheckSetup() as boolean
}

export async function managedGetSetupStep(): Promise<string> {
  if (!isWails()) return 'mode'
  return await (await App()).ManagedGetSetupStep() as string
}

export async function managedDisconnectByTunnelID(tunnelID: string): Promise<void> {
  if (!isWails()) return
  await (await App()).ManagedDisconnectByTunnelID(tunnelID)
}

export async function managedSetMode(mode: string): Promise<void> {
  if (!isWails()) return
  await (await App()).ManagedSetMode(mode)
}

export async function managedDefaultDeviceName(): Promise<string> {
  if (!isWails()) return 'ProIdentity Desktop'
  return await (await App()).ManagedDefaultDeviceName() as string
}

export async function managedRegisterDevice(serverURL: string, deviceName: string): Promise<void> {
  if (!isWails()) return
  await (await App()).ManagedRegisterDevice(serverURL, deviceName)
}

export async function managedCompleteSetup(): Promise<void> {
  if (!isWails()) return
  await (await App()).ManagedCompleteSetup()
}

export type UpdateCheckResult = {
  current_version: string
  latest_version: string
  available: boolean
  mandatory: boolean
  platform: string
  filename: string
  url: string
  sha256: string
  size: number
  published_at: string
}

export async function checkForUpdate(): Promise<UpdateCheckResult> {
  if (!isWails()) {
    return {
      current_version: '0.5.5',
      latest_version: '0.5.5',
      available: false,
      mandatory: false,
      platform: 'windows-amd64',
      filename: '',
      url: '',
      sha256: '',
      size: 0,
      published_at: '',
    }
  }
  return await (await App()).CheckForUpdate() as UpdateCheckResult
}

// Mock data for browser dev mode
const MOCK_TUNNELS: TunnelInfo[] = [
  {
    id: 'demo-1',
    name: 'Home Server',
    status: 'connected',
    addresses: ['10.0.0.2/24'],
    dns: ['1.1.1.1', '8.8.8.8'],
    mtu: 1420,
    listen_port: 0,
    private_key: '',
    peers: [
      {
        public_key: '',
        endpoint: 'vpn.example.com:51820',
        allowed_ips: ['0.0.0.0/0', '::/0'],
        persistent_keepalive: 25,
      },
    ],
  },
]

const MOCK_MANAGED_SETTINGS: ManagedSettings = {
  server_url: '',
  username: '',
  is_admin: false,
  logged_in: false,
  vpn_name: '',
  totp_enabled: false,
}

const MOCK_SERVERS: ServerInfo[] = [
  { id: 'server-1', name: 'EU West', endpoint: 'eu.vpn.example.com', port: 51820, public_key: '', subnet: '10.8.1.0/24' },
  { id: 'server-2', name: 'US East', endpoint: 'us.vpn.example.com', port: 51820, public_key: '', subnet: '10.8.2.0/24' },
]
