import { useState } from 'react'
import { Power, Globe, FileKey, Loader2, LogIn } from 'lucide-react'
import { useTunnelStore } from '../stores/useTunnelStore'
import { useManagedStore } from '../stores/useManagedStore'
import type { TunnelInfo, ServerStatus } from '../types'
import { Button } from './ui/Button'
import { StatusDot, type Status } from './ui/StatusDot'
import { MonoChip } from './ui/MonoChip'
import { managedDisconnectByTunnelID } from '../wailsbridge'
import { cn } from '../lib/cn'

import type { ServerInfo } from '../types'

type Row =
  | { key: string; kind: 'imported'; tunnel: TunnelInfo }
  | { key: string; kind: 'managed-active'; tunnel: TunnelInfo; server: ServerStatus }
  | { key: string; kind: 'managed-available'; server: ServerStatus }

interface Props {
  /** Fires after a successful connect — host can use it to close a parent sheet. */
  onConnected?: () => void
  /** Fires synchronously the moment the user clicks Connect (before the API
   *  call). Lets the host show optimistic state in the main UI immediately. */
  onConnectIntent?: (server: ServerInfo) => void
  /** Route managed-server connects through App (handles TOTP gate + timeout). */
  onConnectManaged?: (server: ServerInfo) => Promise<void>
  /** Open the login sheet — shown as a CTA when not logged in. */
  onSignIn?: () => void
}

