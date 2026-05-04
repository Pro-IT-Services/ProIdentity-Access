import { create } from 'zustand'
import type { User } from '../api/client'

interface AuthState {
  token: string | null
  user: User | null
  ready: boolean
  setAuth: (token: string, user: User) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  token: sessionStorage.getItem('wg_token'),
  user: null,
  ready: !sessionStorage.getItem('wg_token'),
  setAuth: (token, user) => {
    sessionStorage.setItem('wg_token', token)
    set({ token, user, ready: true })
  },
  logout: () => {
    sessionStorage.removeItem('wg_token')
    set({ token: null, user: null, ready: true })
  },
}))

/** Permission helper. is_admin users implicitly hold every permission. */
export function useHasPerm(perm: string): boolean {
  const user = useAuthStore(s => s.user)
  if (!user) return false
  if (user.is_admin) return true
  return user.permissions?.includes(perm) ?? false
}

/** True if any of the supplied perms is held — useful for area gates. */
export function useHasAnyPerm(...perms: string[]): boolean {
  const user = useAuthStore(s => s.user)
  if (!user) return false
  if (user.is_admin) return true
  return perms.some(p => user.permissions?.includes(p))
}
