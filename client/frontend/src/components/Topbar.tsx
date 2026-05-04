import { useState, useRef, useEffect } from 'react'
import { Settings, Plus, MoreHorizontal, LogOut, LogIn, ListTree, Info } from 'lucide-react'
import { LogoMark } from './brand/LogoMark'
import { useTunnelStore } from '../stores/useTunnelStore'
import { useManagedStore } from '../stores/useManagedStore'
import { StatusDot } from './ui/StatusDot'
import { cn } from '../lib/cn'

interface TopbarProps {
  onImport: () => void
  onSettings: () => void
  onSignIn?: () => void
  onSignOut?: () => void
  onOpenConnections: () => void
  onOpenConfig: () => void
  configEnabled: boolean
}

export function Topbar({ onImport, onSettings, onSignIn, onSignOut, onOpenConnections, onOpenConfig, configEnabled }: TopbarProps) {
  const { tunnels, daemonOnline } = useTunnelStore()
  const { settings } = useManagedStore()
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  // Close menu on outside click
  useEffect(() => {
    if (!menuOpen) return
    const onClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenuOpen(false)
    }
    document.addEventListener('mousedown', onClick)
    return () => document.removeEventListener('mousedown', onClick)
  }, [menuOpen])

  const active = tunnels.find(t => t.status === 'connected') || tunnels.find(t => t.status === 'connecting')

  return (
    <div className="drag relative z-50 h-12 flex-shrink-0 flex items-center px-4 border-b border-border bg-background/80 backdrop-blur" style={{ paddingLeft: 'var(--titlebar-height)' }}>
      <div className="flex items-center gap-2 mr-4">
        <LogoMark size={20} className="text-primary shrink-0" />
        <p className="text-sm font-semibold tracking-tight">
          Pro<span className="text-primary">Identity</span>
          <span className="ml-1.5 text-[10px] uppercase tracking-[0.18em] text-muted-foreground font-medium">Access</span>
        </p>
      </div>

      {/* Active tunnel summary, center-left */}
      <div className="flex items-center gap-2 text-xs min-w-0">
        {active ? (
          <>
            <StatusDot status={active.status as any} pulse />
            <span className="text-muted-foreground">connected to</span>
            <span className="font-medium truncate">{active.name}</span>
          </>
        ) : (
          <>
            <StatusDot status="disconnected" />
            <span className="text-muted-foreground">no active connection</span>
          </>
        )}
      </div>

      {/* Right side */}
      <div className="ml-auto flex items-center gap-1.5 no-drag">
        <div className={cn(
          'flex items-center gap-1.5 text-[11px] mr-1.5',
          daemonOnline ? 'text-success' : 'text-destructive',
        )} title={daemonOnline ? 'Daemon connected' : 'Daemon offline'}>
          <span className={cn('w-1.5 h-1.5 rounded-full', daemonOnline ? 'bg-success' : 'bg-destructive')} />
          Daemon
        </div>

        <IconButton onClick={onOpenConfig} disabled={!configEnabled} label="Configuration">
          <Info className="w-4 h-4" />
        </IconButton>
        <IconButton onClick={onImport} label="Import .conf">
          <Plus className="w-4 h-4" />
        </IconButton>
        <IconButton onClick={onOpenConnections} label="Connections" highlight>
          <ListTree className="w-4 h-4" />
        </IconButton>

        <div className="relative" ref={menuRef}>
          <button
            onClick={() => setMenuOpen(o => !o)}
            aria-label="More"
            className="inline-flex items-center justify-center w-8 h-8 rounded-md text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors cursor-pointer"
          >
            <MoreHorizontal className="w-4 h-4" />
          </button>
          {menuOpen && (
            <div className="absolute top-full right-0 mt-1 w-52 rounded-md border border-border bg-popover shadow-xl py-1 z-50">
              <MenuItem icon={Settings} label="Settings" onClick={() => { setMenuOpen(false); onSettings() }} />
              {!settings.logged_in && onSignIn && (
                <MenuItem icon={LogIn} label="Sign in" onClick={() => { setMenuOpen(false); onSignIn() }} />
              )}
              {settings.logged_in && onSignOut && (
                <>
                  <div className="my-1 h-px bg-border" />
                  <MenuItem icon={LogOut} label={`Sign out (${settings.username})`} onClick={() => { setMenuOpen(false); onSignOut() }} destructive />
                </>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function MenuItem({
  icon: Icon, label, onClick, destructive,
}: { icon: React.ElementType; label: string; onClick: () => void; destructive?: boolean }) {
  return (
    <button
      onClick={onClick}
      className={cn(
        'w-full flex items-center gap-2 px-3 py-1.5 text-sm cursor-pointer transition-colors text-left',
        destructive ? 'text-destructive hover:bg-destructive/10' : 'text-foreground hover:bg-secondary',
      )}
    >
      <Icon className="w-3.5 h-3.5 shrink-0" />
      <span className="truncate">{label}</span>
    </button>
  )
}

function IconButton({
  onClick, disabled, label, highlight, children,
}: { onClick: () => void; disabled?: boolean; label: string; highlight?: boolean; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      title={label}
      aria-label={label}
      className={cn(
        'inline-flex items-center justify-center w-8 h-8 rounded-md transition-colors',
        disabled
          ? 'text-muted-foreground/40 cursor-not-allowed'
          : highlight
            ? 'text-primary hover:bg-primary/10 cursor-pointer'
            : 'text-muted-foreground hover:text-foreground hover:bg-secondary cursor-pointer',
      )}
    >
      {children}
    </button>
  )
}
