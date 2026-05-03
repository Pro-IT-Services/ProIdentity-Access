import { useEffect, useState, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { useAuthStore } from '../stores/useAuthStore'
import { api, type AdminSession, type WGServer, type Diagnostic } from '../api/client'
import { Globe, Activity, Users, Boxes, ArrowRight, Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/PageHeader'
import { StatusPill, StatusDot } from '@/components/StatusPill'
import { MonoChip } from '@/components/MonoChip'
import { WarningCallout } from '@/components/WarningCallout'
import { Empty } from '@/components/Empty'

function relTime(ts: string): string {
  const diff = Math.floor((Date.now() - new Date(ts).getTime()) / 1000)
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

export default function Overview() {
  const user = useAuthStore(s => s.user)
  const [servers, setServers] = useState<WGServer[]>([])
  const [sessions, setSessions] = useState<AdminSession[]>([])
  const [diagnostics, setDiagnostics] = useState<Diagnostic[]>([])
  const [counts, setCounts] = useState({ users: 0, resources: 0, roles: 0 })

  useEffect(() => {
    if (!user?.is_admin) return
    const load = () => {
      api.adminListServers().then(d => setServers(d ?? [])).catch(() => {})
      api.listAllSessions().then(d => setSessions(d ?? [])).catch(() => {})
      api.adminDiagnostics().then(d => setDiagnostics(d ?? [])).catch(() => {})
      Promise.all([api.listUsers(), api.listResources(), api.listGroups()])
        .then(([u, r, g]) => setCounts({ users: (u ?? []).length, resources: (r ?? []).length, roles: (g ?? []).length }))
        .catch(() => {})
    }
    load()
    const t = setInterval(load, 10_000)
    return () => clearInterval(t)
  }, [user])

  const sessionsByServer = useMemo(() => {
    const m: Record<string, AdminSession[]> = {}
    for (const s of sessions) if (s.server_id) (m[s.server_id] ??= []).push(s)
    return m
  }, [sessions])

  const showSetup = user?.is_admin && (servers.length === 0 || counts.resources === 0 || counts.roles === 0)

  if (!user?.is_admin) {
    return (
      <div className="p-6 max-w-3xl mx-auto">
        <PageHeader title={`Welcome, ${user?.username ?? ''}`} />
        <div className="rounded-xl border border-border bg-card p-6">
          <p className="text-sm text-foreground/85 mb-4">
            Connect to a managed VPN server from the desktop app, or view your active connections.
          </p>
          <div className="flex gap-2">
            <Button asChild><Link to="/sessions">My Sessions</Link></Button>
            <Button variant="ghost" asChild><Link to="/profile">Profile & 2FA</Link></Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-6">
      <PageHeader
        title="Overview"
        description="Live status, sessions, and anything that needs your attention."
      />

      {showSetup && <SetupCard servers={servers.length} resources={counts.resources} roles={counts.roles} />}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <Stat label="Servers" value={servers.length} sub={`${servers.filter(s => s.running).length} running`} icon={Globe} />
        <Stat label="Active sessions" value={sessions.length} sub={sessions.length > 0 ? 'live now' : 'no users connected'} icon={Activity} />
        <Stat label="People" value={counts.users} sub={`${counts.roles} roles · ${counts.resources} resources`} icon={Users} />
      </div>

      <section>
        <h2 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider mb-3">Server health</h2>
        {servers.length === 0 ? (
          <Empty
            icon={Globe}
            title="No servers yet"
            hint="Start by creating a WireGuard server. You can do that from the Servers page."
            action={<Button asChild><Link to="/servers"><Plus className="w-4 h-4" /> New Server</Link></Button>}
          />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
            {servers.map(srv => {
              const liveCount = (sessionsByServer[srv.id] ?? []).length
              const running = srv.running ?? srv.is_active
              return (
                <Link
                  key={srv.id}
                  to={`/servers/${srv.id}`}
                  className="group rounded-xl border border-border bg-card hover:border-primary/40 hover:bg-card/80 transition-colors p-4 cursor-pointer"
                >
                  <div className="flex items-start justify-between gap-3 mb-3">
                    <div className="min-w-0">
                      <p className="font-semibold text-sm truncate">{srv.name}</p>
                      <p className="text-[11px] text-muted-foreground font-mono mt-0.5 truncate">
                        {srv.endpoint}:{srv.port} · {srv.interface_name}
                      </p>
                    </div>
                    <StatusPill
                      kind={running ? 'ok' : srv.is_active ? 'warn' : 'idle'}
                      label={running ? 'running' : srv.is_active ? 'down' : 'inactive'}
                      pulse={running}
                    />
                  </div>
                  <div className="flex items-center justify-between text-xs">
                    <span className="text-muted-foreground">Subnet</span>
                    <MonoChip value={srv.subnet} bare copy={false} />
                  </div>
                  <div className="flex items-center justify-between text-xs mt-1">
                    <span className="text-muted-foreground">Live peers</span>
                    <span className="text-foreground font-medium">{liveCount}</span>
                  </div>
                  <div className="mt-3 pt-3 border-t border-border flex items-center text-[11px] text-primary opacity-0 group-hover:opacity-100 transition-opacity">
                    Open server <ArrowRight className="w-3 h-3 ml-auto" />
                  </div>
                </Link>
              )
            })}
          </div>
        )}
      </section>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <section className="lg:col-span-2">
          <h2 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider mb-3">Live sessions</h2>
          {sessions.length === 0 ? (
            <Empty title="No active sessions" hint="When users connect via the desktop client they'll show up here." />
          ) : (
            <div className="rounded-xl border border-border bg-card overflow-hidden">
              <div className="grid grid-cols-[1fr_auto_auto_auto] gap-4 px-4 py-2.5 border-b border-border bg-secondary/40 text-[11px] uppercase tracking-wider text-muted-foreground font-semibold">
                <span>User</span>
                <span>IP</span>
                <span>Last seen</span>
                <span></span>
              </div>
              {sessions.map(s => (
                <div key={s.id} className="grid grid-cols-[1fr_auto_auto_auto] gap-4 items-center px-4 py-2.5 border-b border-border last:border-0 hover:bg-secondary/30 transition-colors">
                  <div className="min-w-0 flex items-center gap-2">
                    <StatusDot kind="ok" pulse />
                    <span className="text-sm truncate">{s.username}</span>
                  </div>
                  <MonoChip value={s.assigned_ip} bare />
                  <span className="text-xs text-muted-foreground whitespace-nowrap">{relTime(s.last_keepalive)}</span>
                  <Button
                    size="sm" variant="ghost"
                    onClick={() => api.terminateSession(s.id).then(() => setSessions(ss => ss.filter(x => x.id !== s.id)))}
                    className="text-destructive hover:text-destructive hover:bg-destructive/10"
                  >
                    Terminate
                  </Button>
                </div>
              ))}
            </div>
          )}
        </section>

        <section>
          <h2 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider mb-3">Attention</h2>
          {diagnostics.length === 0 ? (
            <Empty title="All clear" hint="No drift between DB and live state." />
          ) : (
            <div className="space-y-2">
              {diagnostics.map(d => (
                <WarningCallout
                  key={d.id}
                  tone={d.severity === 'error' ? 'error' : d.severity === 'info' ? 'info' : 'warn'}
                  title={d.title}
                  description={d.detail}
                />
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  )
}

function Stat({ label, value, sub, icon: Icon }: { label: string; value: number; sub: string; icon: React.ElementType }) {
  return (
    <div className="rounded-xl border border-border bg-card p-4 flex items-start gap-4">
      <div className="w-10 h-10 rounded-lg bg-primary/10 border border-primary/20 flex items-center justify-center shrink-0">
        <Icon className="w-5 h-5 text-primary" />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-semibold">{label}</p>
        <p className="text-2xl font-semibold tabular-nums mt-0.5">{value}</p>
        <p className="text-xs text-muted-foreground mt-0.5">{sub}</p>
      </div>
    </div>
  )
}

function SetupCard({ servers, resources, roles }: { servers: number; resources: number; roles: number }) {
  const steps = [
    { done: servers > 0,   label: 'Create your first WireGuard server', to: '/servers' },
    { done: resources > 0, label: 'Define resources (LAN destinations)', to: '/access' },
    { done: roles > 0,     label: 'Create a role and assign people',     to: '/access' },
  ]
  return (
    <div className="rounded-xl border border-primary/30 bg-primary/5 p-5">
      <div className="flex items-start gap-3 mb-4">
        <div className="w-10 h-10 rounded-lg bg-primary/15 border border-primary/30 flex items-center justify-center">
          <Boxes className="w-5 h-5 text-primary" />
        </div>
        <div>
          <p className="text-sm font-semibold">Get started</p>
          <p className="text-xs text-muted-foreground mt-0.5">Three steps to get the first user connected.</p>
        </div>
      </div>
      <div className="space-y-2">
        {steps.map((s, i) => (
          <Link
            key={i}
            to={s.to}
            className="flex items-center gap-3 px-3 py-2.5 rounded-md border border-border bg-card hover:border-primary/40 transition-colors cursor-pointer"
          >
            <div className={`w-5 h-5 rounded-full border-2 flex items-center justify-center text-[10px] font-bold ${s.done ? 'border-success bg-success/20 text-success' : 'border-border text-muted-foreground'}`}>
              {s.done ? '✓' : i + 1}
            </div>
            <span className={`text-sm flex-1 ${s.done ? 'line-through text-muted-foreground' : 'text-foreground'}`}>{s.label}</span>
            <ArrowRight className="w-4 h-4 text-muted-foreground" />
          </Link>
        ))}
      </div>
    </div>
  )
}
