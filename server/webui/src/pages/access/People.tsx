import { useEffect, useMemo, useState } from 'react'
import { api, type User, type ReachRow, type DenialRow, type ResourceGroup } from '../../api/client'
import { Plus, Trash2, KeyRound, Pencil, Shield, ShieldOff, Search, ChevronRight, ShieldAlert, Package, Lock } from 'lucide-react'
import { Activity } from '@/components/Activity'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet'
import { Combobox } from '@/components/ui/combobox'
import { StatusPill } from '@/components/StatusPill'
import { MonoChip } from '@/components/MonoChip'
import { Chip } from '@/components/Chip'
import { Empty } from '@/components/Empty'
import { ConfirmDelete } from '@/components/ConfirmDelete'
import { DangerZone, DangerAction } from '@/components/DangerZone'

type Mode = { kind: 'closed' } | { kind: 'create' } | { kind: 'edit'; id: string }

export default function People() {
  const [users, setUsers] = useState<User[]>([])
  const [allServers, setAllServers] = useState<{ id: string; name: string; subnet: string }[]>([])
  const [query, setQuery] = useState('')
  const [mode, setMode] = useState<Mode>({ kind: 'closed' })
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const load = () => {
    api.listUsers().then(d => setUsers(d ?? []))
    api.adminListServers().then(d => setAllServers((d ?? []).map(s => ({ id: s.id, name: s.name, subnet: s.subnet }))))
  }
  useEffect(() => { load() }, [])

  const filtered = useMemo(() => {
    const q = query.toLowerCase()
    if (!q) return users
    return users.filter(u =>
      u.username.toLowerCase().includes(q) ||
      u.email.toLowerCase().includes(q) ||
      `${u.first_name} ${u.last_name}`.toLowerCase().includes(q),
    )
  }, [users, query])

  const selected = users.find(u => u.id === selectedId) ?? null

  return (
    <div className="flex gap-3">
      <div className="flex-1 min-w-0 space-y-3">
        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <Input
              value={query}
              onChange={e => setQuery(e.target.value)}
              placeholder={`Search ${users.length} ${users.length === 1 ? 'user' : 'users'}…`}
              className="pl-9"
            />
          </div>
          <Button onClick={() => setMode({ kind: 'create' })}>
            <Plus className="w-4 h-4" /> New User
          </Button>
        </div>

        {filtered.length === 0 ? (
          <Empty
            title={users.length === 0 ? 'No users yet' : 'No matches'}
            hint={users.length === 0 ? 'Create the first user account.' : 'Try a different search term.'}
            action={users.length === 0 ? <Button onClick={() => setMode({ kind: 'create' })}><Plus className="w-4 h-4" /> New User</Button> : undefined}
          />
        ) : (
          <div className="rounded-xl border border-border bg-card overflow-hidden">
            {filtered.map(u => (
              <button
                key={u.id}
                onClick={() => setSelectedId(u.id)}
                className="w-full grid grid-cols-[auto_1fr_auto_auto] gap-3 items-center text-left px-4 py-3 border-b border-border last:border-0 hover:bg-secondary/30 transition-colors cursor-pointer"
              >
                <div className="w-8 h-8 rounded-full bg-primary/15 border border-primary/25 flex items-center justify-center shrink-0">
                  <span className="text-xs font-bold text-primary">{u.username[0]?.toUpperCase()}</span>
                </div>
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">
                    {u.username}
                    {(u.first_name || u.last_name) && (
                      <span className="text-muted-foreground font-normal ml-2">{u.first_name} {u.last_name}</span>
                    )}
                  </p>
                  <p className="text-[11px] text-muted-foreground truncate">{u.email}</p>
                </div>
                <div className="flex items-center gap-1.5">
                  {u.is_admin && <StatusPill kind="info" label="admin" />}
                  {!u.is_active && <StatusPill kind="idle" label="disabled" />}
                  {u.totp_enabled && <StatusPill kind="ok" label="2FA" />}
                </div>
                <ChevronRight className="w-4 h-4 text-muted-foreground" />
              </button>
            ))}
          </div>
        )}
      </div>

      <PersonSheet
        mode={mode}
        onClose={() => setMode({ kind: 'closed' })}
        onSaved={() => { load(); setMode({ kind: 'closed' }) }}
        editing={mode.kind === 'edit' ? users.find(u => u.id === mode.id) : undefined}
      />

      {selected && (
        <PersonDrawer
          user={selected}
          allServers={allServers}
          onClose={() => setSelectedId(null)}
          onChanged={() => { load() }}
          onEdit={() => { setMode({ kind: 'edit', id: selected.id }); setSelectedId(null) }}
          onDelete={() => { load(); setSelectedId(null) }}
        />
      )}
    </div>
  )
}

