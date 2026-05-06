import { Outlet, NavLink, useNavigate, Link } from 'react-router-dom'
import { useAuthStore } from '../stores/useAuthStore'
import {
  LayoutDashboard, Globe, Settings, LogOut,
  Activity, User, Users, Network, History,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { ThemeToggle } from './ThemeToggle'

interface NavGroup {
  label?: string
  items: NavItemDef[]
}
interface NavItemDef {
  to: string
  label: string
  icon: React.ElementType
  exact?: boolean
  /** Show if the user holds ANY of these permissions (or is_admin). */
  anyPerm?: string[]
}

const NAV: NavGroup[] = [
  {
    items: [
      { to: '/', label: 'Overview', icon: LayoutDashboard, exact: true },
      { to: '/sessions', label: 'My Sessions', icon: Activity },
    ],
  },
  {
    label: 'Manage',
    items: [
      { to: '/access', label: 'Access', icon: Users, anyPerm: ['users.manage', 'resources.manage'] },
      { to: '/servers', label: 'Servers', icon: Globe, anyPerm: ['servers.manage'] },
      { to: '/topology', label: 'Topology', icon: Network, anyPerm: ['topology.read'] },
      { to: '/connection-history', label: 'Connection History', icon: History, anyPerm: ['sessions.manage'] },
      { to: '/system', label: 'System', icon: Settings, anyPerm: ['system.settings', 'roles.manage', 'audit.read', 'denials.read', 'diagnostics.read'] },
    ],
  },
  {
    label: 'Account',
    items: [
      { to: '/profile', label: 'Profile & 2FA', icon: User },
    ],
  },
]

function visible(user: { is_admin: boolean; permissions?: string[] } | null, item: NavItemDef): boolean {
  if (!item.anyPerm) return true
  if (!user) return false
  if (user.is_admin) return true
  return item.anyPerm.some(p => user.permissions?.includes(p))
}

function NavItem({ to, label, icon: Icon, exact }: { to: string; label: string; icon: React.ElementType; exact?: boolean }) {
  return (
    <NavLink
      to={to}
      end={exact}
      className={({ isActive }) =>
        cn(
          'group flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors duration-150 cursor-pointer',
          isActive
            ? 'bg-primary/15 text-primary font-medium'
            : 'text-muted-foreground hover:text-foreground hover:bg-secondary'
        )
      }
    >
      {({ isActive }) => (
        <>
          <Icon className={cn('w-4 h-4 shrink-0', isActive ? 'text-primary' : 'text-muted-foreground group-hover:text-foreground')} />
          <span className="flex-1">{label}</span>
        </>
      )}
    </NavLink>
  )
}

export default function Layout() {
  const { user, logout } = useAuthStore()
  const navigate = useNavigate()
  const handleLogout = () => { logout(); navigate('/login') }

  return (
    <div className="flex h-screen overflow-hidden bg-background text-foreground">
      <aside className="w-60 flex-shrink-0 flex flex-col border-r border-border bg-card/40">
        <Link to="/" className="px-4 py-4 border-b border-border flex items-center gap-3 hover:bg-secondary/40 transition-colors">
          <div className="w-9 h-9 rounded-lg bg-primary/15 border border-primary/25 flex items-center justify-center text-primary">
            <svg width="18" height="18" viewBox="0 0 256 256" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round">
              <polygon points="178,128 153,171.3 103,171.3 78,128 103,84.7 153,84.7" strokeWidth="8"/>
              <line x1="178" y1="128" x2="192.2" y2="199.3" strokeWidth="6"/>
              <line x1="153" y1="171.3" x2="98.3" y2="219.3" strokeWidth="6"/>
              <line x1="103" y1="171.3" x2="34.1" y2="148" strokeWidth="6"/>
              <line x1="78" y1="128" x2="63.8" y2="56.7" strokeWidth="6"/>
              <line x1="103" y1="84.7" x2="157.7" y2="36.7" strokeWidth="6"/>
              <line x1="153" y1="84.7" x2="221.9" y2="108" strokeWidth="6"/>
              <circle cx="128" cy="128" r="11" fill="currentColor"/>
            </svg>
          </div>
          <div className="leading-tight">
            <p className="text-sm font-semibold">ProIdentity Access</p>
          </div>
        </Link>

        <nav className="flex-1 overflow-y-auto px-3 py-3 scrollbar-thin">
          {NAV.map((group, gi) => {
            const items = group.items.filter(it => visible(user, it))
            if (items.length === 0) return null
            return (
              <div key={gi} className={gi > 0 ? 'mt-4' : ''}>
                {group.label && (
                  <p className="px-3 mb-1 text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/70">
                    {group.label}
                  </p>
                )}
                <div className="space-y-0.5">
                  {items.map(it => <NavItem key={it.to} {...it} />)}
                </div>
              </div>
            )
          })}
        </nav>

        <div className="border-t border-border p-3">
          <div className="flex items-center gap-2 px-2 py-2 rounded-md">
            <div className="w-8 h-8 rounded-full bg-primary/15 border border-primary/25 flex items-center justify-center shrink-0">
              <span className="text-xs font-bold text-primary">{user?.username?.[0]?.toUpperCase()}</span>
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-xs font-semibold truncate">{user?.username}</p>
              <p className="text-[10px] text-muted-foreground">{user?.is_admin ? 'Administrator' : 'User'}</p>
            </div>
            <ThemeToggle />
            <button
              onClick={handleLogout}
              aria-label="Sign out"
              className="h-8 w-8 inline-flex items-center justify-center rounded-md text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors cursor-pointer"
            >
              <LogOut className="w-4 h-4" />
            </button>
          </div>
        </div>
      </aside>

      <div className="flex-1 flex flex-col overflow-hidden">
        <main className="flex-1 overflow-y-auto scrollbar-thin">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
