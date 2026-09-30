import { useMemo } from 'react'
import { useTunnelStore } from '../stores/useTunnelStore'
import { useOpenVPNStore, isLive, OVPN_PREFIX } from '../stores/useOpenVPNStore'
import { t } from '../i18n'

/** A live VPN connection of either kind, for the "Connected now" list. */
export interface ActiveConnection {
  /** Tunnel id (WireGuard) or `ovpn:<session id>` (OpenVPN). */
  id: string
  name: string
  kind: 'WireGuard' | 'OpenVPN'
  status: 'connected' | 'connecting'
  ip: string
  /** Routed networks; a full tunnel is shown as "all traffic" (localized). */
  networks: string[]
  server: string
  /** Unix seconds; 0 when unknown. */
  since: number
  rx: number
  tx: number
}

// WireGuard tunnels don't report when they came up; remember when this
// window first saw each one connected.
const wgSince = new Map<string, number>()

const FULL = new Set(['0.0.0.0/0', '::/0'])

function routes(nets: string[]): string[] {
  const out = nets.filter(n => !FULL.has(n))
  if (out.length < nets.length) out.unshift(t('active.allTraffic'))
  return [...new Set(out)]
}

function host(endpoint: string): string {
  // "203.0.113.10:51820", "[2001:db8::1]:51820", "203.0.113.10:1194 (UDP)"
  const e = endpoint.replace(/\s*\((TCP|UDP)\)$/, '')
  if (e.startsWith('[')) return e.slice(1, e.indexOf(']'))
  return e.split(':')[0] || e
}

export function useActiveConnections(): ActiveConnection[] {
  const tunnels = useTunnelStore(s => s.tunnels)
  const stats = useTunnelStore(s => s.stats)
  const sessions = useOpenVPNStore(s => s.sessions)

  return useMemo(() => {
    const out: ActiveConnection[] = []
    const now = Math.floor(Date.now() / 1000)
    for (const t of tunnels) {
      if (t.status !== 'connected' && t.status !== 'connecting') {
        wgSince.delete(t.id)
        continue
      }
      if (t.status === 'connected' && !wgSince.has(t.id)) wgSince.set(t.id, now)
      const st = stats[t.id]
      out.push({
        id: t.id,
        name: t.name,
        kind: 'WireGuard',
        status: t.status,
        ip: t.addresses[0] ?? '',
        networks: routes(t.peers.flatMap(p => p.allowed_ips)),
        server: host(t.peers[0]?.endpoint ?? ''),
        since: t.status === 'connected' ? wgSince.get(t.id) ?? 0 : 0,
        rx: st?.rx_bytes ?? 0,
        tx: st?.tx_bytes ?? 0,
      })
    }
    for (const s of Object.values(sessions)) {
      if (!isLive(s)) continue
      out.push({
        id: OVPN_PREFIX + s.id,
        name: s.name,
        kind: 'OpenVPN',
        status: s.status as 'connected' | 'connecting',
        ip: s.ip ?? '',
        networks: routes(s.networks ?? []),
        server: host(s.remote ?? ''),
        since: s.connected_at ?? 0,
        rx: s.rx_bytes,
        tx: s.tx_bytes,
      })
    }
    return out
  }, [tunnels, stats, sessions])
}
