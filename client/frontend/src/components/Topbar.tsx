import { useState, useRef, useEffect } from 'react'
import { Settings, Plus, MoreHorizontal, LogOut, LogIn, ListTree, Info } from 'lucide-react'
import { LogoMark } from './brand/LogoMark'
import { useTunnelStore } from '../stores/useTunnelStore'
import { useManagedStore } from '../stores/useManagedStore'
import { useActiveConnections } from '../lib/activeConnections'
import { StatusDot } from './ui/StatusDot'
import { cn } from '../lib/cn'
import { t, tp } from '../i18n'

interface TopbarProps {
  onImport: () => void
  /** Server policy: show the Import action. */
  canImport?: boolean
  onSettings: () => void
  onSignIn?: () => void
  onSignOut?: () => void
  onOpenConnections: () => void
  onOpenConfig: () => void
  configEnabled: boolean
}

export function Topbar({ onImport, canImport = true, onSettings, onSignIn, onSignOut, onOpenConnections, onOpenConfig, configEnabled }: TopbarProps) {
  const { daemonOnline } = useTunnelStore()
  const conns = useActiveConnections()
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

  // WireGuard and OpenVPN alike.
  const active = conns.find(c => c.status === 'connected') || conns[0]
  const others = conns.length - 1

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
            <StatusDot status={active.status} pulse />
            <span className="text-muted-foreground">{active.status === 'connected' ? t('topbar.connectedTo') : t('topbar.connectingTo')}</span>
            <span className="font-medium truncate">{active.name}</span>
            {others > 0 && <span className="text-muted-foreground shrink-0">{tp('topbar.moreConnections', others)}</span>}
          </>
        ) : (
          <>
            <StatusDot status="disconnected" />
            <span className="text-muted-foreground">{t('topbar.noActive')}</span>
          </>
        )}
      </div>

      {/* Right side */}
      <div className="ml-auto flex items-center gap-1.5 no-drag">
        <div className={cn(
          'flex items-center gap-1.5 text-[11px] mr-1.5',
          daemonOnline ? 'text-success' : 'text-destructive',
        )} title={daemonOnline ? t('topbar.daemonConnected') : t('topbar.daemonOffline')}>
          <span className={cn('w-1.5 h-1.5 rounded-full', daemonOnline ? 'bg-success' : 'bg-destructive')} />
          {t('topbar.daemon')}
        </div>

        <TextButton onClick={onOpenConnections} icon={ListTree} label={t('common.connections')} highlight />
        {canImport && <TextButton onClick={onImport} icon={Plus} label={t('topbar.import')} />}
        <TextButton onClick={onSettings} icon={Settings} label={t('common.settings')} />

        <div className="relative" ref={menuRef}>
          <button
            onClick={() => setMenuOpen(o => !o)}
            aria-label={t('common.more')}
            className="inline-flex items-center justify-center w-8 h-8 rounded-md text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors cursor-pointer"
          >
            <MoreHorizontal className="w-4 h-4" />
          </button>
          {menuOpen && (
            <div className="absolute top-full right-0 mt-1 w-56 rounded-md border border-border bg-popover shadow-xl py-1 z-50">
              <MenuItem icon={Info} label={t('common.configuration')} disabled={!configEnabled} onClick={() => { setMenuOpen(false); onOpenConfig() }} />
              {!settings.logged_in && onSignIn && (
                <MenuItem icon={LogIn} label={t('common.signIn')} onClick={() => { setMenuOpen(false); onSignIn() }} />
              )}
              {settings.logged_in && onSignOut && (
                <>
                  <div className="my-1 h-px bg-border" />
                  <MenuItem icon={LogOut} label={t('topbar.signOutAs', { username: settings.username })} onClick={() => { setMenuOpen(false); onSignOut() }} destructive />
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
  icon: Icon, label, onClick, destructive, disabled,
}: { icon: React.ElementType; label: string; onClick: () => void; destructive?: boolean; disabled?: boolean }) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={cn(
        'w-full flex items-center gap-2 px-3 py-1.5 text-sm transition-colors text-left',
        disabled ? 'text-muted-foreground/40 cursor-not-allowed'
          : destructive ? 'text-destructive hover:bg-destructive/10 cursor-pointer'
          : 'text-foreground hover:bg-secondary cursor-pointer',
      )}
    >
      <Icon className="w-3.5 h-3.5 shrink-0" />
      <span className="truncate">{label}</span>
    </button>
  )
}

/** Labeled topbar action (icon + text) so the controls read clearly for every user. */
function TextButton({
  onClick, icon: Icon, label, highlight,
}: { onClick: () => void; icon: React.ElementType; label: string; highlight?: boolean }) {
  return (
    <button
      onClick={onClick}
      aria-label={label}
      className={cn(
        'inline-flex items-center gap-1.5 h-8 px-2.5 rounded-md text-[13px] font-medium transition-colors cursor-pointer',
        highlight
          ? 'text-primary hover:bg-primary/10'
          : 'text-muted-foreground hover:text-foreground hover:bg-secondary',
      )}
    >
      <Icon className="w-4 h-4 shrink-0" />
      <span>{label}</span>
    </button>
  )
}