export function ConnectionsList({ onConnected, onConnectIntent, onConnectManaged, onSignIn }: Props) {
  const { tunnels, selectedId, setSelected, connect, disconnect } = useTunnelStore()
  const { servers, settings, connectServer, loadServers } = useManagedStore()
  const [busyKey, setBusyKey] = useState<string | null>(null)

  const rows = buildRows(tunnels, servers)
  const activeTunnel = tunnels.find(t => t.status === 'connected' || t.status === 'connecting')

  const handleConnect = async (row: Row) => {
    setBusyKey(row.key)
    try {
      if (row.kind === 'managed-available') {
        onConnectIntent?.(row.server.server)
        if (onConnectManaged) {
          await onConnectManaged(row.server.server)
        } else {
          await connectServer(row.server.server)
        }
        onConnected?.()
      } else {
        await connect(row.tunnel.id)
        onConnected?.()
      }
    } catch (e: any) {
      console.warn('connect failed', e)
    } finally {
      setBusyKey(null)
    }
  }

  const handleDisconnect = async (row: Row) => {
    setBusyKey(row.key)
    try {
      if (row.kind === 'managed-active') {
        await managedDisconnectByTunnelID(row.tunnel.id)
        await loadServers()
      } else if (row.kind === 'imported') {
        await disconnect(row.tunnel.id)
      }
    } finally {
      setBusyKey(null)
    }
  }

  const showSignIn = !settings.logged_in && !!onSignIn

  if (rows.length === 0) {
    return (
      <div className="space-y-3">
        <p className="text-sm text-muted-foreground italic px-1">
          {showSignIn
            ? <>Sign in to a managed server to load available connections, or import a <span className="font-mono">.conf</span> file.</>
            : <>No connections yet. Import a <span className="font-mono">.conf</span> file or wait for managed servers to load.</>}
        </p>
        {showSignIn && (
          <Button variant="primary" size="sm" onClick={onSignIn}>
            <LogIn className="w-3.5 h-3.5" /> Sign in
          </Button>
        )}
      </div>
    )
  }

  return (
    <div className="space-y-3">
      {showSignIn && (
        <div className="flex items-center justify-between gap-3 px-3 py-2 rounded-lg border border-dashed border-border bg-secondary/20">
          <span className="text-xs text-muted-foreground">Sign in to load managed servers.</span>
          <Button variant="primary" size="sm" onClick={onSignIn}>
            <LogIn className="w-3.5 h-3.5" /> Sign in
          </Button>
        </div>
      )}
    <div className="rounded-xl border border-border bg-card overflow-hidden">
      <div className="px-4 py-2 border-b border-border bg-secondary/30 flex items-center justify-between">
        <span className="text-[11px] uppercase tracking-wider font-semibold text-muted-foreground">
          Available connections
        </span>
        <span className="text-[11px] text-muted-foreground">{rows.length} total</span>
      </div>
      {rows.map(row => {
        const meta = rowMeta(row)
        const isActive = !!activeTunnel
          && (row.kind === 'imported' && row.tunnel.id === activeTunnel.id
            || row.kind === 'managed-active' && row.tunnel.id === activeTunnel.id)
        const status: Status = isActive
          ? (activeTunnel!.status as any) || 'disconnected'
          : meta.status
        const showSelected = (row.kind === 'imported' || row.kind === 'managed-active') && row.tunnel.id === selectedId
        const busy = busyKey === row.key
        const otherActive = !!activeTunnel && !isActive
        // While busy, treat the row's effective status as "connecting" so the
        // status dot and label visibly change immediately on click — even before
        // the managed store flips its own connecting flag.
        const effectiveStatus: Status = busy && !isActive ? 'connecting' : status
        return (
          <div
            key={row.key}
            onClick={() => {
              if (row.kind !== 'managed-available') setSelected(row.tunnel.id)
            }}
            className={cn(
              'relative w-full grid grid-cols-[auto_1fr_auto] gap-3 items-center text-left px-4 py-2.5 border-b border-border last:border-0',
              'hover:bg-secondary/30 transition-colors cursor-pointer overflow-hidden',
              showSelected && 'bg-secondary/40',
              busy && 'bg-warning/5',
            )}
          >
            {/* Indeterminate progress sweep along the row's bottom edge — loud signal that something is happening. */}
            {busy && (
              <span aria-hidden="true" className="absolute left-0 right-0 bottom-0 h-0.5 overflow-hidden">
                <span className="absolute inset-y-0 w-1/3 bg-warning conn-row-progress" />
                <style>{`
                  .conn-row-progress {
                    animation: conn-row-progress-anim 1.1s linear infinite;
                  }
                  @keyframes conn-row-progress-anim {
                    0%   { transform: translateX(-100%); }
                    100% { transform: translateX(400%); }
                  }
                `}</style>
              </span>
            )}

            <span className={cn(
              'w-7 h-7 rounded-md flex items-center justify-center shrink-0 border transition-colors',
              isActive
                ? 'bg-success/10 border-success/30 text-success'
                : busy
                  ? 'bg-warning/10 border-warning/30 text-warning'
                  : 'bg-secondary/40 border-border text-muted-foreground',
            )}>
              {busy ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <meta.Icon className="w-3.5 h-3.5" />}
            </span>
            <div className="min-w-0">
              <div className="flex items-start gap-2">
                <p className="min-w-0 text-sm font-medium leading-tight break-words" title={meta.name}>{meta.name}</p>
                <span className="text-[10px] uppercase tracking-wider text-muted-foreground border border-border rounded px-1.5 py-0.5">
                  {meta.badge}
                </span>
              </div>
              <div className="flex items-center gap-2 mt-0.5">
                <StatusDot status={effectiveStatus} pulse={busy || effectiveStatus === 'connecting'} />
                <span className={cn(
                  'text-[11px] capitalize',
                  busy ? 'text-warning font-medium' : 'text-muted-foreground',
                )}>
                  {busy ? (isActive ? 'disconnecting…' : 'connecting…') : labelFor(effectiveStatus)}
                </span>
                {!busy && meta.sub && <MonoChip value={meta.sub} bare copy={false} className="text-[11px]" />}
              </div>
            </div>
            {isActive ? (
              <Button
                variant="ghost"
                size="sm"
                disabled={busy}
                onClick={(e) => { e.stopPropagation(); handleDisconnect(row) }}
                className={cn('min-w-[150px]', busy
                  ? 'text-warning bg-warning/10'
                  : 'text-destructive hover:bg-destructive/10')}
              >
                {busy
                  ? <><Loader2 className="w-4 h-4 animate-spin" /> Disconnecting…</>
                  : <><Power className="w-3.5 h-3.5" /> Disconnect</>}
              </Button>
            ) : (
              <Button
                variant={busy ? 'secondary' : 'primary'}
                size="sm"
                disabled={busy || otherActive}
                title={otherActive ? `Disconnect ${activeTunnel!.name} first` : undefined}
                onClick={(e) => { e.stopPropagation(); handleConnect(row) }}
                className={cn('min-w-[150px]', busy && 'bg-warning/15 text-warning')}
              >
                {busy
                  ? <><Loader2 className="w-4 h-4 animate-spin" /> Connecting…</>
                  : <><Power className="w-3.5 h-3.5" /> Connect</>}
              </Button>
            )}
          </div>
        )
      })}
    </div>
    </div>
  )
}

