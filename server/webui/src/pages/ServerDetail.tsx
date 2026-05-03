import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api, type WGServer, type AdminSession, type ResourceGroup, type User, type Installation, type UserConfig } from '../api/client'
import {
  ArrowLeft, Globe, Activity, KeyRound, Settings as SettingsIcon, Trash2, FileKey, Monitor, Save,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { StatusPill, StatusDot } from '@/components/StatusPill'
import { MonoChip } from '@/components/MonoChip'
import { Empty } from '@/components/Empty'
import { Combobox } from '@/components/ui/combobox'
import { Chip } from '@/components/Chip'
import { ConfirmDelete } from '@/components/ConfirmDelete'
import { DangerZone, DangerAction } from '@/components/DangerZone'
import { cn } from '@/lib/utils'

type Tab = 'live' | 'access' | 'config' | 'configs' | 'devices'

const TABS: { id: Tab; label: string; icon: React.ElementType }[] = [
  { id: 'live',    label: 'Live',          icon: Activity },
  { id: 'access',  label: 'Access',        icon: KeyRound },
  { id: 'config',  label: 'Configuration', icon: SettingsIcon },
  { id: 'configs', label: 'Stored Configs', icon: FileKey },
  { id: 'devices', label: 'Devices',       icon: Monitor },
]

function relTime(ts: string | null | undefined): string {
  if (!ts) return '—'
  const diff = Math.floor((Date.now() - new Date(ts).getTime()) / 1000)
  if (diff < 0) return 'just now'
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

export default function ServerDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [srv, setSrv] = useState<WGServer | null>(null)
  const [tab, setTab] = useState<Tab>(() => {
    const h = window.location.hash.replace('#', '')
    return TABS.find(t => t.id === h)?.id ?? 'live'
  })

  const load = async () => {
    if (!id) return
    const list = await api.adminListServers()
    setSrv((list ?? []).find(s => s.id === id) ?? null)
  }
  useEffect(() => { load() }, [id])

  const onTab = (t: Tab) => { setTab(t); window.history.replaceState(null, '', `#${t}`) }

  if (!srv) {
    return (
      <div className="p-6 max-w-3xl mx-auto">
        <Button variant="ghost" onClick={() => navigate('/servers')}><ArrowLeft className="w-4 h-4" /> Back</Button>
        <p className="text-muted-foreground text-sm mt-4">Loading…</p>
      </div>
    )
  }

  const running = srv.running ?? srv.is_active

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <Button variant="ghost" size="sm" onClick={() => navigate('/servers')} className="mb-4 -ml-2"><ArrowLeft className="w-4 h-4" /> All servers</Button>

      <div className="flex flex-wrap items-start justify-between gap-3 mb-6">
        <div className="flex items-center gap-3 min-w-0">
          <div className="w-10 h-10 rounded-lg bg-primary/15 border border-primary/25 flex items-center justify-center shrink-0">
            <Globe className="w-5 h-5 text-primary" />
          </div>
          <div className="min-w-0">
            <h1 className="text-xl sm:text-2xl font-semibold tracking-tight">{srv.name}</h1>
            <div className="flex items-center gap-2 mt-1 text-xs text-muted-foreground flex-wrap">
              <MonoChip value={`${srv.endpoint}:${srv.port}`} />
              <span>·</span>
              <MonoChip value={srv.interface_name} copy={false} />
              <span>·</span>
              <MonoChip value={srv.subnet} />
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <StatusPill
            kind={running ? 'ok' : srv.is_active ? 'warn' : 'idle'}
            label={running ? 'running' : srv.is_active ? 'down' : 'inactive'}
            pulse={running}
          />
          {srv.external && <StatusPill kind="warn" label="external" />}
        </div>
      </div>

      <div className="border-b border-border mb-6">
        <div className="flex gap-1 overflow-x-auto scrollbar-thin">
          {TABS.map(t => (
            <button
              key={t.id}
              onClick={() => onTab(t.id)}
              className={cn(
                'inline-flex items-center gap-2 px-4 py-2.5 text-sm border-b-2 transition-colors cursor-pointer -mb-px whitespace-nowrap',
                tab === t.id ? 'border-primary text-foreground font-medium' : 'border-transparent text-muted-foreground hover:text-foreground hover:border-border',
              )}
            >
              <t.icon className="w-4 h-4" /> {t.label}
            </button>
          ))}
        </div>
      </div>

      {tab === 'live'    && <LiveTab serverId={srv.id} />}
      {tab === 'access'  && <AccessTab serverId={srv.id} />}
      {tab === 'config'  && <ConfigTab server={srv} onSaved={load} onDeleted={() => navigate('/servers')} />}
      {tab === 'configs' && <StoredConfigsTab serverId={srv.id} />}
      {tab === 'devices' && <DevicesTab serverId={srv.id} />}
    </div>
  )
}

