import { useEffect, useMemo, useState } from 'react'
import { api, type WGProfile, type WGAssignment, type WGHolder, type User } from '../api/client'
import { Plus, Search, KeyRound, Users, Trash2, UserPlus, X, Unplug } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet'
import { PageHeader } from '@/components/PageHeader'
import { Empty } from '@/components/Empty'

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result).split(',')[1] ?? '')
    reader.onerror = () => reject(new Error('could not read file'))
    reader.readAsDataURL(file)
  })
}

function Tag({ tone = 'muted', children }: { tone?: 'muted' | 'ok' | 'warn'; children: React.ReactNode }) {
  const cls =
    tone === 'ok' ? 'bg-primary/10 text-primary'
      : tone === 'warn' ? 'bg-warning/10 text-warning'
        : 'bg-secondary text-muted-foreground'
  return <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium ${cls}`}>{children}</span>
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

function duration(seconds: number): string {
  if (seconds < 60) return 'just now'
  const m = Math.floor(seconds / 60)
  if (m < 60) return `${m} min`
  const h = Math.floor(m / 60)
  return h < 24 ? `${h} h ${m % 60} min` : `${Math.floor(h / 24)} d ${h % 24} h`
}

function holderText(h: WGHolder): string {
  return `${h.name}${h.device ? ` on ${h.device}` : ''}, ${duration(h.held_seconds)}`
}

const SHARED_HINT = 'A shared profile can be assigned to many users. It is still one WireGuard peer, so only one device can be connected at a time; the others see who is using it. A profile that is not shared can be assigned to one user only.'

export default function WireGuardProfiles() {
  const [profiles, setProfiles] = useState<WGProfile[]>([])
  const [query, setQuery] = useState('')
  const [uploading, setUploading] = useState(false)
  const [manage, setManage] = useState<WGProfile | null>(null)

  const load = () => { api.adminListWGProfiles().then(d => setProfiles(d ?? [])).catch(() => {}) }
  useEffect(() => {
    load()
    const t = setInterval(load, 15000) // keep "in use by" current
    return () => clearInterval(t)
  }, [])

  const filtered = useMemo(() => {
    const q = query.toLowerCase()
    if (!q) return profiles
    return profiles.filter(p => p.name.toLowerCase().includes(q) || (p.description ?? '').toLowerCase().includes(q))
  }, [profiles, query])

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <PageHeader
        title="WireGuard profiles"
        description="Upload .conf profiles for WireGuard servers outside ProIdentity and assign them to users. Profiles are stored encrypted, served only to assigned users, and used on one device at a time."
        actions={<Button onClick={() => setUploading(true)}><Plus className="w-4 h-4" /> Upload Profile</Button>}
      />

      <div className="relative mb-4">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
        <Input value={query} onChange={e => setQuery(e.target.value)} placeholder={`Search ${profiles.length} ${profiles.length === 1 ? 'profile' : 'profiles'}…`} className="pl-9 max-w-md" />
      </div>

      {filtered.length === 0 ? (
        <Empty
          icon={KeyRound}
          title={profiles.length === 0 ? 'No WireGuard profiles yet' : 'No matches'}
          hint={profiles.length === 0 ? 'Upload a WireGuard .conf. Assign it to one user, or mark it shared to give it to several.' : 'Try a different search term.'}
          action={profiles.length === 0 ? <Button onClick={() => setUploading(true)}><Plus className="w-4 h-4" /> Upload Profile</Button> : undefined}
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
          {filtered.map(p => (
            <button key={p.id} onClick={() => setManage(p)}
              className="group text-left rounded-xl border border-border bg-card hover:border-primary/40 hover:bg-card/80 transition-colors p-4">
              <div className="flex items-start justify-between gap-3 mb-3">
                <div className="min-w-0">
                  <p className="font-semibold text-sm truncate">{p.name}</p>
                  {p.description && <p className="text-[11px] text-muted-foreground truncate mt-0.5">{p.description}</p>}
                </div>
                {p.shared ? <Tag tone="ok"><Users className="w-3 h-3" /> Shared</Tag> : <Tag>Single user</Tag>}
              </div>
              {(p.address || p.endpoint) && (
                <p className="text-[11px] font-mono text-muted-foreground truncate mb-2">
                  {[p.address, p.endpoint].filter(Boolean).join('  →  ')}
                </p>
              )}
              <div className="flex flex-wrap gap-1.5">
                <Tag>{p.assigned_count} assigned</Tag>
                {p.in_use_by
                  ? <Tag tone="warn">In use: {holderText(p.in_use_by)}</Tag>
                  : <Tag>Free</Tag>}
              </div>
            </button>
          ))}
        </div>
      )}

      <UploadSheet open={uploading} onClose={() => setUploading(false)} onSaved={() => { load(); setUploading(false) }} />
      <ManageSheet profile={manage} onClose={() => setManage(null)} onChanged={load} />
    </div>
  )
}

function UploadSheet({ open, onClose, onSaved }: { open: boolean; onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [shared, setShared] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (open) { setName(''); setDescription(''); setFile(null); setShared(false); setError('') }
  }, [open])

  const onFile = (f: File | null) => {
    setFile(f)
    if (f && !name) setName(f.name.replace(/\.conf$/i, ''))
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!file) { setError('Choose a .conf file'); return }
    setBusy(true); setError('')
    try {
      const config = await fileToBase64(file)
      await api.adminCreateWGProfile({ name, description: description || undefined, config, shared })
      onSaved()
    } catch (err: any) { setError(err.message ?? 'Upload failed') }
    finally { setBusy(false) }
  }

  return (
    <Sheet open={open} onOpenChange={v => { if (!v) onClose() }}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Upload WireGuard Profile</SheetTitle>
          <SheetDescription>A client .conf with its [Interface] private key and the server [Peer]. It is encrypted before it is stored; keys are never shown here.</SheetDescription>
        </SheetHeader>
        <form onSubmit={submit} className="contents">
          <SheetBody>
            <div className="space-y-4">
              {error && <p className="text-sm text-destructive">{error}</p>}
              <Field label=".conf file">
                <Input type="file" accept=".conf,text/plain" onChange={e => onFile(e.target.files?.[0] ?? null)} required />
              </Field>
              <Field label="Display name">
                <Input value={name} onChange={e => setName(e.target.value)} placeholder="Branch office" required />
              </Field>
              <Field label="Description" hint="Optional">
                <Input value={description} onChange={e => setDescription(e.target.value)} placeholder="Maintenance access to the branch router" />
              </Field>
              <div className="rounded-md border border-border bg-card/40 p-3">
                <label className="flex items-start gap-2 cursor-pointer">
                  <input type="checkbox" className="mt-1" checked={shared} onChange={e => setShared(e.target.checked)} />
                  <div>
                    <p className="text-sm">Shared between users</p>
                    <p className="text-xs text-muted-foreground">{SHARED_HINT}</p>
                  </div>
                </label>
              </div>
            </div>
          </SheetBody>
          <SheetFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? 'Uploading…' : 'Upload'}</Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}

function ManageSheet({ profile, onClose, onChanged }: { profile: WGProfile | null; onClose: () => void; onChanged: () => void }) {
  const [assignments, setAssignments] = useState<WGAssignment[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [addUserId, setAddUserId] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [editName, setEditName] = useState('')
  const [editDescription, setEditDescription] = useState('')
  const [editShared, setEditShared] = useState(false)
  const [newFile, setNewFile] = useState<File | null>(null)
  const [saved, setSaved] = useState(false)
  const [holder, setHolder] = useState<WGHolder | undefined>()
  const open = profile !== null

  const load = (id: string) => {
    api.adminWGAssignments(id).then(d => setAssignments(d ?? [])).catch(() => {})
    api.listUsers().then(d => setUsers(d ?? [])).catch(() => {})
  }
  useEffect(() => {
    if (!profile) return
    setError(''); setAddUserId(''); setSaved(false); setNewFile(null)
    setEditName(profile.name); setEditDescription(profile.description ?? ''); setEditShared(profile.shared)
    setHolder(profile.in_use_by)
    load(profile.id)
  }, [profile])

  const dirty = !!profile && (editName !== profile.name || editDescription !== (profile.description ?? '') || editShared !== profile.shared || !!newFile)
  // Only one user may hold a profile that isn't shared (as saved).
  const canAdd = !!profile?.shared || assignments.length === 0

  const saveDetails = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!profile) return
    setBusy(true); setError(''); setSaved(false)
    try {
      await api.adminUpdateWGProfile(profile.id, {
        name: editName,
        description: editDescription || undefined,
        shared: editShared,
        config: newFile ? await fileToBase64(newFile) : undefined,
      })
      Object.assign(profile, { name: editName.trim(), description: editDescription.trim() || undefined, shared: editShared })
      setNewFile(null); setSaved(true); onChanged()
    } catch (err: any) { setError(err.message ?? 'Save failed') }
    finally { setBusy(false) }
  }

  const unassigned = useMemo(() => {
    const taken = new Set(assignments.map(a => a.user_id))
    return users.filter(u => !taken.has(u.id) && u.is_active)
  }, [users, assignments])

  const add = async () => {
    if (!profile || !addUserId) return
    setBusy(true); setError('')
    try {
      await api.adminAssignWG(profile.id, addUserId)
      setAddUserId('')
      load(profile.id); onChanged()
    } catch (err: any) { setError(err.message ?? 'Assign failed') }
    finally { setBusy(false) }
  }

  const remove = async (userId: string) => {
    if (!profile) return
    try { await api.adminUnassignWG(profile.id, userId); load(profile.id); onChanged() } catch (err: any) { setError(err.message ?? 'Remove failed') }
  }

  const release = async () => {
    if (!profile || !holder) return
    if (!confirm(`Disconnect ${holder.name}${holder.device ? ` (${holder.device})` : ''} from "${profile.name}"? Their app disconnects within a minute and the profile becomes free.`)) return
    try { await api.adminReleaseWGProfile(profile.id); setHolder(undefined); onChanged() } catch (err: any) { setError(err.message ?? 'Release failed') }
  }

  const del = async () => {
    if (!profile || !confirm(`Delete profile "${profile.name}"? This removes it for all assigned users.`)) return
    try { await api.adminDeleteWGProfile(profile.id); onChanged(); onClose() } catch (err: any) { setError(err.message ?? 'Delete failed') }
  }

  return (
    <Sheet open={open} onOpenChange={v => { if (!v) onClose() }}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>{profile?.name}</SheetTitle>
          <SheetDescription className="font-mono">
            {[profile?.address, profile?.endpoint].filter(Boolean).join('  →  ') || 'WireGuard profile'}
          </SheetDescription>
        </SheetHeader>
        <SheetBody>
          <div className="space-y-5">
            {error && <p className="text-sm text-destructive">{error}</p>}

            <div className="rounded-md border border-border bg-card/40 p-3 flex items-center justify-between gap-3">
              <div className="min-w-0">
                <p className="text-xs font-medium text-muted-foreground">Current connection</p>
                <p className="text-sm truncate">{holder ? holderText(holder) : 'Free, nobody is connected'}</p>
              </div>
              {holder && (
                <Button type="button" size="sm" variant="outline" onClick={release}><Unplug className="w-4 h-4" /> Disconnect</Button>
              )}
            </div>

            <form onSubmit={saveDetails} className="space-y-3 rounded-md border border-border bg-card/40 p-3">
              <Field label="Display name">
                <Input value={editName} onChange={e => { setEditName(e.target.value); setSaved(false) }} required />
              </Field>
              <Field label="Description" hint="Optional">
                <Input value={editDescription} onChange={e => { setEditDescription(e.target.value); setSaved(false) }} />
              </Field>
              <Field label="Replace .conf" hint="Optional. Devices pick up the new file on their next connect.">
                <Input type="file" accept=".conf,text/plain" onChange={e => { setNewFile(e.target.files?.[0] ?? null); setSaved(false) }} />
              </Field>
              <label className="flex items-start gap-2 cursor-pointer">
                <input type="checkbox" className="mt-1" checked={editShared} onChange={e => { setEditShared(e.target.checked); setSaved(false) }} />
                <div>
                  <p className="text-sm">Shared between users</p>
                  <p className="text-xs text-muted-foreground">{SHARED_HINT}</p>
                </div>
              </label>
              <div className="flex items-center justify-end gap-2">
                {saved && <span className="text-xs text-muted-foreground">Saved</span>}
                <Button type="submit" size="sm" disabled={busy || !dirty || !editName.trim()}>Save details</Button>
              </div>
            </form>

            <div>
              <p className="text-xs font-medium text-muted-foreground mb-2">Assign to user</p>
              {canAdd ? (
                <div className="flex gap-2">
                  <select
                    value={addUserId}
                    onChange={e => setAddUserId(e.target.value)}
                    className="flex-1 h-9 rounded-md border border-border bg-background px-2 text-sm"
                  >
                    <option value="">Select a user…</option>
                    {unassigned.map(u => <option key={u.id} value={u.id}>{u.username} ({u.email})</option>)}
                  </select>
                  <Button onClick={add} disabled={busy || !addUserId}><UserPlus className="w-4 h-4" /> Add</Button>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">This profile isn't shared, so it can belong to one user only. Mark it as shared to assign more users.</p>
              )}
            </div>

            <div>
              <p className="text-xs font-medium text-muted-foreground mb-2">{assignments.length} assigned</p>
              {assignments.length === 0 ? (
                <p className="text-sm text-muted-foreground">No users assigned yet.</p>
              ) : (
                <div className="space-y-1.5">
                  {assignments.map(a => (
                    <div key={a.user_id} className="flex items-center justify-between gap-2 rounded-md border border-border bg-card/40 px-3 py-2">
                      <div className="min-w-0">
                        <p className="text-sm truncate">{a.username}</p>
                        <p className="text-[11px] text-muted-foreground truncate">{a.email}</p>
                      </div>
                      <button onClick={() => remove(a.user_id)} className="text-muted-foreground hover:text-destructive transition-colors" aria-label="Remove">
                        <X className="w-4 h-4" />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </SheetBody>
        <SheetFooter>
          <Button type="button" variant="ghost" onClick={del} className="text-destructive hover:text-destructive"><Trash2 className="w-4 h-4" /> Delete profile</Button>
          <Button type="button" onClick={onClose}>Done</Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
