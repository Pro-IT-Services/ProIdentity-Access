import { useState, useEffect } from 'react'
import {
  Power, Trash2, Copy, Check, Activity, Server,
  Clock, ArrowDown, ArrowUp, ChevronDown, ChevronRight, ChevronLeft,
} from 'lucide-react'
import { useTunnelStore } from '../stores/useTunnelStore'
import { useManagedStore } from '../stores/useManagedStore'
import { managedDisconnectByTunnelID } from '../bridge'

export default function TunnelDetail({ onBack }: { onBack: () => void }) {
  const { tunnels, selectedId, stats, connect, disconnect, refresh, delete: deleteTunnel } = useTunnelStore()
  const { loadServers } = useManagedStore()
  const tunnel = tunnels.find(t => t.id === selectedId)
  const [expandPeers, setExpandPeers] = useState(true)
  const [confirmDelete, setConfirmDelete] = useState(false)

  useEffect(() => {
    if (!tunnel) onBack()
  }, [tunnel])

  if (!tunnel) return null

  const tunnelStats = selectedId ? stats[selectedId] : null
  const isConnected = tunnel.status === 'connected'
  const isConnecting = tunnel.status === 'connecting'
  const isError = tunnel.status === 'error'

  const handleToggle = async () => {
    if (isConnected) {
      if (tunnel.is_managed) {
        // Full managed disconnect: terminates server session, removes tunnel, updates mSessions
        await managedDisconnectByTunnelID(tunnel.id)
        await Promise.all([loadServers(), refresh()])
      } else {
        await disconnect(tunnel.id)
      }
    } else {
      await connect(tunnel.id)
    }
  }

  const handleDelete = async () => {
    if (!confirmDelete) {
      setConfirmDelete(true)
      setTimeout(() => setConfirmDelete(false), 3000)
      return
    }
    await deleteTunnel(tunnel.id)
  }

  return (
    <div className="h-full flex flex-col animate-fade-in">
      {/* Top bar */}
      <div className="flex items-center gap-3 px-4 py-3 border-b border-bg-border flex-shrink-0">
        <button
          onClick={onBack}
          className="p-2 -ml-1 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-border transition-colors flex-shrink-0"
        >
          <ChevronLeft className="w-5 h-5" />
        </button>
        <div className="flex-1 min-w-0">
          <h1 className="text-base font-semibold text-text-primary truncate">{tunnel.name}</h1>
          <StatusBadge status={tunnel.status} />
        </div>
        <div className="flex items-center gap-2">
          {/* Toggle button */}
          <button
            onClick={handleToggle}
            disabled={isConnecting}
            className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all ${
              isConnected
                ? 'bg-success/10 text-success hover:bg-success/20 border border-success/20'
                : isConnecting
                  ? 'bg-warning/10 text-warning border border-warning/20 cursor-not-allowed'
                  : 'bg-accent hover:bg-accent-hover text-white border border-transparent'
            }`}
          >
            <Power className="w-4 h-4" />
            {isConnecting ? 'Connecting…' : isConnected ? 'Disconnect' : 'Connect'}
          </button>

          {/* Delete */}
          <button
            onClick={handleDelete}
            className={`p-2 rounded-lg transition-colors ${
              confirmDelete
                ? 'bg-danger/20 text-danger border border-danger/30'
                : 'text-text-secondary hover:text-danger hover:bg-danger/10 border border-transparent'
            }`}
            title={confirmDelete ? 'Click again to confirm' : 'Delete tunnel'}
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Error bar */}
      {isError && tunnel.error && (
        <div className="mx-4 mt-3 px-4 py-3 bg-danger/10 border border-danger/20 rounded-lg text-sm text-danger">
          {tunnel.error}
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-4">

        {/* Stats row (only when connected) */}
        {isConnected && (
          <div className="grid grid-cols-3 gap-3">
            <StatCard
              icon={<ArrowDown className="w-4 h-4 text-success" />}
              label="Received"
              value={tunnelStats ? formatBytes(tunnelStats.rx_bytes) : '—'}
            />
            <StatCard
              icon={<ArrowUp className="w-4 h-4 text-accent-light" />}
              label="Sent"
              value={tunnelStats ? formatBytes(tunnelStats.tx_bytes) : '—'}
            />
            <StatCard
              icon={<Clock className="w-4 h-4 text-text-secondary" />}
              label="Handshake"
              value={tunnelStats ? formatHandshake(tunnelStats.last_handshake) : '—'}
            />
          </div>
        )}

        {/* Interface — hidden for managed tunnels */}
        {!tunnel.is_managed && (
          <Section title="Interface" icon={<Activity className="w-4 h-4" />}>
            <Row label="Addresses" value={tunnel.addresses?.join(', ') || '—'} copyable />
            {tunnel.dns?.length > 0 && (
              <Row label="DNS" value={tunnel.dns.join(', ')} copyable />
            )}
            <Row label="MTU" value={String(tunnel.mtu || 1420)} />
            {tunnel.listen_port > 0 && (
              <Row label="Listen Port" value={String(tunnel.listen_port)} />
            )}
          </Section>
        )}

        {/* Peers — managed tunnels only show Allowed IPs */}
        <Section
          title={`Peers (${tunnel.peers?.length ?? 0})`}
          icon={<Server className="w-4 h-4" />}
          collapsible
          expanded={expandPeers}
          onToggle={() => setExpandPeers(v => !v)}
        >
          {tunnel.peers?.map((peer, i) => (
            <div key={i} className={`${i > 0 ? 'border-t border-bg-border pt-3 mt-3' : ''}`}>
              {!tunnel.is_managed && peer.endpoint && <Row label="Endpoint" value={peer.endpoint} copyable />}
              <Row label="Allowed IPs" value={peer.allowed_ips?.join(', ') || '—'} />
              {!tunnel.is_managed && peer.persistent_keepalive > 0 && (
                <Row label="Keepalive" value={`${peer.persistent_keepalive}s`} />
              )}
            </div>
          ))}
        </Section>

      </div>
    </div>
  )
}

// --- Sub-components ---

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { label: string; cls: string }> = {
    connected:    { label: 'Connected',    cls: 'text-success bg-success/10 border-success/20' },
    connecting:   { label: 'Connecting…',  cls: 'text-warning bg-warning/10 border-warning/20' },
    disconnected: { label: 'Disconnected', cls: 'text-text-muted bg-bg-border/40 border-bg-border' },
    error:        { label: 'Error',        cls: 'text-danger  bg-danger/10  border-danger/20' },
  }
  const { label, cls } = map[status] ?? map.disconnected
  return (
    <span className={`inline-flex items-center gap-1 mt-1 px-2 py-0.5 rounded text-xs font-medium border ${cls}`}>
      {status === 'connected' && (
        <span className="inline-block w-1.5 h-1.5 rounded-full bg-success animate-pulse-slow" />
      )}
      {label}
    </span>
  )
}

function Section({
  title, icon, children, collapsible, expanded, onToggle,
}: {
  title: string
  icon: React.ReactNode
  children: React.ReactNode
  collapsible?: boolean
  expanded?: boolean
  onToggle?: () => void
}) {
  return (
    <div className="bg-bg-card border border-bg-border rounded-xl overflow-hidden">
      <button
        onClick={collapsible ? onToggle : undefined}
        className={`w-full flex items-center gap-2 px-4 py-3 text-left ${
          collapsible ? 'cursor-pointer hover:bg-bg-hover' : 'cursor-default'
        } transition-colors`}
      >
        <span className="text-text-secondary">{icon}</span>
        <span className="text-xs font-semibold text-text-secondary uppercase tracking-wider flex-1">
          {title}
        </span>
        {collapsible && (
          <span className="text-text-muted">
            {expanded ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
          </span>
        )}
      </button>
      {(!collapsible || expanded) && (
        <div className="px-4 pb-4 space-y-2.5 border-t border-bg-border">
          {children}
        </div>
      )}
    </div>
  )
}

function Row({
  label, value, mono, copyable,
}: {
  label: string
  value: string
  mono?: boolean
  copyable?: boolean
}) {
  const [copied, setCopied] = useState(false)

  const copy = () => {
    navigator.clipboard.writeText(value)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  return (
    <div className="flex items-start justify-between gap-4 pt-2.5">
      <span className="text-xs text-text-muted flex-shrink-0 mt-0.5">{label}</span>
      <div className="flex items-start gap-1.5 min-w-0">
        <span className={`text-xs text-right break-all selectable ${
          mono ? 'font-mono text-text-secondary' : 'text-text-primary'
        }`}>
          {value}
        </span>
        {copyable && (
          <button
            onClick={copy}
            className="flex-shrink-0 text-text-muted hover:text-text-secondary transition-colors mt-0.5"
          >
            {copied
              ? <Check className="w-3 h-3 text-success" />
              : <Copy className="w-3 h-3" />
            }
          </button>
        )}
      </div>
    </div>
  )
}

function StatCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="bg-bg-card border border-bg-border rounded-xl px-4 py-3">
      <div className="flex items-center gap-1.5 mb-1">
        {icon}
        <span className="text-xs text-text-muted">{label}</span>
      </div>
      <p className="text-sm font-semibold text-text-primary font-mono">{value}</p>
    </div>
  )
}

// --- Formatters ---

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(1)} MB`
  return `${(bytes / 1024 ** 3).toFixed(2)} GB`
}

function formatHandshake(ts: number): string {
  if (!ts) return 'Never'
  const ago = Math.floor(Date.now() / 1000 - ts)
  if (ago < 60) return `${ago}s ago`
  if (ago < 3600) return `${Math.floor(ago / 60)}m ago`
  return `${Math.floor(ago / 3600)}h ago`
}