function PersonSheet({
  mode, onClose, onSaved, editing,
}: {
  mode: Mode
  onClose: () => void
  onSaved: () => void
  editing?: User
}) {
  const open = mode.kind !== 'closed'
  const isEdit = mode.kind === 'edit'

  const [form, setForm] = useState({
    username: '', email: '', password: '', first_name: '', last_name: '', is_admin: false,
  })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (mode.kind === 'create') {
      setForm({ username: '', email: '', password: '', first_name: '', last_name: '', is_admin: false })
    } else if (mode.kind === 'edit' && editing) {
      setForm({
        username: editing.username,
        email: editing.email,
        password: '',
        first_name: editing.first_name ?? '',
        last_name: editing.last_name ?? '',
        is_admin: editing.is_admin,
      })
    }
    setError('')
  }, [mode.kind, editing?.id])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true); setError('')
    try {
      if (isEdit && editing) {
        const patch: any = { email: form.email, first_name: form.first_name, last_name: form.last_name, is_admin: form.is_admin }
        if (form.password) patch.password = form.password
        await api.updateUser(editing.id, patch)
      } else {
        await api.createUser({ ...form })
      }
      onSaved()
    } catch (e: any) { setError(e.message ?? 'Failed') }
    finally { setBusy(false) }
  }

  return (
    <Sheet open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>{isEdit ? `Edit ${editing?.username}` : 'New User'}</SheetTitle>
          <SheetDescription>
            {isEdit ? 'Change profile details, role, or reset password.' : 'Create a new account. They can sign in immediately.'}
          </SheetDescription>
        </SheetHeader>
        <form onSubmit={submit} className="contents">
          <SheetBody>
            <div className="space-y-4">
              {error && <p className="text-sm text-destructive">{error}</p>}
              <div className="grid grid-cols-2 gap-3">
                <Field label="Username">
                  <Input value={form.username} onChange={e => setForm(p => ({ ...p, username: e.target.value }))} disabled={isEdit} required autoFocus />
                </Field>
                <Field label="Email">
                  <Input type="email" value={form.email} onChange={e => setForm(p => ({ ...p, email: e.target.value }))} />
                </Field>
                <Field label="First name">
                  <Input value={form.first_name} onChange={e => setForm(p => ({ ...p, first_name: e.target.value }))} />
                </Field>
                <Field label="Last name">
                  <Input value={form.last_name} onChange={e => setForm(p => ({ ...p, last_name: e.target.value }))} />
                </Field>
              </div>
              <Field label={isEdit ? 'New password (leave blank to keep)' : 'Password'} hint={isEdit ? 'Only set this if you want to reset.' : 'They\'ll use this to sign in.'}>
                <Input type="password" value={form.password} onChange={e => setForm(p => ({ ...p, password: e.target.value }))} required={!isEdit} />
              </Field>
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" checked={form.is_admin} onChange={e => setForm(p => ({ ...p, is_admin: e.target.checked }))} />
                <span className="text-sm">Administrator <span className="text-muted-foreground">— full access to this panel</span></span>
              </label>
            </div>
          </SheetBody>
          <SheetFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? 'Saving…' : isEdit ? 'Save' : 'Create User'}</Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}

