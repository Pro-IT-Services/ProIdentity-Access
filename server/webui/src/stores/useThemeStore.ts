import { create } from 'zustand'

export type Theme = 'light' | 'dark' | 'system'

function applyTheme(t: Theme) {
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
  const dark = t === 'dark' || (t === 'system' && prefersDark)
  document.documentElement.classList.toggle('dark', dark)
}

function initialTheme(): Theme {
  const saved = (typeof localStorage !== 'undefined' && localStorage.getItem('theme')) as Theme | null
  if (saved === 'light' || saved === 'dark' || saved === 'system') return saved
  return 'system'
}

interface ThemeState {
  theme: Theme
  setTheme: (t: Theme) => void
  toggle: () => void
}

export const useThemeStore = create<ThemeState>((set, get) => ({
  theme: initialTheme(),
  setTheme: (t) => {
    localStorage.setItem('theme', t)
    applyTheme(t)
    set({ theme: t })
  },
  toggle: () => {
    const cur = get().theme
    // light -> dark -> system -> light
    const next: Theme = cur === 'light' ? 'dark' : cur === 'dark' ? 'system' : 'light'
    get().setTheme(next)
  },
}))

// Re-apply on system preference change when in 'system' mode
if (typeof window !== 'undefined' && window.matchMedia) {
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (useThemeStore.getState().theme === 'system') applyTheme('system')
  })
}
