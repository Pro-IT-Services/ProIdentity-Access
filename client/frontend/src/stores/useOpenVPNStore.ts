import { create } from 'zustand'
import * as api from '../wailsbridge'
import type { OpenVPNStatus } from '../wailsbridge'
import type { TunnelInfo } from '../types'

/** Prefix for OpenVPN sessions shown in places that expect a TunnelInfo. */
export const OVPN_PREFIX = 'ovpn:'

export function isOpenVPNTunnel(t: TunnelInfo | null): boolean {
  return !!t && t.id.startsWith(OVPN_PREFIX)
}

/** A live OpenVPN session in the shape the status card understands. */
export function openvpnAsTunnel(s: OpenVPNStatus): TunnelInfo {
  const host = (s.remote ?? '').replace(/\s*\((TCP|UDP)\)$/, '')
  return {
    id: OVPN_PREFIX + s.id,
    name: s.name,
    status: s.status as TunnelInfo['status'],
    addresses: s.ip ? [s.ip] : [],
    dns: [],
    mtu: 0,
    listen_port: 0,
    private_key: '',
    peers: [{ public_key: '', endpoint: host, allowed_ips: s.networks ?? [], persistent_keepalive: 0 }],
    error: s.error,
  }
}

export const isLive = (s: OpenVPNStatus) => s.status === 'connected' || s.status === 'connecting'

interface OpenVPNStore {
  sessions: Record<string, OpenVPNStatus>
  load: () => Promise<void>
  apply: (s: OpenVPNStatus) => void
}

/**
 * OpenVPN sessions for the whole app (connections sheet and main screen).
 * App owns the single `openvpn.changed` subscription: Wails' EventsOff drops
 * every listener of an event, so components must not subscribe themselves.
 */
export const useOpenVPNStore = create<OpenVPNStore>((set) => ({
  sessions: {},
  load: async () => {
    try {
      const list = await api.managedListOpenVPNSessions()
      const next: Record<string, OpenVPNStatus> = {}
      for (const s of list) next[s.id] = s
      set({ sessions: next })
    } catch { /* service not reachable yet */ }
  },
  apply: (s) => set(state => ({ sessions: { ...state.sessions, [s.id]: s } })),
}))