function PersonDrawer({
  user, allServers, onClose, onChanged, onEdit, onDelete,
}: {
  user: User
  allServers: { id: string; name: string; subnet: string }[]
  onClose: () => void
  onChanged: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const [servers, setServers] = useState<{ id: string; name: string; subnet: string }[]>([])
  const [reach, setReach] = useState<ReachRow[]>([])
  const [denials, setDenials] = useState<DenialRow[]>([])
  const [confirm, setConfirm] = useState(false)
  const [showDisable2FA, setShowDisable2FA] = useState(false)
  const [adminPw, setAdminPw] = useState('')
  const [disable2FAError, setDisable2FAError] = useState('')
  const [disable2FALoading, setDisable2FALoading] = useState(false)
  const [userBundles, setUserBundles] = useState<Record<string, ResourceGroup[]>>({})
  const [serverAllowedBundles, setServerAllowedBundles] = useState<Record<string, ResourceGroup[]>>({})

  const load = () => {
    api.adminUserServers(user.id).then(d => {
      const srvs = d ?? []
      setServers(srvs)
      for (const s of srvs) {
        api.adminUserBundles(user.id, s.id).then(b => setUserBundles(prev => ({ ...prev, [s.id]: b ?? [] })))
        api.adminServerBundles(s.id).then(b => setServerAllowedBundles(prev => ({ ...prev, [s.id]: b ?? [] })))
      }
    })
    api.userReach(user.id).then(d => setReach(d ?? []))
    api.adminDenials({ user_id: user.id, limit: 20 }).then(d => setDenials(d.items ?? [])).catch(() => {})
  }
  useEffect(load, [user.id])

  const addServer = async (sid: string) => { await api.adminAddUserServer(user.id, sid); load() }
  const removeServer = async (sid: string) => { await api.adminRemoveUserServer(user.id, sid); load() }
  const addBundle = async (serverId: string, bundleId: string) => { await api.adminAddUserBundle(user.id, serverId, bundleId); load() }
  const removeBundle = async (serverId: string, bundleId: string) => { await api.adminRemoveUserBundle(user.id, serverId, bundleId); load() }

  // Group reach by resource
  const reachByResource = useMemo(() => {
    const m: Record<string, ReachRow[]> = {}
    for (const r of reach) (m[r.resource_id] ??= []).push(r)
    return m
  }, [reach])

  const reachableCount = Object.keys(reachByResource).length

  return (
    <Sheet open onOpenChange={(v) => { if (!v) onClose() }}>
      <SheetContent width="w-full sm:max-w-2xl">
        <SheetHeader>
          <div className="flex items-start gap-3">
            <div className="w-10 h-10 rounded-full bg-primary/15 border border-primary/25 flex items-center justify-center shrink-0">
              <span className="text-sm font-bold text-primary">{user.username[0]?.toUpperCase()}</span>
            </div>
            <div className="flex-1 min-w-0">
              <SheetTitle>
                {user.username}
                {(user.first_name || user.last_name) && <span className="text-muted-foreground font-normal ml-2">{user.first_name} {user.last_name}</span>}
              </SheetTitle>
              <SheetDescription>{user.email}</SheetDescription>
              <div className="flex items-center gap-1.5 mt-2">
                {user.is_admin && <StatusPill kind="info" label="admin" />}
                {user.is_active ? <StatusPill kind="ok" label="active" /> : <StatusPill kind="idle" label="disabled" />}
                {user.totp_enabled && <StatusPill kind="ok" label="2FA on" />}
              </div>
            </div>
          </div>
        </SheetHeader>
        <SheetBody>
          <div className="space-y-6">
            <Section title="Reach" hint={`${reachableCount} resource${reachableCount === 1 ? '' : 's'} reachable`}>
              {reachableCount === 0 ? (
                <p className="text-sm text-muted-foreground italic">
                  No reachable resources yet. Give them access to a server below and assign bundles to control what they can reach.
                </p>
              ) : (
                <div className="space-y-2">
                  {Object.entries(reachByResource).map(([rid, rows]) => {
                    const first = rows[0]
                    const cidr = first.type === 'network' && first.mask != null ? `${first.ip_address}/${first.mask}` : first.ip_address
                    return (
                      <div key={rid} className="rounded-md border border-border bg-card/60 px-3 py-2.5">
                        <div className="flex items-center justify-between gap-2 mb-1.5">
                          <p className="text-sm font-medium truncate">{first.resource_name}</p>
                          <MonoChip value={cidr} bare />
                        </div>
                        <ul className="space-y-1 text-[11px] text-muted-foreground">
                          {rows.map((r, i) => (
                            <li key={`${r.server_id}-${r.bundle_id}-${i}`} className="flex items-center gap-1.5">
                              on <span className="text-foreground/85">{r.server_name}</span>
                              {' via '}<span className="text-foreground/85">{r.bundle_name}</span>
                            </li>
                          ))}
                        </ul>
                      </div>
                    )
                  })}
                </div>
              )}
            </Section>

            <Section title="Activity" hint="Last 24 hours">
              <Activity userId={user.id} topBy="resource" topTitle="Top resources reached" emptyLabel="No traffic recorded yet — try connecting via the desktop client." />
            </Section>

            {denials.length > 0 && (
              <Section title="Recent denials" hint={`${denials.length} blocked`}>
                <div className="rounded-md border border-warning/30 bg-warning/5 divide-y divide-warning/15">
                  {denials.slice(0, 8).map(d => (
                    <div key={d.id} className="px-3 py-2 flex items-center gap-2 text-xs">
                      <ShieldAlert className="w-3.5 h-3.5 text-warning shrink-0" />
                      <span className="font-mono truncate">{d.dst_ip}{d.dst_port ? `:${d.dst_port}` : ''}</span>
                      <span className="text-muted-foreground uppercase">{d.proto}</span>
                      <span className="text-muted-foreground ml-auto">×{d.count}</span>
                    </div>
                  ))}
                </div>
                <p className="text-[11px] text-muted-foreground mt-2">
                  This user tried to reach destinations they aren't allowed to. Often a hint that a resource is missing from one of their roles.
                </p>
              </Section>
            )}

            <Section title="Servers & bundles" hint={`${servers.length} server${servers.length === 1 ? '' : 's'}`}>
              <div className="space-y-3">
                {servers.map(s => {
                  const assigned = userBundles[s.id] ?? []
                  const allowed = serverAllowedBundles[s.id] ?? []
                  const available = allowed.filter(b => !assigned.some(a => a.id === b.id))
                  return (
                    <div key={s.id} className="rounded-md border border-border bg-card/60 px-3 py-2.5 space-y-2">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <Chip label={s.name} hint={s.subnet} tone="warning" onRemove={() => removeServer(s.id)} />
                        </div>
                      </div>
                      <div className="pl-1 space-y-1.5">
                        <p className="text-[11px] text-muted-foreground uppercase tracking-wider font-semibold flex items-center gap-1">
                          <Package className="w-3 h-3" /> Assigned bundles
                        </p>
                        <div className="flex flex-wrap items-center gap-1.5">
                          {assigned.map(b => <Chip key={b.id} label={b.name} tone="primary" onRemove={() => removeBundle(s.id, b.id)} />)}
                          {available.length > 0 && (
                            <Combobox
                              placeholder="Assign bundle"
                              options={available.map(b => ({ value: b.id, label: b.name }))}
                              onSelect={(bid) => addBundle(s.id, bid)}
                            />
                          )}
                          {assigned.length === 0 && available.length === 0 && (
                            <p className="text-[11px] text-muted-foreground italic">No allowed bundles on this server.</p>
                          )}
                        </div>
                      </div>
                    </div>
                  )
                })}
                <Combobox
                  placeholder="Grant server access"
                  options={allServers.filter(s => !servers.some(x => x.id === s.id)).map(s => ({ value: s.id, label: s.name, hint: s.subnet }))}
                  onSelect={addServer}
                />
              </div>
              <p className="text-[11px] text-muted-foreground mt-2">
                Assign bundles per server to control which resources this user can reach. Only bundles allowed on the server are available.
              </p>
            </Section>

            <Section title="Account">
              <div className="grid grid-cols-2 gap-2">
                <Button variant="ghost" onClick={onEdit}><Pencil className="w-4 h-4" /> Edit profile</Button>
                <Button variant="ghost" onClick={async () => { await api.updateUser(user.id, { is_active: !user.is_active }); onChanged() }}>
                  {user.is_active ? <><ShieldOff className="w-4 h-4" /> Disable</> : <><Shield className="w-4 h-4" /> Enable</>}
                </Button>
                <Button variant="ghost" onClick={async () => { await api.updateUser(user.id, { is_admin: !user.is_admin }); onChanged() }}>
                  <KeyRound className="w-4 h-4" /> {user.is_admin ? 'Demote to user' : 'Promote to admin'}
                </Button>
                {user.totp_enabled && (
                  <Button variant="ghost" onClick={() => { setShowDisable2FA(true); setAdminPw(''); setDisable2FAError('') }}>
                    <Lock className="w-4 h-4" /> Disable 2FA
                  </Button>
                )}
              </div>
              {showDisable2FA && (
                <div className="mt-3 rounded-md border border-warning/30 bg-warning/5 p-3 space-y-2">
                  <p className="text-sm font-medium">Enter your admin password to disable this user's 2FA</p>
                  <Input type="password" placeholder="Your password" value={adminPw} onChange={e => setAdminPw(e.target.value)} />
                  {disable2FAError && <p className="text-xs text-destructive">{disable2FAError}</p>}
                  <div className="flex gap-2">
                    <Button size="sm" variant="destructive" disabled={!adminPw || disable2FALoading} onClick={async () => {
                      setDisable2FALoading(true); setDisable2FAError('')
                      try {
                        await api.updateUser(user.id, { disable_totp: true, admin_password: adminPw })
                        setShowDisable2FA(false); setAdminPw(''); onChanged()
                      } catch (e: any) { setDisable2FAError(e.message || 'Failed') }
                      finally { setDisable2FALoading(false) }
                    }}>
                      {disable2FALoading ? 'Disabling…' : 'Confirm'}
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => setShowDisable2FA(false)}>Cancel</Button>
                  </div>
                </div>
              )}
            </Section>

            <DangerZone>
              <DangerAction
                title="Delete this user"
                description="Removes the account and all assignments. Active sessions are not terminated automatically."
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
        title={`Delete user "${user.username}"`}
        description={<>This permanently removes the account and unlinks them from every role. Their active sessions stay live until they expire.</>}
        confirmText={user.username}
        actionLabel="Delete user"
        onConfirm={async () => { await api.deleteUser(user.id); onDelete() }}
      />
    </Sheet>
  )
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label className="text-xs">{label}</Label>
      {children}
      {hint && <p className="text-[11px] text-muted-foreground">{hint}</p>}
    </div>
  )
}

function Section({ title, hint, children }: { title: string; hint?: string; children: React.ReactNode }) {
  return (
    <section>
      <div className="flex items-baseline justify-between mb-2">
        <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{title}</h3>
        {hint && <span className="text-[11px] text-muted-foreground">{hint}</span>}
      </div>
      {children}
    </section>
  )
}