function LiveTab({ serverId }: { serverId: string }) {
  const [sessions, setSessions] = useState<AdminSession[]>([])

  const load = () => api.listAllSessions().then(d => setSessions((d ?? []).filter(s => s.server_id === serverId))).catch(() => {})
  useEffect(() => { load(); const t = setInterval(load, 5_000); return () => clearInterval(t) }, [serverId])

  const terminate = async (id: string) => {
    if (!confirm('Terminate this session?')) return
    await api.terminateSession(id)
    load()
  }

  if (sessions.length === 0) {
    return <Empty icon={Activity} title="No live sessions" hint="When a user connects via the desktop client, their session will appear here in real time." />
  }

  return (
    <div className="rounded-xl border border-border bg-card overflow-hidden">
      <div className="grid grid-cols-[1fr_auto_auto_auto_auto] gap-4 px-4 py-2.5 border-b border-border bg-secondary/40 text-[11px] uppercase tracking-wider text-muted-foreground font-semibold">
        <span>User</span>
        <span>Assigned IP</span>
        <span>Started</span>
        <span>Last seen</span>
        <span></span>
      </div>
      {sessions.map(s => (
        <div key={s.id} className="grid grid-cols-[1fr_auto_auto_auto_auto] gap-4 items-center px-4 py-2.5 border-b border-border last:border-0 hover:bg-secondary/30 transition-colors">
          <div className="flex items-center gap-2 min-w-0">
            <StatusDot kind="ok" pulse />
            <span className="text-sm truncate">{s.username}</span>
            <span className="text-[11px] text-muted-foreground truncate">{s.email}</span>
          </div>
          <MonoChip value={s.assigned_ip} bare />
          <span className="text-xs text-muted-foreground whitespace-nowrap">{relTime(s.created_at)}</span>
          <span className="text-xs text-muted-foreground whitespace-nowrap">{relTime(s.last_keepalive)}</span>
          <Button size="sm" variant="ghost" onClick={() => terminate(s.id)} className="text-destructive hover:text-destructive hover:bg-destructive/10">Terminate</Button>
        </div>
      ))}
    </div>
  )
}

