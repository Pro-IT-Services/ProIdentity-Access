import { create } from 'zustand'
import type { ManagedSettings, ServerInfo, ServerStatus } from '../types'
import * as api from '../bridge'

interface ManagedStore {
  settings: ManagedSettings
  servers: ServerStatus[]
  loading: boolean
  error: string | null

  // Actions
  loadSettings: () => Promise<void>
  loadServers: () => Promise<void>
  saveServerURL: (url: string) => Promise<void>
  login: (username: string, password: string, totpCode: string) => Promise<{ requireTOTP: boolean }>
  logout: () => Promise<void>
  connectServer: (server: ServerInfo, totpCode?: string) => Promise<void>
  disconnectServer: (serverID: string) => Promise<void>
  clearError: () => void
}

export const useManagedStore = create<ManagedStore>((set, _get) => ({
  settings: { server_url: '', username: '', is_admin: false, logged_in: false, vpn_name: '', totp_enabled: false },
  servers: [],
  loading: false,
  error: null,

  loadSettings: async () => {
    try {
      const settings = await api.managedGetSettings()
      set({ settings })
    } catch {
      // not critical
    }
  },

  loadServers: async () => {
    try {
      const [serverList, activeIDs] = await Promise.all([
        api.managedListServers(),
        api.managedActiveServers(),
      ])
      const activeSet = new Set(activeIDs ?? [])
      const servers: ServerStatus[] = (serverList ?? []).map(s => ({
        server: s,
        connected: activeSet.has(s.id),
        tunnelId: null,
        connecting: false,
        error: null,
      }))
      set({ servers })
    } catch (e) {
      set({ error: String(e) })
    }
  },

  saveServerURL: async (url) => {
    set({ error: null })
    try {
      await api.managedSaveServerURL(url)
      set(s => ({ settings: { ...s.settings, server_url: url } }))
    } catch (e) {
      set({ error: String(e) })
      throw e
    }
  },

  login: async (username, password, totpCode) => {
    set({ loading: true, error: null })
    try {
      const result = await api.managedLogin(username, password, totpCode)
      if (result.require_totp) {
        set({ loading: false })
        return { requireTOTP: true }
      }
      set(s => ({
        loading: false,
        settings: {
          ...s.settings,
          username: result.username,
          is_admin: result.is_admin,
          logged_in: true,
          totp_enabled: result.totp_enabled,
        },
      }))
      return { requireTOTP: false }
    } catch (e) {
      set({ loading: false, error: String(e) })
      throw e
    }
  },

  logout: async () => {
    set({ loading: true, error: null })
    try {
      await api.managedLogout()
      set(s => ({
        loading: false,
        servers: [],
        settings: { ...s.settings, username: '', is_admin: false, logged_in: false },
      }))
    } catch (e) {
      set({ loading: false, error: String(e) })
    }
  },

  connectServer: async (server, totpCode = '') => {
    // Mark as connecting
    set(s => ({
      servers: s.servers.map(ss =>
        ss.server.id === server.id ? { ...ss, connecting: true, error: null } : ss
      ),
    }))
    try {
      const tunnel = await api.managedConnectServer(server.id, server.name, totpCode)
      set(s => ({
        servers: s.servers.map(ss =>
          ss.server.id === server.id
            ? { ...ss, connecting: false, connected: true, tunnelId: tunnel.id }
            : ss
        ),
      }))
    } catch (e) {
      set(s => ({
        servers: s.servers.map(ss =>
          ss.server.id === server.id
            ? { ...ss, connecting: false, error: String(e) }
            : ss
        ),
      }))
      throw e
    }
  },

  disconnectServer: async (serverID) => {
    set(s => ({
      servers: s.servers.map(ss =>
        ss.server.id === serverID ? { ...ss, connecting: true, error: null } : ss
      ),
    }))
    try {
      await api.managedDisconnectServer(serverID)
      set(s => ({
        servers: s.servers.map(ss =>
          ss.server.id === serverID
            ? { ...ss, connecting: false, connected: false, tunnelId: null }
            : ss
        ),
      }))
    } catch (e) {
      set(s => ({
        servers: s.servers.map(ss =>
          ss.server.id === serverID
            ? { ...ss, connecting: false, error: String(e) }
            : ss
        ),
      }))
      throw e
    }
  },

  clearError: () => set({ error: null }),
}))
