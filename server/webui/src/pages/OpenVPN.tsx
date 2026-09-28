import { useEffect, useMemo, useState } from 'react'
import { api, type OpenVPNProfile, type OpenVPNAssignment, type User } from '../api/client'
import { Plus, Search, FileKey, ShieldCheck, Trash2, UserPlus, X } from 'lucide-react'
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

export default function OpenVPN() {
  const [profiles, setProfiles] = useState<OpenVPNProfile[]>([])
  const [query, setQuery] = useState('')
  const [uploading, setUploading] = useState(false)
  const [manage, setManage] = useState<OpenVPNProfile | null>(null)

  const load = () => { api.adminListOpenVPN().then(d => setProfiles(d ?? [])).catch(() => {}) }
  useEffect(() => { load() }, [])

  const filtered = useMemo(() => {
    const q = query.toLowerCase()
    if (!q) return profiles
    return profiles.filter(p => p.name.toLowerCase().includes(q) || (p.description ?? '').toLowerCase().includes(q))
  }, [profiles, query])

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <PageHeader
        title="OpenVPN"
        description="Upload .ovpn profiles and assign them to users. Profiles are stored encrypted and served only to assigned users."
        actions={<Button onClick={() => setUploading(true)}><Plus className="w-4 h-4" /> Upload Profile</Button>}
      />

      <div className="relative mb-4">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
        <Input value={query} onChange={e => setQuery(e.target.value)} placeholder={`Search ${profiles.length} ${profiles.length === 1 ? 'profile' : 'profiles'}…`} className="pl-9 max-w-md" />
      </div>

      {filtered.length === 0 ? (
        <Empty
          icon={FileKey}
          title={profiles.length === 0 ? 'No OpenVPN profiles yet' : 'No matches'}
          hint={profiles.length === 0 ? 'Upload a .ovpn profile. It will be encrypted at rest and assignable to one or many users.' : 'Try a different search term.'}
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
                <Tag tone={p.dev_type === 'tap' ? 'warn' : 'muted'}>{p.dev_type.toUpperCase()}</Tag>
              </div>
              <div className="flex flex-wrap gap-1.5">
                {p.requires_totp && <Tag tone="ok"><ShieldCheck className="w-3 h-3" /> TOTP</Tag>}
                {p.auth_user_pass && <Tag>user/pass</Tag>}
                {p.dev_type === 'tap' && p.allow_custom_ip && <Tag>custom IP</Tag>}
                {p.autofill_name && <Tag>autofill: {p.autofill_name}</Tag>}
                <Tag>{p.assigned_count} assigned</Tag>
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

const AUTOFILL_HINT = 'Text your password manager searches for. While the connect form is open, the desktop app shows it in its window title, so a RoboForm login matched to exe://ProIdentity Access/*text* fills it. Leave empty to use the display name.'

function UploadSheet({ open, onClose, onSaved }: { open: boolean; onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState('')
  const [autofillName, setAutofillName] = useState('')
  const [description, setDescription] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [requiresTotp, setRequiresTotp] = useState(false)
  const [allowCustomIp, setAllowCustomIp] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (open) { setName(''); setAutofillName(''); setDescription(''); setFile(null); setRequiresTotp(false); setAllowCustomIp(false); setError('') }
  }, [open])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!file) { setError('Choose a .ovpn file'); return }
    setBusy(true); setError('')
    try {
      const config = await fileToBase64(file)
      await api.adminCreateOpenVPN({ name, autofill_name: autofillName || undefined, description: description || undefined, config, requires_totp: requiresTotp, allow_custom_ip: allowCustomIp })
      onSaved()
    } catch (err: any) { setError(err.message ?? 'Upload failed') }
    finally { setBusy(false) }
  }

  return (
    <Sheet open={open} onOpenChange={v => { if (!v) onClose() }}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Upload OpenVPN Profile</SheetTitle>
          <SheetDescription>TUN/TAP and the auth mode are auto-detected from the file. The profile is encrypted before it is stored.</SheetDescription>
        </SheetHeader>
        <form onSubmit={submit} className="contents">
          <SheetBody>
            <div className="space-y-4">
              {error && <p className="text-sm text-destructive">{error}</p>}
              <Field label="Display name">
                <Input value={name} onChange={e => setName(e.target.value)} placeholder="Corp OpenVPN" required autoFocus />
              </Field>
              <Field label="Description" hint="Optional">
                <Input value={description} onChange={e => setDescription(e.target.value)} placeholder="Legacy site-to-site profile" />
              </Field>
              <Field label="Autofill search name" hint={AUTOFILL_HINT}>
                <Input value={autofillName} onChange={e => setAutofillName(e.target.value)} placeholder={name || 'Same as display name'} />
              </Field>
              <Field label=".ovpn file">
                <Input type="file" accept=".ovpn,.conf,text/plain" onChange={e => setFile(e.target.files?.[0] ?? null)} required />
              </Field>
              <div className="rounded-md border border-border bg-card/40 p-3 space-y-3">
                <label className="flex items-start gap-2 cursor-pointer">
                  <input type="checkbox" className="mt-1" checked={requiresTotp} onChange={e => setRequiresTotp(e.target.checked)} />
                  <div>
                    <p className="text-sm">Requires TOTP (external)</p>
                    <p className="text-xs text-muted-foreground">The client will append a one-time code to the password field at connect. This is the profile's own 2FA, not ProIdentity's.</p>
                  </div>
                </label>
                <label className="flex items-start gap-2 cursor-pointer">
                  <input type="checkbox" className="mt-1" checked={allowCustomIp} onChange={e => setAllowCustomIp(e.target.checked)} />
                  <div>
                    <p className="text-sm">Allow custom IP (TAP only)</p>
                    <p className="text-xs text-muted-foreground">For TAP profiles, let the user set a static adapter address in the app. Ignored for TUN and on mobile.</p>
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

function ManageSheet({ profile, onClose, onChanged }: { profile: OpenVPNProfile | null; onClose: () => void; onChanged: () => void }) {
  const [assignments, setAssignments] = useState<OpenVPNAssignment[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [addUserId, setAddUserId] = useState('')
  const [addCustomIp, setAddCustomIp] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [editName, setEditName] = useState('')
  const [editAutofill, setEditAutofill] = useState('')
  const [editDescription, setEditDescription] = useState('')
  const [saved, setSaved] = useState(false)
  const open = profile !== null
  const isTap = profile?.dev_type === 'tap'
  const wantsCustomIp = isTap && profile?.allow_custom_ip

  const load = (id: string) => {
    api.adminOpenVPNAssignments(id).then(d => setAssignments(d ?? [])).catch(() => {})
    api.listUsers().then(d => setUsers(d ?? [])).catch(() => {})
  }
  useEffect(() => {
    if (!profile) return
    setError(''); setAddUserId(''); setAddCustomIp(''); setSaved(false)
    setEditName(profile.name); setEditAutofill(profile.autofill_name ?? ''); setEditDescription(profile.description ?? '')
    load(profile.id)
  }, [profile])

  const dirty = !!profile && (editName !== profile.name || editAutofill !== (profile.autofill_name ?? '') || editDescription !== (profile.description ?? ''))

  const saveDetails = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!profile) return
    setBusy(true); setError(''); setSaved(false)
    try {
      await api.adminUpdateOpenVPN(profile.id, {
        name: editName,
        autofill_name: editAutofill || undefined,
        description: editDescription || undefined,
        requires_totp: profile.requires_totp,
        allow_custom_ip: profile.allow_custom_ip,
      })
      Object.assign(profile, { name: editName.trim(), autofill_name: editAutofill.trim() || undefined, description: editDescription.trim() || undefined })
      setSaved(true); onChanged()
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
      await api.adminAssignOpenVPN(profile.id, addUserId, wantsCustomIp ? (addCustomIp || undefined) : undefined)
      setAddUserId(''); setAddCustomIp('')
      load(profile.id); onChanged()
    } catch (err: any) { setError(err.message ?? 'Assign failed') }
    finally { setBusy(false) }
  }

  const remove = async (userId: string) => {
    if (!profile) return
    try { await api.adminUnassignOpenVPN(profile.id, userId); load(profile.id); onChanged() } catch { /* ignore */ }
  }

  const del = async () => {
    if (!profile || !confirm(`Delete profile "${profile.name}"? This removes it for all assigned users.`)) return
    try { await api.adminDeleteOpenVPN(profile.id); onChanged(); onClose() } catch (err: any) { setError(err.message ?? 'Delete failed') }
  }

  return (
    <Sheet open={open} onOpenChange={v => { if (!v) onClose() }}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>{profile?.name}</SheetTitle>
          <SheetDescription>
            {profile?.dev_type.toUpperCase()} · {profile?.auth_user_pass ? 'username/password' : 'no user auth'}
            {profile?.requires_totp ? ' · requires TOTP' : ''}
          </SheetDescription>
        </SheetHeader>
        <SheetBody>
          <div className="space-y-5">
            {error && <p className="text-sm text-destructive">{error}</p>}

            <form onSubmit={saveDetails} className="space-y-3 rounded-md border border-border bg-card/40 p-3">
              <Field label="Display name">
                <Input value={editName} onChange={e => { setEditName(e.target.value); setSaved(false) }} required />
              </Field>
              <Field label="Autofill search name" hint={AUTOFILL_HINT}>
                <Input value={editAutofill} onChange={e => { setEditAutofill(e.target.value); setSaved(false) }} placeholder={editName || 'Same as display name'} />
              </Field>
              <Field label="Description" hint="Optional">
                <Input value={editDescription} onChange={e => { setEditDescription(e.target.value); setSaved(false) }} />
              </Field>
              <div className="flex items-center justify-end gap-2">
                {saved && <span className="text-xs text-muted-foreground">Saved</span>}
                <Button type="submit" size="sm" disabled={busy || !dirty || !editName.trim()}>Save details</Button>
              </div>
            </form>

            <div>
              <p className="text-xs font-medium text-muted-foreground mb-2">Assign to user</p>
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
              {wantsCustomIp && (
                <Input className="mt-2 font-mono" value={addCustomIp} onChange={e => setAddCustomIp(e.target.value)} placeholder="Custom TAP IP, e.g. 10.9.0.15 (optional)" />
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
                        <p className="text-[11px] text-muted-foreground truncate">{a.email}{a.custom_ip ? ` · ${a.custom_ip}` : ''}</p>
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
