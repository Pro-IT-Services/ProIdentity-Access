import { create } from 'zustand'
import * as api from '../wailsbridge'
import type { UpdateState } from '../wailsbridge'
import { t } from '../i18n'

/** "Later" hides the prompt for this long, per version (not for mandatory updates). */
const SNOOZE_MS = 24 * 60 * 60 * 1000
const SNOOZE_KEY = 'proidentity.update.snooze'

function readSnooze(): { version: string; until: number } | null {
  try {
    const raw = localStorage.getItem(SNOOZE_KEY)
    return raw ? JSON.parse(raw) : null
  } catch {
    return null
  }
}

interface UpdateStore {
  status: UpdateState | null
  checking: boolean
  installing: boolean
  error: string
  /** Prompt dismissed for this session (re-shown on the next periodic check). */
  dismissed: boolean

  check: (opts?: { silent?: boolean }) => Promise<void>
  install: () => Promise<void>
  later: () => void
  applyEvent: (st: UpdateState) => void
  shouldPrompt: () => boolean
}

export const useUpdateStore = create<UpdateStore>((set, get) => ({
  status: null,
  checking: false,
  installing: false,
  error: '',
  dismissed: false,

  check: async ({ silent = false } = {}) => {
    if (get().checking) return
    set({ checking: true, error: silent ? get().error : '' })
    try {
      const status = await api.checkForUpdate()
      set({ status, dismissed: false })
    } catch (e: any) {
      if (!silent) set({ error: String(e?.message ?? e) })
    } finally {
      set({ checking: false })
    }
  },

  install: async () => {
    set({ installing: true, error: '' })
    try {
      await api.installUpdate()
    } catch (e: any) {
      set({ installing: false, error: String(e?.message ?? e) })
    }
  },

  later: () => {
    const v = get().status?.latest_version
    if (v) {
      try { localStorage.setItem(SNOOZE_KEY, JSON.stringify({ version: v, until: Date.now() + SNOOZE_MS })) } catch { /* private mode */ }
      // Tell the service too, so it doesn't reopen the app to offer this version.
      api.snoozeUpdate(v).catch(() => {})
    }
    set({ dismissed: true })
  },

  applyEvent: (status) => {
    set({
      status,
      installing: status.state === 'downloading' || status.state === 'installing',
      error: status.state === 'failed' ? (status.error ?? t('update.failed')) : '',
    })
  },

  shouldPrompt: () => {
    const { status, dismissed, installing } = get()
    if (!status) return false
    if (installing || ['downloading', 'installing'].includes(status.state)) return true
    if (status.state === 'failed') return !dismissed
    if (status.state !== 'available') return false
    if (status.mandatory) return true
    // "Later" is a 24 h snooze per version (also held by the service), so a
    // long-running app prompts again once it expires.
    const snooze = readSnooze()
    return !(snooze && snooze.version === status.latest_version && snooze.until > Date.now())
  },
}))