function AccessTab({ serverId }: { serverId: string }) {
  const [bundles, setBundles] = useState<ResourceGroup[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [allBundles, setAllBundles] = useState<ResourceGroup[]>([])
  const [allUsers, setAllUsers] = useState<User[]>([])

  const load = () => {
    api.adminServerBundles(serverId).then(d => setBundles(d ?? []))
    api.adminServerUsers(serverId).then(d => setUsers(d ?? []))
    api.listResourceGroups().then(d => setAllBundles(d ?? []))
    api.listUsers().then(d => setAllUsers(d ?? []))
  }
  useEffect(() => { load() }, [serverId])

  const addBundle = async (bid: string) => { await api.adminAddServerBundle(serverId, bid); load() }
  const removeBundle = async (bid: string) => { await api.adminRemoveServerBundle(serverId, bid); load() }
  const addUser = async (uid: string) => { await api.adminAddServerUser(serverId, uid); load() }
  const removeUser = async (uid: string) => { await api.adminRemoveServerUser(serverId, uid); load() }

  return (
    <div className="space-y-6">
      <Section title="People with access" hint={`${users.length} ${users.length === 1 ? 'user' : 'users'}`}
        description="These users can connect to this server. Assign bundles per user in their profile to control what they can reach.">
        <div className="flex flex-wrap items-center gap-1.5">
          {users.map(u => <Chip key={u.id} label={u.username} hint={u.email} tone="primary" onRemove={() => removeUser(u.id)} />)}
          <Combobox
            placeholder="Grant access"
            options={allUsers.filter(u => !users.some(x => x.id === u.id)).map(u => ({ value: u.id, label: u.username, hint: u.email }))}
            onSelect={addUser}
          />
        </div>
      </Section>

      <Section title="Allowed bundles" hint={`${bundles.length} ${bundles.length === 1 ? 'bundle' : 'bundles'}`}
        description="Bundles that can be assigned to users on this server. Adding a bundle here makes it available for assignment — it does not give access to anyone automatically.">
        <div className="flex flex-wrap items-center gap-1.5">
          {bundles.map(b => <Chip key={b.id} label={b.name} tone="success" onRemove={() => removeBundle(b.id)} />)}
          <Combobox
            placeholder="Allow bundle"
            options={allBundles.filter(b => !bundles.some(x => x.id === b.id)).map(b => ({ value: b.id, label: b.name, hint: b.description ?? undefined }))}
            onSelect={addBundle}
          />
        </div>
      </Section>
    </div>
  )
}

function ConfigTab({ server, onSaved, onDeleted }: { server: WGServer; onSaved: () => void; onDeleted: () => void }) {
  const [form, setForm] = useState({ name: server.name, endpoint: server.endpoint, dns: server.dns ?? '' })
  const [busy, setBusy] = useState(false)
  const [confirm, setConfirm] = useState(false)

  useEffect(() => { setForm({ name: server.name, endpoint: server.endpoint, dns: server.dns ?? '' }) }, [server.id])

  const dirty = form.name !== server.name || form.endpoint !== server.endpoint || (form.dns ?? '') !== (server.dns ?? '')

  const save = async () => {
    setBusy(true)
    try {
      await api.adminUpdateServer(server.id, { name: form.name, endpoint: form.endpoint, dns: form.dns })
      onSaved()
    } finally { setBusy(false) }
  }

  return (
    <div className="space-y-6 max-w-3xl">
      <section className="rounded-xl border border-border bg-card divide-y divide-border">
        <KV label="Endpoint" edit>
          <Input value={form.endpoint} onChange={e => setForm(p => ({ ...p, endpoint: e.target.value }))} className="max-w-xs" />
        </KV>
        <KV label="UDP port"><span className="text-sm font-mono">{server.port}</span></KV>
        <KV label="Interface"><MonoChip value={server.interface_name} copy={false} /></KV>
        <KV label="Subnet"><MonoChip value={server.subnet} /></KV>
        <KV label="Display name" edit>
          <Input value={form.name} onChange={e => setForm(p => ({ ...p, name: e.target.value }))} className="max-w-xs" />
        </KV>
        <KV label="DNS servers" edit>
          <Input value={form.dns} onChange={e => setForm(p => ({ ...p, dns: e.target.value }))} placeholder="1.1.1.1,8.8.8.8" className="max-w-xs" />
        </KV>
        <KV label="Type"><span className="text-sm">{server.external ? 'External (manual peer)' : 'Managed by controller'}</span></KV>
        <KV label="Created"><span className="text-sm text-muted-foreground">{relTime(server.created_at)}</span></KV>
      </section>

      <div className="flex justify-end">
        <Button onClick={save} disabled={!dirty || busy}><Save className="w-4 h-4" /> {busy ? 'Saving…' : 'Save changes'}</Button>
      </div>

      <DangerZone>
        <DangerAction
          title="Delete this server"
          description="Tears down the kernel interface, removes the firewall MASQUERADE rule, and drops every session and IP-pool entry tied to it."
          action={
            <Button variant="destructive" size="sm" onClick={() => setConfirm(true)}>
              <Trash2 className="w-4 h-4" /> Delete
            </Button>
          }
        />
      </DangerZone>

      <ConfirmDelete
        open={confirm}
        onOpenChange={setConfirm}
        title={`Delete server "${server.name}"`}
        description={
          <>
            <p>The kernel WireGuard interface <span className="font-mono text-foreground">{server.interface_name}</span> will be torn down. Any user currently connected to this server will be disconnected.</p>
            <p className="text-warning text-xs mt-2">This cannot be undone — the keypair will be destroyed.</p>
          </>
        }
        confirmText={server.name}
        actionLabel="Delete server"
        onConfirm={async () => { await api.adminDeleteServer(server.id); onDeleted() }}
      />
    </div>
  )
}

function StoredConfigsTab({ serverId }: { serverId: string }) {
  const [configs, setConfigs] = useState<UserConfig[]>([])
  useEffect(() => { api.adminListUserConfigs().then(d => setConfigs(d ?? [])) }, [serverId])

  // Server-side store has no per-server filtering today; show all and let admin filter visually.
  if (configs.length === 0) {
    return <Empty icon={FileKey} title="No stored configs" hint="When users save a WireGuard config to the server (encrypted), it shows up here." />
  }
  return (
    <div className="rounded-xl border border-border bg-card divide-y divide-border">
      {configs.map(c => (
        <div key={c.id} className="grid grid-cols-[1fr_auto_auto_auto] gap-4 items-center px-4 py-3">
          <div className="min-w-0">
            <p className="text-sm font-medium truncate">{c.name}</p>
            <p className="text-[11px] text-muted-foreground truncate">{c.username} · {c.email}</p>
          </div>
          <span className="text-xs text-muted-foreground">{relTime(c.created_at)}</span>
          <Button size="sm" variant="ghost" onClick={async () => { if (confirm('Delete this stored config?')) { await api.adminDeleteUserConfig(c.id); api.adminListUserConfigs().then(d => setConfigs(d ?? [])) } }} className="text-destructive hover:text-destructive hover:bg-destructive/10">
            Delete
          </Button>
        </div>
      ))}
    </div>
  )
}

function DevicesTab({ serverId }: { serverId: string }) {
  const [installs, setInstalls] = useState<Installation[]>([])
  useEffect(() => { api.listInstallations().then(d => setInstalls(d ?? [])) }, [serverId])

  if (installs.length === 0) {
    return <Empty icon={Monitor} title="No registered devices" hint="Devices register themselves the first time they pair with the server." />
  }
  return (
    <div className="rounded-xl border border-border bg-card divide-y divide-border">
      {installs.map(i => (
        <div key={i.id} className="grid grid-cols-[1fr_auto_auto_auto] gap-4 items-center px-4 py-3">
          <div className="min-w-0">
            <p className="text-sm font-medium truncate">{i.device_name || '(unnamed)'}</p>
            <p className="text-[11px] text-muted-foreground truncate">{i.username}</p>
          </div>
          <StatusPill kind={i.is_active ? 'ok' : 'idle'} label={i.is_active ? 'active' : 'revoked'} />
          <span className="text-xs text-muted-foreground">{i.last_seen ? `seen ${relTime(i.last_seen)}` : '—'}</span>
          {i.is_active && (
            <Button size="sm" variant="ghost"
              onClick={async () => { if (confirm('Revoke this device? It will be signed out immediately.')) { await api.revokeInstallation(i.id); api.listInstallations().then(d => setInstalls(d ?? [])) } }}
              className="text-destructive hover:text-destructive hover:bg-destructive/10">
              Revoke
            </Button>
          )}
        </div>
      ))}
    </div>
  )
}

function KV({ label, children }: { label: string; edit?: boolean; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[160px_1fr] gap-4 items-center px-4 py-2.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <div>{children}</div>
    </div>
  )
}

function Section({ title, hint, description, children }: { title: string; hint?: string; description?: string; children: React.ReactNode }) {
  return (
    <section>
      <div className="flex items-baseline justify-between mb-2">
        <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{title}</h3>
        {hint && <span className="text-[11px] text-muted-foreground">{hint}</span>}
      </div>
      {description && <p className="text-xs text-muted-foreground mb-3 max-w-prose">{description}</p>}
      {children}
    </section>
  )
}
