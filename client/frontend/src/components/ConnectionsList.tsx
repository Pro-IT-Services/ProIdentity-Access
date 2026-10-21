import { useCallback, useEffect, useState } from 'react'
import { Building2, FileKey, LogIn, Plus, Search, Users } from 'lucide-react'
import { useTunnelStore } from '../stores/useTunnelStore'
import { useManagedStore } from '../stores/useManagedStore'
import type { TunnelInfo, ServerStatus, ServerInfo } from '../types'
import { Button } from './ui/Button'
import type { Status } from './ui/StatusDot'
import { managedDisconnectByTunnelID, managedListWireGuardProfiles, type WGProfileView } from '../wailsbridge'
import { ConnRow, ConnGroupCard, ProtocolHeader, RowTag } from './ConnRow'
import { toast } from './ui/Toast'
import { useOpenVPN, OpenVPNRow, OpenVPNImport, ovpnSessionId } from './OpenVPNPanel'
import { t, tNode } from '../i18n'

type WgRow =
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
  /** Open the WireGuard import sheet. */
  onImportWireGuard?: () => void
}

/**
 * Every connection, grouped by where it comes from: "ProIdentity Access"
 * (assigned by the organization) and "Imported" (added on this device), each
 * split by protocol. Connections run side by side; the daemon refuses only
 * ones whose networks overlap, and says why.
 */
