import { create } from 'zustand'
import type { TunnelInfo, StatsInfo } from '../types'
import * as api from '../bridge'

interface TunnelStore {
  tunnels: TunnelInfo[]
  stats: Record<string, StatsInfo>
  selectedId: string | null
  daemonOnline: boolean
  loading: boolean
  error: string | null

  // Actions
  setSelected: (id: string | null) => void
  refresh: () => Promise<void>
  import: (name: string, content: string) => Promise<void>
  delete: (id: string) => Promise<void>
  connect: (id: string) => Promise<void>
  disconnect: (id: string) => Promise<void>
  updateStats: (stats: StatsInfo) => void
  updateTunnel: (info: TunnelInfo) => void
  clearError: () => void
}

export const useTunnelStore = create<TunnelStore>((set, get) => ({
  tunnels: [],
  stats: {},
  selectedId: null,
  daemonOnline: true, // always true on Android
  loading: false,
  error: null,

  setSelected: (id) => set({ selectedId: id }),

  refresh: async () => {
    set({ loading: true, error: null })
    try {
      const tunnels = await api.listTunnels().catch(() => [])
      set({ daemonOnline: true, tunnels, loading: false })

      // Auto-select first tunnel if nothing is selected
      const { selectedId } = get()
      if (!selectedId && tunnels.length > 0) {
        set({ selectedId: tunnels[0].id })
      }
    } catch (e) {
      set({ loading: false, error: String(e) })
    }
  },

  import: async (name, content) => {
    set({ error: null })
    try {
      const t = await api.importTunnel(name, content)
      if (!t || !t.id) throw new Error('Bridge returned empty response')
      set(s => ({
        tunnels: [...s.tunnels, t],
        selectedId: t.id,
      }))
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e)
      set({ error: msg })
      throw new Error(msg)
    }
  },

  delete: async (id) => {
    set({ error: null })
    try {
      await api.deleteTunnel(id)
      set(s => {
        const tunnels = s.tunnels.filter(t => t.id !== id)
        const selectedId = s.selectedId === id
          ? (tunnels[0]?.id ?? null)
          : s.selectedId
        return { tunnels, selectedId }
      })
    } catch (e) {
      set({ error: String(e) })
      throw e
    }
  },

  connect: async (id) => {
    // Optimistic update
    set(s => ({
      tunnels: s.tunnels.map(t =>
        t.id === id ? { ...t, status: 'connecting' as const } : t
      ),
    }))
    try {
      await api.connectTunnel(id)
    } catch (e) {
      set(s => ({
        error: String(e),
        tunnels: s.tunnels.map(t =>
          t.id === id ? { ...t, status: 'error' as const, error: String(e) } : t
        ),
      }))
      throw e
    }
  },

  disconnect: async (id) => {
    try {
      await api.disconnectTunnel(id)
      set(s => ({
        tunnels: s.tunnels.map(t =>
          t.id === id ? { ...t, status: 'disconnected' as const } : t
        ),
      }))
    } catch (e) {
      set({ error: String(e) })
      throw e
    }
  },

  updateStats: (stats) => {
    set(s => ({ stats: { ...s.stats, [stats.tunnel_id]: stats } }))
  },

  updateTunnel: (info) => {
    set(s => ({
      tunnels: s.tunnels.map(t => (t.id === info.id ? info : t)),
    }))
  },

  clearError: () => set({ error: null }),
}))
