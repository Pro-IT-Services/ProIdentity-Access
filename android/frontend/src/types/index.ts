export type TunnelStatus = 'disconnected' | 'connecting' | 'connected' | 'error'

export interface PeerInfo {
  public_key: string
  endpoint: string
  allowed_ips: string[]
  persistent_keepalive: number
}

export interface TunnelInfo {
  id: string
  name: string
  status: TunnelStatus
  addresses: string[]
  dns: string[]
  mtu: number
  listen_port: number
  private_key: string
  peers: PeerInfo[]
  error?: string
  is_managed?: boolean
}

export interface StatsInfo {
  tunnel_id: string
  rx_bytes: number
  tx_bytes: number
  last_handshake: number // Unix timestamp seconds
}

export interface DaemonStatus {
  running: boolean
  daemon_version: string
}

export interface ManagedSettings {
  server_url: string
  username: string
  is_admin: boolean
  logged_in: boolean
  vpn_name: string
  totp_enabled: boolean
}

export interface ManagedLoginResult {
  username: string
  is_admin: boolean
  require_totp: boolean
  totp_enabled: boolean
}

export interface ServerInfo {
  id: string
  name: string
  endpoint: string
  port: number
  public_key: string
  subnet: string
  dns?: string
}

export interface ServerStatus {
  server: ServerInfo
  connected: boolean
  tunnelId: string | null
  connecting: boolean
  error: string | null
}

export interface SetupStatus {
  setup_done: boolean
  mode: string
}
