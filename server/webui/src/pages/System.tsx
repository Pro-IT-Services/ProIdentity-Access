import { useEffect, useMemo, useState } from 'react'
import { api, type Diagnostic, type AuditRow, type DenialRow, type Group, type User, type PermDef } from '../api/client'
import { Save, Settings as SettingsIcon, AlertTriangle, ScrollText, Search, ChevronLeft, ChevronRight, ShieldAlert, KeyRound, Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { PageHeader } from '@/components/PageHeader'
import { WarningCallout } from '@/components/WarningCallout'
import { Empty } from '@/components/Empty'
import { StatusPill } from '@/components/StatusPill'
import { MonoChip } from '@/components/MonoChip'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet'
import { Combobox } from '@/components/ui/combobox'
import { Chip } from '@/components/Chip'
import { ConfirmDelete } from '@/components/ConfirmDelete'
import { DangerZone, DangerAction } from '@/components/DangerZone'
import { useHasPerm } from '@/stores/useAuthStore'
import { cn } from '@/lib/utils'

type Tab = 'settings' | 'roles' | 'health' | 'audit' | 'denials'

const TAB_DEFS: { id: Tab; label: string; icon: React.ElementType; perm: string }[] = [
  { id: 'settings', label: 'Settings',  icon: SettingsIcon,    perm: 'system.settings'  },
  { id: 'roles',    label: 'Roles',     icon: KeyRound,        perm: 'roles.manage'     },
  { id: 'health',   label: 'Health',    icon: AlertTriangle,   perm: 'diagnostics.read' },
  { id: 'denials',  label: 'Denials',   icon: ShieldAlert,     perm: 'denials.read'     },
  { id: 'audit',    label: 'Audit log', icon: ScrollText,      perm: 'audit.read'       },
]

const SETTING_DEFS: { key: string; label: string; description: string; type?: string; secret?: boolean }[] = [
  { key: 'vpn_name',           label: 'VPN display name',  description: 'Shown in the desktop client.' },
  { key: 'session_timeout',    label: 'Session timeout',   description: 'Seconds without keepalive before a session is terminated.', type: 'number' },
  { key: 'keepalive_interval', label: 'Keepalive interval', description: 'How often clients should send keepalive (informational).', type: 'number' },
  { key: 'webauthn_rp_id',     label: 'WebAuthn RP ID',    description: 'Relying-party domain (e.g. vpn.example.com). Required for passkeys.' },
  { key: 'webauthn_rp_name',   label: 'WebAuthn RP name',  description: 'Display name shown during passkey registration.' },
  { key: 'webauthn_origin',    label: 'WebAuthn origin',   description: 'Full origin URL (e.g. https://vpn.example.com).' },
  { key: 'push_auth_enabled',  label: 'Push Auth',         description: 'Enable ProIdentity Cloud push authentication. Replaces classic TOTP when enabled. Set to "true" or "false".', type: 'text' },
  { key: 'push_auth_api_key',  label: 'Push Auth API key', description: 'Your ProIdentity Cloud Service Provider API key (pi_live_xxx). Leave empty to keep the current key.', secret: true },
]

export default function System() {
  // Filter tabs to ones the user holds permission for.
  const tabsAvailable = useTabsForUser()
  const [tab, setTab] = useState<Tab>(() => {
    const h = window.location.hash.replace('#', '') as Tab
    return tabsAvailable.find(t => t.id === h)?.id ?? tabsAvailable[0]?.id ?? 'settings'
  })
  const onTab = (t: Tab) => { setTab(t); window.history.replaceState(null, '', `#${t}`) }

  if (tabsAvailable.length === 0) {
    return (
      <div className="p-6 max-w-3xl mx-auto">
        <PageHeader title="System" />
        <Empty title="Nothing to show" hint="You don't hold any system-area permissions. Ask an admin to grant you one." />
      </div>
    )
  }

  return (
    <div className="p-6 max-w-5xl mx-auto">
      <PageHeader
        title="System"
        description="Server-wide settings and live health checks."
      />

      <div className="border-b border-border mb-6">
        <div className="flex gap-1">
          {tabsAvailable.map(t => (
            <button
              key={t.id}
              onClick={() => onTab(t.id)}
              className={cn(
                'inline-flex items-center gap-2 px-4 py-2.5 text-sm border-b-2 transition-colors cursor-pointer -mb-px',
                tab === t.id ? 'border-primary text-foreground font-medium' : 'border-transparent text-muted-foreground hover:text-foreground hover:border-border',
              )}
            >
              <t.icon className="w-4 h-4" /> {t.label}
            </button>
          ))}
        </div>
      </div>

      {tab === 'settings' && <SettingsTab />}
      {tab === 'roles'    && <RolesTab />}
      {tab === 'health'   && <HealthTab />}
      {tab === 'denials'  && <DenialsTab />}
      {tab === 'audit'    && <AuditTab />}
    </div>
  )
}

/** Returns only the tabs the current user has permission to view. */
function useTabsForUser() {
  const settings = useHasPerm('system.settings')
  const roles    = useHasPerm('roles.manage')
  const health   = useHasPerm('diagnostics.read')
  const denials  = useHasPerm('denials.read')
  const audit    = useHasPerm('audit.read')
  return useMemo(() => {
    const allow: Record<Tab, boolean> = { settings, roles, health, denials, audit }
    return TAB_DEFS.filter(t => allow[t.id])
  }, [settings, roles, health, denials, audit])
}

function RolesTab() {
  const [roles, setRoles] = useState<Group[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [members, setMembers] = useState<Record<string, User[]>>({}) // roleId -> users in that role
  const [query, setQuery] = useState('')
  const [mode, setMode] = useState<{ kind: 'closed' } | { kind: 'create' } | { kind: 'edit'; id: string }>({ kind: 'closed' })
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const load = async () => {
    const [rs, us] = await Promise.all([api.listGroups(), api.listUsers()])
    setRoles(rs ?? []); setUsers(us ?? [])
    // Compute role membership by scanning each user's role list.
    const userRoleLists = await Promise.all((us ?? []).map(u => api.userGroups(u.id).then(gs => ({ user: u, gs }))))
    const map: Record<string, User[]> = {}
    for (const { user, gs } of userRoleLists) {
      for (const g of gs) (map[g.id] ??= []).push(user)
    }
    setMembers(map)
  }
  useEffect(() => { load() }, [])

  const filtered = roles.filter(r => {
    const q = query.toLowerCase()
    return !q || r.name.toLowerCase().includes(q) || (r.description?.toLowerCase().includes(q) ?? false)
  })
  const selected = roles.find(r => r.id === selectedId) ?? null

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Roles group people for administrative purposes. They <em>do not</em> grant VPN access — that's managed by attaching bundles to servers and granting people server access on the Access page.
      </p>

      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input value={query} onChange={e => setQuery(e.target.value)} placeholder={`Search ${roles.length} ${roles.length === 1 ? 'role' : 'roles'}…`} className="pl-9" />
        </div>
        <Button onClick={() => setMode({ kind: 'create' })}><Plus className="w-4 h-4" /> New Role</Button>
      </div>

      {filtered.length === 0 ? (
        <Empty
          icon={KeyRound}
          title={roles.length === 0 ? 'No roles yet' : 'No matches'}
          hint={roles.length === 0 ? 'Roles are useful for grouping people you want to refer to as a unit (e.g. "Engineering", "Contractors"). Future: per-role admin permissions.' : 'Try a different search term.'}
          action={roles.length === 0 ? <Button onClick={() => setMode({ kind: 'create' })}><Plus className="w-4 h-4" /> New Role</Button> : undefined}
        />
      ) : (
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          {filtered.map(r => {
            const m = members[r.id] ?? []
            return (
              <button
                key={r.id}
                onClick={() => setSelectedId(r.id)}
                className="w-full grid grid-cols-[1fr_auto_auto] gap-3 items-center text-left px-4 py-3 border-b border-border last:border-0 hover:bg-secondary/30 transition-colors cursor-pointer"
              >
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">{r.name}</p>
                  {r.description && <p className="text-[11px] text-muted-foreground truncate">{r.description}</p>}
                </div>
                <span className="text-[11px] text-muted-foreground whitespace-nowrap">
                  {m.length} {m.length === 1 ? 'member' : 'members'}
                </span>
                <ChevronRight className="w-4 h-4 text-muted-foreground" />
              </button>
            )
          })}
        </div>
      )}

      <RoleSheet
        mode={mode}
        onClose={() => setMode({ kind: 'closed' })}
        onSaved={() => { load(); setMode({ kind: 'closed' }) }}
        editing={mode.kind === 'edit' ? roles.find(r => r.id === mode.id) : undefined}
      />

      {selected && (
        <RoleDrawer
          role={selected}
          allUsers={users}
          members={members[selected.id] ?? []}
          onClose={() => setSelectedId(null)}
          onChanged={load}
          onEdit={() => { setMode({ kind: 'edit', id: selected.id }); setSelectedId(null) }}
          onDelete={() => { load(); setSelectedId(null) }}
        />
      )}
    </div>
  )
}

function RoleSheet({
  mode, onClose, onSaved, editing,
}: {
  mode: { kind: 'closed' } | { kind: 'create' } | { kind: 'edit'; id: string }
  onClose: () => void
  onSaved: () => void
  editing?: Group
}) {
  const open = mode.kind !== 'closed'
  const isEdit = mode.kind === 'edit'
  const [form, setForm] = useState({ name: '', description: '' })
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (mode.kind === 'create') setForm({ name: '', description: '' })
    else if (mode.kind === 'edit' && editing) setForm({ name: editing.name, description: editing.description ?? '' })
    setError('')
  }, [mode.kind, editing?.id])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true); setError('')
    try {
      if (isEdit && editing) await api.updateGroup(editing.id, form)
      else await api.createGroup(form)
      onSaved()
    } catch (e: any) { setError(e.message ?? 'Failed') }
    finally { setBusy(false) }
  }

  return (
    <Sheet open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>{isEdit ? `Edit ${editing?.name}` : 'New Role'}</SheetTitle>
          <SheetDescription>
            A named group of people. Admin-permission semantics are coming later — for now this is purely organizational.
          </SheetDescription>
        </SheetHeader>
        <form onSubmit={submit} className="contents">
          <SheetBody>
            <div className="space-y-4">
              {error && <p className="text-sm text-destructive">{error}</p>}
              <div className="space-y-1.5">
                <Label className="text-xs">Name</Label>
                <Input value={form.name} onChange={e => setForm(p => ({ ...p, name: e.target.value }))} placeholder="e.g. Engineering" required autoFocus />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Description <span className="text-muted-foreground">(optional)</span></Label>
                <Input value={form.description} onChange={e => setForm(p => ({ ...p, description: e.target.value }))} />
              </div>
            </div>
          </SheetBody>
          <SheetFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? 'Saving…' : isEdit ? 'Save' : 'Create Role'}</Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}

