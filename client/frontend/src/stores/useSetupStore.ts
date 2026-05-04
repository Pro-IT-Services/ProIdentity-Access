import { create } from 'zustand'
import * as api from '../wailsbridge'

type SetupStep = 'mode' | 'server' | 'register' | 'login' | 'done'
type AppMode = 'standalone' | 'managed' | null

interface SetupStore {
  // State
  setupDone: boolean
  step: SetupStep
  mode: AppMode
  serverURL: string
  deviceName: string
  loading: boolean
  error: string | null

  // Actions
  checkSetup: () => Promise<void>
  chooseMode: (mode: AppMode) => Promise<void>
  saveServerURL: (url: string) => void
  persistServerURL: () => Promise<void>
  setDeviceName: (name: string) => void
  ensureDeviceName: () => Promise<void>
  registerDevice: () => Promise<void>
  completeSetup: () => Promise<void>
  clearError: () => void
}

export const useSetupStore = create<SetupStore>((set, get) => ({
  setupDone: false,
  step: 'mode',
  mode: null,
  serverURL: '',
  deviceName: '',
  loading: false,
  error: null,

  checkSetup: async () => {
    try {
      const step = await api.managedGetSetupStep()
      if (step === 'done') {
        set({ setupDone: true, step: 'done' })
      } else {
        // Restore in-progress wizard state
        const mode: AppMode = step !== 'mode' ? 'managed' : null
        // Restore server URL from settings if we're past the mode step
        let serverURL = ''
        if (step !== 'mode') {
          const settings = await api.managedGetSettings()
          serverURL = settings.server_url ?? ''
        }
        set({ setupDone: false, step: step as SetupStep, mode, serverURL })
        if (step === 'register') {
          await get().ensureDeviceName()
        }
      }
    } catch {
      set({ setupDone: false })
    }
  },

  chooseMode: async (mode) => {
    set({ loading: true, error: null, mode })
    try {
      if (mode === 'standalone') {
        await api.managedSetMode('standalone')
        set({ loading: false, setupDone: true })
      } else {
        await api.managedSetMode('managed')
        set({ loading: false, step: 'server' })
      }
    } catch (e) {
      set({ loading: false, error: String(e) })
      throw e
    }
  },

  saveServerURL: (url) => {
    set({ serverURL: url })
  },

  // Called when advancing from the server URL step — persists to disk immediately
  persistServerURL: async () => {
    const { serverURL } = get()
    if (serverURL.trim()) {
      await api.managedSaveServerURL(serverURL)
    }
    await get().ensureDeviceName()
  },

  setDeviceName: (name) => {
    set({ deviceName: name })
  },

  ensureDeviceName: async () => {
    if (get().deviceName.trim()) return
    try {
      const name = await api.managedDefaultDeviceName()
      if (name.trim()) set({ deviceName: name })
    } catch {
      set({ deviceName: 'ProIdentity Desktop' })
    }
  },

  registerDevice: async () => {
    await get().ensureDeviceName()
    const { serverURL, deviceName } = get()
    set({ loading: true, error: null })
    try {
      await api.managedSaveServerURL(serverURL)
      await api.managedRegisterDevice(serverURL, deviceName)
      set({ loading: false, step: 'login' })
    } catch (e) {
      set({ loading: false, error: String(e) })
      throw e
    }
  },

  completeSetup: async () => {
    set({ loading: true, error: null })
    try {
      await api.managedCompleteSetup()
      set({ loading: false, setupDone: true })
    } catch (e) {
      set({ loading: false, error: String(e) })
      throw e
    }
  },

  clearError: () => set({ error: null }),
}))