export function ConnectionsList({ onConnected, onConnectIntent, onConnectManaged, onSignIn, onImportWireGuard }: Props) {
  const { tunnels, selectedId, setSelected, connect, disconnect } = useTunnelStore()
  const { servers, settings, connectServer, loadServers } = useManagedStore()
  const canImport = settings.allow_personal_import !== false
  const ovpn = useOpenVPN()
  const wgp = useWGProfiles(settings.logged_in)
  const [busyKey, setBusyKey] = useState<string | null>(null)
  const [query, setQuery] = useState('')

  const wgRows = buildRows(tunnels, servers)
  const q = query.trim().toLocaleLowerCase()
  const match = (name: string) => !q || name.toLocaleLowerCase().includes(q)

  const accessWg = wgRows.filter(r => r.kind !== 'imported' && match(rowName(r)))
  const importedWg = wgRows.filter(r => r.kind === 'imported' && match(rowName(r)))
  const accessWgp = wgp.profiles.filter(p => match(p.name))
  const accessOvpn = ovpn.profiles.filter(p => p.source === 'assigned' && match(p.name))
  const importedOvpn = ovpn.profiles.filter(p => p.source === 'local' && match(p.name))
  const total = wgRows.length + wgp.profiles.length + ovpn.profiles.length
  const shown = accessWg.length + accessWgp.length + importedWg.length + accessOvpn.length + importedOvpn.length

  const handleConnect = async (row: WgRow) => {
    setBusyKey(row.key)
    try {
      if (row.kind === 'managed-available') {
        onConnectIntent?.(row.server.server)
        if (onConnectManaged) await onConnectManaged(row.server.server)
        else await connectServer(row.server.server)
      } else {
        await connect(row.tunnel.id)
      }
      onConnected?.()
    } catch (e: any) {
      console.warn('connect failed', e)
    } finally {
      setBusyKey(null)
    }
  }

  const handleDisconnect = async (row: WgRow) => {
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

  const renderWg = (row: WgRow) => {
    const busy = busyKey === row.key
    const status = rowStatus(row)
    const live = status === 'connected' || status === 'connecting'
    const tunnelId = row.kind === 'managed-available' ? null : row.tunnel.id
    return (
      <ConnRow
        key={row.key}
        name={rowName(row)}
        icon={row.kind === 'imported' ? FileKey : Building2}
        status={busy && !live ? 'connecting' : status}
        sub={rowSub(row)}
        busy={busy}
        live={live}
        selected={!!tunnelId && tunnelId === selectedId}
        onSelect={tunnelId ? () => setSelected(tunnelId) : undefined}
        onConnect={() => handleConnect(row)}
        onDisconnect={() => handleDisconnect(row)}
      />
    )
  }

  // An assigned WireGuard profile: one device at a time, so a profile someone
  // else has up shows who and can't be connected.
  const renderWgp = (p: WGProfileView) => {
    const busy = busyKey === p.id
    const tunnel = tunnels.find(x => x.id === p.id)
    const status: Status = tunnel?.status === 'connected' || tunnel?.status === 'connecting' || tunnel?.status === 'error'
      ? tunnel.status : 'disconnected'
    const live = status === 'connected' || status === 'connecting'
    const holder = p.in_use_by && !p.in_use_by.this_device && !live ? p.in_use_by : undefined
    const inUse = holder
      ? holder.you
        ? t('conns.inUseByYou', { device: holder.device || '?' })
        : t('conns.inUseBy', { name: holder.name, device: holder.device || '?' })
      : undefined
    const run = async (fn: () => Promise<void>, closeAfter: boolean) => {
      setBusyKey(p.id)
      try {
        await fn()
        if (closeAfter) onConnected?.()
      } catch (e: any) {
        toast(`${p.name}: ${String(e?.message ?? e)}`, 'warning', 6000)
      } finally {
        setBusyKey(null)
        wgp.reload()
      }
    }
    return (
      <ConnRow
        key={p.id}
        name={p.name}
        icon={Building2}
        status={busy && !live ? 'connecting' : status}
        statusText={inUse}
        connectBlocked={inUse}
        sub={inUse ? undefined : p.address}
        tags={p.shared ? <RowTag><Users className="w-3 h-3" /> {t('conns.badgeShared')}</RowTag> : undefined}
        busy={busy}
        live={live}
        selected={!!tunnel && tunnel.id === selectedId}
        onSelect={tunnel ? () => setSelected(tunnel.id) : undefined}
        onConnect={() => run(() => connect(p.id), true)}
        onDisconnect={() => run(() => disconnect(p.id), false)}
      />
    )
  }

  const renderOvpn = (p: (typeof ovpn.profiles)[number]) => {
    const id = ovpnSessionId(p)
    return (
      <OpenVPNRow
        key={id}
        profile={p}
        session={ovpn.sessions[id]}
        formOpen={ovpn.openForm === id}
        onToggleForm={() => ovpn.setOpenForm(cur => (cur === id ? null : id))}
        onFormDone={() => ovpn.setOpenForm(null)}
        onDeleted={ovpn.reload}
      />
    )
  }

  const showSignIn = !settings.logged_in && !!onSignIn
  const hasAccess = accessWg.length + accessWgp.length + accessOvpn.length > 0
  const hasImported = importedWg.length + importedOvpn.length > 0

  if (total === 0 && ovpn.loaded) {
    return (
      <div className="space-y-3">
        <p className="text-sm text-muted-foreground px-1">
          {showSignIn
            ? tNode('conns.signInHint', { file: <span key="f" className="font-mono">.conf</span> })
            : tNode('conns.empty', { file: <span key="f" className="font-mono">.conf</span> })}
        </p>
        {showSignIn && (
          <Button variant="primary" size="sm" onClick={onSignIn}>
            <LogIn className="w-3.5 h-3.5" /> {t('common.signIn')}
          </Button>
        )}
        {canImport && <ImportLinks onImportWireGuard={onImportWireGuard} onImported={ovpn.reload} />}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {total > 6 && (
        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
          <input
            value={query} onChange={e => setQuery(e.target.value)}
            placeholder={t('conns.search')} aria-label={t('conns.search')}
            className="w-full h-9 rounded-lg border border-border bg-background pl-8 pr-2 text-sm"
          />
        </div>
      )}

      {(hasAccess || showSignIn) && (
        <section className="space-y-2.5">
          <GroupHeading icon={Building2} title={t('conns.groupAccess')} desc={t('conns.groupAccessDesc')} />
          {showSignIn && (
            <div className="flex items-center justify-between gap-3 px-3 py-2 rounded-lg border border-dashed border-border bg-secondary/20">
              <span className="text-xs text-muted-foreground">{t('conns.signInBanner')}</span>
              <Button variant="primary" size="sm" onClick={onSignIn}>
                <LogIn className="w-3.5 h-3.5" /> {t('common.signIn')}
              </Button>
            </div>
          )}
          {hasAccess && (
            <ConnGroupCard>
              {accessWg.length + accessWgp.length > 0 && <>
                <ProtocolHeader label="WireGuard" count={accessWg.length + accessWgp.length} />
                {accessWg.map(renderWg)}
                {accessWgp.map(renderWgp)}
              </>}
              {accessOvpn.length > 0 && <>
                <ProtocolHeader label="OpenVPN" count={accessOvpn.length} />
                {accessOvpn.map(renderOvpn)}
              </>}
            </ConnGroupCard>
          )}
        </section>
      )}

      {(hasImported || canImport) && (
        <section className="space-y-2.5">
          <GroupHeading icon={FileKey} title={t('conns.groupImported')} desc={t('conns.groupImportedDesc')} />
          {hasImported ? (
            <ConnGroupCard>
              {importedWg.length > 0 && <>
                <ProtocolHeader label="WireGuard" count={importedWg.length} />
                {importedWg.map(renderWg)}
              </>}
              {importedOvpn.length > 0 && <>
                <ProtocolHeader label="OpenVPN" count={importedOvpn.length} />
                {importedOvpn.map(renderOvpn)}
              </>}
            </ConnGroupCard>
          ) : !q && (
            <p className="text-xs text-muted-foreground px-1">{t('conns.importedEmpty')}</p>
          )}
          {canImport && <ImportLinks onImportWireGuard={onImportWireGuard} onImported={ovpn.reload} />}
        </section>
      )}

      {q && shown === 0 && (
        <p className="text-sm text-muted-foreground text-center py-4">{t('ovpn.noMatches')}</p>
      )}
    </div>
  )
}

function ImportLinks({ onImportWireGuard, onImported }: { onImportWireGuard?: () => void; onImported: () => void }) {
  return (
    <div className="flex flex-wrap items-center justify-end gap-x-4 gap-y-1">
      {onImportWireGuard && (
        <button onClick={onImportWireGuard}
          className="inline-flex items-center gap-1 text-[11px] text-muted-foreground hover:text-foreground transition-colors">
          <Plus className="w-3 h-3" /> {t('conns.importConf')}
        </button>
      )}
      <OpenVPNImport onImported={onImported} />
    </div>
  )
}

function GroupHeading({ icon: Icon, title, desc }: { icon: React.ElementType; title: string; desc: string }) {
  return (
    <div className="flex items-start gap-2.5 px-1">
      <Icon className="w-4 h-4 mt-0.5 text-primary shrink-0" />
      <div className="min-w-0">
        <h3 className="text-sm font-semibold leading-tight">{title}</h3>
        <p className="text-xs text-muted-foreground mt-0.5">{desc}</p>
      </div>
    </div>
  )
}

/** Assigned WireGuard profiles, refreshed while the sheet is open so "in use by" stays current. */
function useWGProfiles(loggedIn: boolean) {
  const [profiles, setProfiles] = useState<WGProfileView[]>([])
  const reload = useCallback(() => {
    if (!loggedIn) { setProfiles([]); return }
    managedListWireGuardProfiles().then(setProfiles).catch(() => {})
  }, [loggedIn])
  useEffect(() => {
    reload()
    const timer = setInterval(reload, 15000)
    return () => clearInterval(timer)
  }, [reload])
  return { profiles, reload }
}

function buildRows(allTunnels: TunnelInfo[], servers: ServerStatus[]): WgRow[] {
  // Assigned profiles ("wgp:") have their own rows.
  const tunnels = allTunnels.filter(t => !t.id.startsWith('wgp:'))
  const rows: WgRow[] = []
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

function rowName(row: WgRow): string {
  return row.kind === 'imported' ? row.tunnel.name : row.server.server.name
}

function rowSub(row: WgRow): string {
  switch (row.kind) {
    case 'imported':          return row.tunnel.addresses?.[0] ?? ''
    case 'managed-active':    return row.tunnel.addresses?.[0] ?? row.server.server.subnet
    case 'managed-available': return row.server.server.subnet
  }
}

function rowStatus(row: WgRow): Status {
  const s = row.kind === 'managed-available'
    ? (row.server.connecting ? 'connecting' : row.server.error ? 'error' : 'disconnected')
    : row.tunnel.status
  return s === 'connected' || s === 'connecting' || s === 'error' ? s : 'disconnected'
}
