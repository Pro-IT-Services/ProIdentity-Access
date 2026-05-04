import { create } from 'zustand'

export interface ThroughputSample { ts: number; rxBps: number; txBps: number }

interface State {
  /** Last N samples per tunnel id. */
  history: Record<string, ThroughputSample[]>
  /** Last cumulative reading per tunnel id (for delta computation). */
  last: Record<string, { ts: number; rx: number; tx: number }>
  pushSample: (tunnelId: string, ts: number, rx: number, tx: number) => void
  reset: (tunnelId: string) => void
}

const MAX_SAMPLES = 60   // ~60 seconds of history at 1 Hz

export const useTrafficHistory = create<State>((set) => ({
  history: {},
  last: {},
  pushSample: (tunnelId, ts, rx, tx) =>
    set(state => {
      const prev = state.last[tunnelId]
      if (!prev) {
        // first sample for this tunnel — no delta possible yet
        return {
          history: { ...state.history, [tunnelId]: [] },
          last:    { ...state.last, [tunnelId]: { ts, rx, tx } },
        }
      }
      const dt = Math.max(1, (ts - prev.ts) / 1000)   // seconds, never zero
      const rxBps = Math.max(0, (rx - prev.rx) / dt)
      const txBps = Math.max(0, (tx - prev.tx) / dt)
      const arr = state.history[tunnelId] ?? []
      const next = arr.length >= MAX_SAMPLES
        ? [...arr.slice(arr.length - MAX_SAMPLES + 1), { ts, rxBps, txBps }]
        : [...arr, { ts, rxBps, txBps }]
      return {
        history: { ...state.history, [tunnelId]: next },
        last:    { ...state.last, [tunnelId]: { ts, rx, tx } },
      }
    }),
  reset: (tunnelId) =>
    set(state => {
      const { [tunnelId]: _h, ...history } = state.history
      const { [tunnelId]: _l, ...last } = state.last
      return { history, last }
    }),
}))