function buildRows(tunnels: TunnelInfo[], servers: ServerStatus[]): Row[] {
  const rows: Row[] = []
  const managedTunnelIds = new Set<string>()
  for (const ss of servers) {
    const t = resolveManagedTunnel(ss, tunnels, managedTunnelIds)
    if (t) {
      managedTunnelIds.add(t.id)
      rows.push({ key: `m-${ss.server.id}`, kind: 'managed-active', tunnel: t, server: ss })
      continue
    }
    rows.push({ key: `m-${ss.server.id}`, kind: 'managed-available', server: ss })
  }
  for (const t of tunnels) {
    if (managedTunnelIds.has(t.id)) continue
    if (t.is_managed) continue
    rows.push({ key: `t-${t.id}`, kind: 'imported', tunnel: t })
  }
  return rows
}

function resolveManagedTunnel(ss: ServerStatus, tunnels: TunnelInfo[], used: Set<string>): TunnelInfo | undefined {
  if (ss.tunnelId) {
    const t = tunnels.find(x => x.id === ss.tunnelId)
    if (t) return t
  }
  if (!ss.connected) return undefined

  const serverName = normalizeName(ss.server.name)
  const exactName = tunnels.find(t =>
    !used.has(t.id)
    && normalizeName(t.name) === serverName
    && (t.is_managed || t.status === 'connected' || t.status === 'connecting')
  )
  if (exactName) return exactName

  const activeManaged = tunnels.filter(t =>
    !used.has(t.id)
    && t.is_managed
    && (t.status === 'connected' || t.status === 'connecting')
  )
  if (activeManaged.length === 1) return activeManaged[0]

  const managed = tunnels.filter(t => !used.has(t.id) && t.is_managed)
  return managed.length === 1 ? managed[0] : undefined
}

function normalizeName(name: string): string {
  return name.trim().toLocaleLowerCase()
}

function rowMeta(row: Row): { name: string; badge: string; sub: string; status: Status; Icon: React.ElementType } {
  switch (row.kind) {
    case 'imported':
      return {
        name: row.tunnel.name,
        badge: 'imported',
        sub: row.tunnel.addresses[0] ?? '',
        status: row.tunnel.status as any,
        Icon: FileKey,
      }
    case 'managed-active':
      return {
        name: row.server.server.name,
        badge: 'managed',
        sub: row.tunnel.addresses[0] ?? row.server.server.subnet,
        status: row.tunnel.status as any,
        Icon: Globe,
      }
    case 'managed-available':
      return {
        name: row.server.server.name,
        badge: 'managed',
        sub: row.server.server.subnet,
        status: row.server.connecting ? 'connecting' : row.server.error ? 'error' : 'disconnected',
        Icon: Globe,
      }
  }
}

function labelFor(s: Status): string {
  switch (s) {
    case 'connected':    return 'connected'
    case 'connecting':   return 'connecting'
    case 'reconnecting': return 'reconnecting'
    case 'error':        return 'error'
    case 'disconnected': return 'available'
  }
}