function RoleDrawer({
  role, allUsers, members, onClose, onChanged, onEdit, onDelete,
}: {
  role: Group
  allUsers: User[]
  members: User[]
  onClose: () => void
  onChanged: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const [confirm, setConfirm] = useState(false)
  const [catalog, setCatalog] = useState<PermDef[]>([])
  const [perms, setPerms] = useState<string[]>(role.permissions ?? [])
  const [savingPerms, setSavingPerms] = useState(false)
  const candidates = allUsers.filter(u => !members.some(m => m.id === u.id))

  useEffect(() => { setPerms(role.permissions ?? []) }, [role.id, role.permissions])
  useEffect(() => { api.permCatalog().then(setCatalog).catch(() => {}) }, [])

  const addMember = async (uid: string) => { await api.addUserGroup(uid, role.id); onChanged() }
  const removeMember = async (uid: string) => { await api.removeUserGroup(uid, role.id); onChanged() }
  const togglePerm = (key: string) =>
    setPerms(p => p.includes(key) ? p.filter(x => x !== key) : [...p, key])
  const savePerms = async () => {
    setSavingPerms(true)
    try {
      await api.updateRolePermissions(role.id, perms)
      onChanged()
    } finally { setSavingPerms(false) }
  }

  const groupedCatalog = useMemo(() => {
    const m = new Map<string, PermDef[]>()
    for (const p of catalog) {
      if (!m.has(p.category)) m.set(p.category, [])
      m.get(p.category)!.push(p)
    }
    return m
  }, [catalog])

  const dirty = useMemo(() => {
    const a = new Set(role.permissions ?? [])
    const b = new Set(perms)
    if (a.size !== b.size) return true
    for (const x of a) if (!b.has(x)) return true
    return false
  }, [perms, role.permissions])

  return (
    <Sheet open onOpenChange={(v) => { if (!v) onClose() }}>
      <SheetContent width="w-full sm:max-w-2xl">
        <SheetHeader>
          <div className="flex items-start gap-3">
            <div className="w-10 h-10 rounded-lg bg-primary/15 border border-primary/25 flex items-center justify-center shrink-0">
              <KeyRound className="w-5 h-5 text-primary" />
            </div>
            <div className="flex-1 min-w-0">
              <SheetTitle>{role.name}</SheetTitle>
              {role.description && <SheetDescription>{role.description}</SheetDescription>}
            </div>
          </div>
        </SheetHeader>
        <SheetBody>
          <div className="space-y-6">
            <section>
              <div className="flex items-baseline justify-between mb-2">
                <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Members</h3>
                <span className="text-[11px] text-muted-foreground">{members.length} {members.length === 1 ? 'person' : 'people'}</span>
              </div>
              <div className="flex flex-wrap items-center gap-1.5">
                {members.map(u => <Chip key={u.id} label={u.username} hint={u.email} tone="primary" onRemove={() => removeMember(u.id)} />)}
                <Combobox
                  placeholder="Add member"
                  options={candidates.map(u => ({ value: u.id, label: u.username, hint: u.email }))}
                  onSelect={addMember}
                  emptyText="No more users to add"
                />
              </div>
            </section>

            <section>
              <div className="flex items-baseline justify-between mb-2">
                <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Permissions</h3>
                <span className="text-[11px] text-muted-foreground">
                  {perms.length} of {catalog.length} granted
                </span>
              </div>
              <p className="text-xs text-muted-foreground mb-3">
                Members of this role gain these admin abilities. Combined across all roles a user is in.
              </p>
              <div className="space-y-3">
                {Array.from(groupedCatalog.entries()).map(([cat, defs]) => (
                  <div key={cat} className="rounded-md border border-border bg-card/60 p-3">
                    <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground mb-2">{cat}</p>
                    <div className="space-y-1.5">
                      {defs.map(d => {
                        const checked = perms.includes(d.key)
                        return (
                          <label key={d.key} className="flex items-start gap-2.5 px-2 py-1.5 rounded hover:bg-secondary/40 cursor-pointer transition-colors">
                            <input
                              type="checkbox"
                              checked={checked}
                              onChange={() => togglePerm(d.key)}
                              className="mt-1 cursor-pointer"
                            />
                            <div className="flex-1 min-w-0">
                              <p className="text-sm">{d.label} <span className="font-mono text-[10px] text-muted-foreground ml-1">{d.key}</span></p>
                              <p className="text-[11px] text-muted-foreground">{d.description}</p>
                            </div>
                          </label>
                        )
                      })}
                    </div>
                  </div>
                ))}
              </div>
              <div className="flex justify-end mt-3">
                <Button size="sm" onClick={savePerms} disabled={!dirty || savingPerms}>
                  <Save className="w-4 h-4" /> {savingPerms ? 'Saving…' : 'Save permissions'}
                </Button>
              </div>
            </section>

            <section>
              <Button variant="ghost" onClick={onEdit}>Edit name / description</Button>
            </section>

            <DangerZone>
              <DangerAction
                title="Delete this role"
                description="Removes the role and unlinks every member. The members themselves are not affected."
                action={
                  <Button variant="destructive" size="sm" onClick={() => setConfirm(true)}>
                    <Trash2 className="w-4 h-4" /> Delete
                  </Button>
                }
              />
            </DangerZone>
          </div>
        </SheetBody>
      </SheetContent>

      <ConfirmDelete
        open={confirm}
        onOpenChange={setConfirm}
        title={`Delete role "${role.name}"`}
        description={<>Members are unlinked. The users themselves stay.</>}
        confirmText={role.name}
        actionLabel="Delete role"
        onConfirm={async () => { await api.deleteGroup(role.id); onDelete() }}
      />
    </Sheet>
  )
}

function DenialsTab() {
  const [rows, setRows] = useState<DenialRow[]>([])
  const [total, setTotal] = useState(0)
  const [limit] = useState(50)
  const [offset, setOffset] = useState(0)
  const [user, setUser] = useState('')
  const [dst, setDst] = useState('')
  const [loading, setLoading] = useState(false)

  const load = () => {
    setLoading(true)
    api.adminDenials({ limit, offset, user_id: user || undefined, dst_ip: dst || undefined })
      .then(d => { setRows(d.items ?? []); setTotal(d.total ?? 0) })
      .finally(() => setLoading(false))
  }
  useEffect(() => { load(); const t = setInterval(load, 15_000); return () => clearInterval(t) }, [offset, user, dst])

  const page = Math.floor(offset / limit) + 1
  const totalPages = Math.max(1, Math.ceil(total / limit))

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Packets from VPN clients dropped by the firewall — i.e. attempts to reach destinations or ports the user isn't allowed to. Auto-refreshes every 15s.
      </p>

      <div className="flex flex-wrap items-center gap-2">
        <div className="relative flex-1 min-w-[160px]">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input value={user} onChange={e => { setOffset(0); setUser(e.target.value) }} placeholder="Filter by user id" className="pl-9 font-mono text-xs" />
        </div>
        <div className="relative flex-1 min-w-[160px]">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input value={dst} onChange={e => { setOffset(0); setDst(e.target.value) }} placeholder="Filter by destination IP" className="pl-9 font-mono text-xs" />
        </div>
        <Button variant="ghost" onClick={load} disabled={loading}>{loading ? 'Loading…' : 'Refresh'}</Button>
      </div>

      {rows.length === 0 ? (
        <Empty
          icon={ShieldAlert}
          title="No denials"
          hint="When a VPN user tries to reach a destination or port that no role grants them, it shows up here. If you've never seen anything, that's a good sign."
        />
      ) : (
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          <div className="grid grid-cols-[140px_1fr_1fr_auto_60px_60px] gap-3 px-4 py-2.5 border-b border-border bg-secondary/40 text-[11px] uppercase tracking-wider text-muted-foreground font-semibold">
            <span>Last seen</span>
            <span>Who</span>
            <span>Tried to reach</span>
            <span>Port</span>
            <span>Proto</span>
            <span>Count</span>
          </div>
          {rows.map(r => (
            <div key={r.id} className="grid grid-cols-[140px_1fr_1fr_auto_60px_60px] gap-3 items-center px-4 py-2.5 border-b border-border last:border-0 hover:bg-secondary/30 transition-colors">
              <span className="text-xs text-muted-foreground" title={new Date(r.last_ts).toLocaleString()}>
                {relTime(r.last_ts)}
              </span>
              <div className="min-w-0">
                {r.username ? (
                  <span className="text-sm truncate">{r.username}</span>
                ) : (
                  <span className="text-xs text-muted-foreground italic">unattributed</span>
                )}
                <span className="block text-[11px] text-muted-foreground font-mono truncate">{r.src_ip}</span>
              </div>
              <MonoChip value={r.dst_ip} bare />
              <span className="text-xs font-mono tabular-nums text-right">{r.dst_port ?? '—'}</span>
              <span className="text-xs font-mono uppercase">{r.proto}</span>
              <span className="text-xs font-mono tabular-nums text-right">{r.count}</span>
            </div>
          ))}
        </div>
      )}

      {total > limit && (
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>Showing {offset + 1}–{Math.min(offset + limit, total)} of {total}</span>
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}>
              <ChevronLeft className="w-4 h-4" /> Prev
            </Button>
            <span className="px-2">Page {page} / {totalPages}</span>
            <Button variant="ghost" size="sm" disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}>
              Next <ChevronRight className="w-4 h-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

function relTime(ts: string): string {
  const diff = Math.floor((Date.now() - new Date(ts).getTime()) / 1000)
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return new Date(ts).toLocaleString()
}

function AuditTab() {
  const [rows, setRows] = useState<AuditRow[]>([])
  const [total, setTotal] = useState(0)
  const [limit] = useState(50)
  const [offset, setOffset] = useState(0)
  const [actor, setActor] = useState('')
  const [action, setAction] = useState('')
  const [expanded, setExpanded] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const load = () => {
    setLoading(true)
    api.adminAudit({ limit, offset, actor: actor || undefined, action: action || undefined })
      .then(d => { setRows(d.items ?? []); setTotal(d.total ?? 0) })
      .finally(() => setLoading(false))
  }
  // Reload on filter/page change.
  useEffect(() => { load() }, [offset, actor, action])

  const page = Math.floor(offset / limit) + 1
  const totalPages = Math.max(1, Math.ceil(total / limit))

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative flex-1 min-w-[160px]">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input value={actor} onChange={e => { setOffset(0); setActor(e.target.value) }} placeholder="Filter by actor (username or id)" className="pl-9" />
        </div>
        <div className="relative flex-1 min-w-[160px]">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input value={action} onChange={e => { setOffset(0); setAction(e.target.value) }} placeholder="Filter by action or path" className="pl-9" />
        </div>
        <Button variant="ghost" onClick={() => { setOffset(0); load() }} disabled={loading}>{loading ? 'Loading…' : 'Refresh'}</Button>
      </div>

      {rows.length === 0 ? (
        <Empty icon={ScrollText} title="No audit entries" hint="Mutating actions (login, create, update, delete) appear here as they happen." />
      ) : (
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          <div className="grid grid-cols-[110px_1fr_140px_2fr_60px_70px] gap-3 px-4 py-2.5 border-b border-border bg-secondary/40 text-[11px] uppercase tracking-wider text-muted-foreground font-semibold">
            <span>When</span>
            <span>Actor</span>
            <span>Action</span>
            <span>Target / path</span>
            <span>Status</span>
            <span></span>
          </div>
          {rows.map(r => {
            const ok = r.success === 1
            const targetLabel = r.target_label || r.target_id || ''
            const isExpanded = expanded === r.id
            return (
              <div key={r.id} className="border-b border-border last:border-0">
                <button
                  onClick={() => setExpanded(isExpanded ? null : r.id)}
                  className="w-full grid grid-cols-[110px_1fr_140px_2fr_60px_70px] gap-3 items-center text-left px-4 py-2.5 hover:bg-secondary/30 transition-colors cursor-pointer"
                >
                  <span className="text-xs text-muted-foreground whitespace-nowrap" title={new Date(r.ts).toLocaleString()}>{relTime(r.ts)}</span>
                  <span className="text-sm truncate">
                    {r.actor_username || <span className="text-muted-foreground italic">anonymous</span>}
                  </span>
                  <span className="text-xs">
                    {r.action ? <span className="font-mono">{r.action}</span> : <span className="font-mono text-muted-foreground">{r.method}</span>}
                  </span>
                  <span className="text-xs truncate flex items-center gap-2 min-w-0">
                    {targetLabel && <span className="truncate">{targetLabel}</span>}
                    <MonoChip value={r.path} bare copy={false} />
                  </span>
                  <span className="text-xs font-mono tabular-nums">{r.status_code}</span>
                  <StatusPill kind={ok ? 'ok' : 'down'} label={ok ? 'ok' : 'fail'} />
                </button>
                {isExpanded && (
                  <div className="px-4 py-3 bg-secondary/20 border-t border-border text-xs space-y-1 font-mono">
                    {r.error_message && <KV label="error" value={r.error_message} className="text-destructive" />}
                    {r.ip && <KV label="ip" value={r.ip} />}
                    {r.user_agent && <KV label="user-agent" value={r.user_agent} truncate />}
                    {r.target_type && <KV label="target type" value={r.target_type} />}
                    {r.target_id && <KV label="target id" value={r.target_id} />}
                    {r.detail && <KV label="detail" value={r.detail} />}
                    <KV label="entry id" value={r.id} />
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {total > limit && (
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>Showing {offset + 1}–{Math.min(offset + limit, total)} of {total}</span>
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}>
              <ChevronLeft className="w-4 h-4" /> Prev
            </Button>
            <span className="px-2">Page {page} / {totalPages}</span>
            <Button variant="ghost" size="sm" disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}>
              Next <ChevronRight className="w-4 h-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

function KV({ label, value, truncate, className }: { label: string; value: string; truncate?: boolean; className?: string }) {
  return (
    <div className="grid grid-cols-[100px_1fr] gap-2">
      <span className="text-muted-foreground">{label}</span>
      <span className={cn(truncate && 'truncate', className)}>{value}</span>
    </div>
  )
}

function SettingsTab() {
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [saved, setSaved] = useState(false)
  const [syncMessage, setSyncMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => { api.getSettings().then(setSettings).catch(() => {}) }, [])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true); setError('')
    try {
      const payload = Object.fromEntries(Object.entries(settings).filter(([key, value]) => {
        const def = SETTING_DEFS.find(d => d.key === key)
        return !def?.secret || (value !== '' && value !== '__configured__')
      }))
      const res = await api.updateSettings(payload)
      if (res.push_auth_sync) {
        const { checked, synced, failed } = res.push_auth_sync
        setSyncMessage(`Push Auth checked ${checked} users, synced ${synced}, failed ${failed}.`)
      } else {
        setSyncMessage('')
      }
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (e: any) { setError(e.message ?? 'Failed') }
    finally { setBusy(false) }
  }

  return (
    <form onSubmit={submit} className="space-y-4 max-w-2xl">
      {error && <WarningCallout tone="error" title={error} />}
      {saved && <WarningCallout tone="success" title="Settings saved." />}
      {syncMessage && <WarningCallout tone="info" title={syncMessage} />}

      <div className="rounded-xl border border-border bg-card divide-y divide-border">
        {SETTING_DEFS.map(def => (
          <div key={def.key} className="grid grid-cols-1 sm:grid-cols-[200px_1fr] gap-3 px-4 py-3.5">
            <div>
              <Label className="text-sm">{def.label}</Label>
              <p className="text-[11px] text-muted-foreground mt-0.5">{def.description}</p>
            </div>
            <div>
              <Input
                type={def.secret ? 'password' : def.type ?? 'text'}
                value={settings[def.key] ?? ''}
                onChange={e => setSettings(p => ({ ...p, [def.key]: e.target.value }))}
                placeholder={def.secret ? '••••••••' : undefined}
                className="font-mono text-sm"
              />
            </div>
          </div>
        ))}
      </div>

      <div className="flex justify-end">
        <Button type="submit" disabled={busy}><Save className="w-4 h-4" /> {busy ? 'Saving…' : 'Save settings'}</Button>
      </div>
    </form>
  )
}

function HealthTab() {
  const [diagnostics, setDiagnostics] = useState<Diagnostic[]>([])
  const [refreshing, setRefreshing] = useState(false)

  const load = () => {
    setRefreshing(true)
    api.adminDiagnostics().then(d => setDiagnostics(d ?? [])).catch(() => {}).finally(() => setRefreshing(false))
  }
  useEffect(() => { load(); const t = setInterval(load, 15_000); return () => clearInterval(t) }, [])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          Drift between the database and the live system. Auto-refreshes every 15s.
        </p>
        <Button variant="ghost" size="sm" onClick={load} disabled={refreshing}>{refreshing ? 'Checking…' : 'Refresh now'}</Button>
      </div>

      {diagnostics.length === 0 ? (
        <Empty title="All clear" hint="No drift detected." />
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
    </div>
  )
}
