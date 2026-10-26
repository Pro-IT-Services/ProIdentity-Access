import { useEffect, useMemo, useState } from 'react'
import { api, type OpenVPNProfile, type OpenVPNAssignment, type User } from '../api/client'
import { Plus, Search, FileKey, ShieldCheck, Trash2, UserPlus, X, Pencil, Users, Check } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
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

function Toggle({ checked, onChange, title, hint }: { checked: boolean; onChange: (v: boolean) => void; title: string; hint: string }) {
  return (
    <label className="flex items-start gap-2 cursor-pointer">
      <input type="checkbox" className="mt-1" checked={checked} onChange={e => onChange(e.target.checked)} />
      <div>
        <p className="text-sm">{title}</p>
        <p className="text-xs text-muted-foreground">{hint}</p>
      </div>
    </label>
  )
}

const AUTOFILL_HINT = 'Text your password manager searches for. While the connect form is open, the desktop app shows it in its window title, so a RoboForm login matched to exe://ProIdentity Access/*text* fills it. Leave empty to use the display name.'
const TOTP_HINT = "The client will append a one-time code to the password field at connect. This is the profile's own 2FA, not ProIdentity's."
const CUSTOM_IP_HINT = 'For TAP profiles, let the user set a static adapter address in the app. Ignored for TUN and on mobile.'

type ManageTab = 'settings' | 'users'

export default function OpenVPN() {
  const [profiles, setProfiles] = useState<OpenVPNProfile[]>([])
  const [query, setQuery] = useState('')
  const [uploading, setUploading] = useState(false)
  const [manage, setManage] = useState<{ profile: OpenVPNProfile; tab: ManageTab } | null>(null)

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
            <div key={p.id} className="rounded-xl border border-border bg-card hover:border-primary/40 transition-colors p-4 flex flex-col">
              <button onClick={() => setManage({ profile: p, tab: 'settings' })} className="text-left flex-1">
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
                </div>
              </button>
              <div className="flex items-center gap-1 mt-3 pt-3 border-t border-border">
                <Button size="sm" variant="ghost" onClick={() => setManage({ profile: p, tab: 'settings' })}><Pencil className="w-3.5 h-3.5" /> Edit</Button>
                <Button size="sm" variant="ghost" onClick={() => setManage({ profile: p, tab: 'users' })}><Users className="w-3.5 h-3.5" /> {p.assigned_count} assigned</Button>
              </div>
            </div>
          ))}
        </div>
      )}

      <UploadSheet open={uploading} onClose={() => setUploading(false)} onSaved={() => { load(); setUploading(false) }} />
      <ManageSheet target={manage} onClose={() => setManage(null)} onChanged={load} />
    </div>
  )
}

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
                <Toggle checked={requiresTotp} onChange={setRequiresTotp} title="Requires TOTP (external)" hint={TOTP_HINT} />
                <Toggle checked={allowCustomIp} onChange={setAllowCustomIp} title="Allow custom IP (TAP only)" hint={CUSTOM_IP_HINT} />
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

function ManageSheet({ target, onClose, onChanged }: { target: { profile: OpenVPNProfile; tab: ManageTab } | null; onClose: () => void; onChanged: () => void }) {
  const profile = target?.profile ?? null
  const [tab, setTab] = useState<ManageTab>('settings')
  const [assignments, setAssignments] = useState<OpenVPNAssignment[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [addUserId, setAddUserId] = useState('')
  const [addCustomIp, setAddCustomIp] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  // Settings form
  const [name, setName] = useState('')
  const [autofill, setAutofill] = useState('')
  const [description, setDescription] = useState('')
  const [requiresTotp, setRequiresTotp] = useState(false)
  const [allowCustomIp, setAllowCustomIp] = useState(false)
  const [devType, setDevType] = useState<'tun' | 'tap'>('tun')
  const [newFile, setNewFile] = useState<File | null>(null)
  const [fileKey, setFileKey] = useState(0) // resets the file input
  // Per-user IP being edited
  const [ipEdit, setIpEdit] = useState<{ userId: string; value: string } | null>(null)

  const open = profile !== null
  const isTap = (profile?.dev_type ?? devType) === 'tap'
  const wantsCustomIp = profile?.dev_type === 'tap' && profile?.allow_custom_ip

  const load = (id: string) => {
    api.adminOpenVPNAssignments(id).then(d => setAssignments(d ?? [])).catch(() => {})
    api.listUsers().then(d => setUsers(d ?? [])).catch(() => {})
  }
  useEffect(() => {
    if (!profile || !target) return
    setTab(target.tab)
    setError(''); setSaved(false); setAddUserId(''); setAddCustomIp(''); setIpEdit(null)
    setName(profile.name); setAutofill(profile.autofill_name ?? ''); setDescription(profile.description ?? '')
    setRequiresTotp(profile.requires_totp); setAllowCustomIp(profile.allow_custom_ip)
    setDevType(profile.dev_type === 'tap' ? 'tap' : 'tun')
    setNewFile(null); setFileKey(k => k + 1)
    load(profile.id)
  }, [target])

  const dirty = !!profile && (
    name !== profile.name || autofill !== (profile.autofill_name ?? '') || description !== (profile.description ?? '') ||
    requiresTotp !== profile.requires_totp || allowCustomIp !== profile.allow_custom_ip || devType !== profile.dev_type || !!newFile
  )

  const saveSettings = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!profile) return
    setBusy(true); setError(''); setSaved(false)
    try {
      await api.adminUpdateOpenVPN(profile.id, {
        name,
        autofill_name: autofill || undefined,
        description: description || undefined,
        requires_totp: requiresTotp,
        allow_custom_ip: allowCustomIp,
        dev_type: devType,
        config: newFile ? await fileToBase64(newFile) : undefined,
      })
      Object.assign(profile, {
        name: name.trim(), autofill_name: autofill.trim() || undefined, description: description.trim() || undefined,
        requires_totp: requiresTotp, allow_custom_ip: allowCustomIp, dev_type: devType,
      })
      setNewFile(null); setFileKey(k => k + 1)
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
      await api.adminAssignOpenVPN(profile.id, addUserId, wantsCustomIp ? (addCustomIp.trim() || undefined) : undefined)
      setAddUserId(''); setAddCustomIp('')
      load(profile.id); onChanged()
    } catch (err: any) { setError(err.message ?? 'Assign failed') }
    finally { setBusy(false) }
  }

  // Assigning again updates the user's custom IP.
  const saveIp = async () => {
    if (!profile || !ipEdit) return
    setBusy(true); setError('')
    try {
      await api.adminAssignOpenVPN(profile.id, ipEdit.userId, ipEdit.value.trim() || undefined)
      setIpEdit(null); load(profile.id)
    } catch (err: any) { setError(err.message ?? 'Save failed') }
    finally { setBusy(false) }
  }

  const remove = async (userId: string) => {
    if (!profile) return
    try { await api.adminUnassignOpenVPN(profile.id, userId); load(profile.id); onChanged() } catch (err: any) { setError(err.message ?? 'Remove failed') }
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
          {error && <p className="text-sm text-destructive mb-3">{error}</p>}
          <Tabs value={tab} onValueChange={v => { setTab(v as ManageTab); setError('') }}>
            <TabsList>
              <TabsTrigger value="settings"><Pencil className="w-3.5 h-3.5 mr-1.5" /> Settings</TabsTrigger>
              <TabsTrigger value="users"><Users className="w-3.5 h-3.5 mr-1.5" /> Users ({assignments.length})</TabsTrigger>
            </TabsList>

            <TabsContent value="settings">
              <form onSubmit={saveSettings} className="space-y-4">
                <Field label="Display name">
                  <Input value={name} onChange={e => { setName(e.target.value); setSaved(false) }} required />
                </Field>
                <Field label="Description" hint="Optional">
                  <Input value={description} onChange={e => { setDescription(e.target.value); setSaved(false) }} />
                </Field>
                <Field label="Autofill search name" hint={AUTOFILL_HINT}>
                  <Input value={autofill} onChange={e => { setAutofill(e.target.value); setSaved(false) }} placeholder={name || 'Same as display name'} />
                </Field>
                <Field label="Connection type" hint="Detected from the file. Change it only if the detection is wrong.">
                  <select value={devType} onChange={e => { setDevType(e.target.value as 'tun' | 'tap'); setSaved(false) }}
                    className="w-full h-9 rounded-md border border-border bg-background px-2 text-sm">
                    <option value="tun">TUN (routed)</option>
                    <option value="tap">TAP (bridged)</option>
                  </select>
                </Field>
                <div className="rounded-md border border-border bg-card/40 p-3 space-y-3">
                  <Toggle checked={requiresTotp} onChange={v => { setRequiresTotp(v); setSaved(false) }} title="Requires TOTP (external)" hint={TOTP_HINT} />
                  <Toggle checked={allowCustomIp} onChange={v => { setAllowCustomIp(v); setSaved(false) }} title="Allow custom IP (TAP only)" hint={CUSTOM_IP_HINT} />
                </div>
                <Field label="Replace .ovpn file" hint="Optional. TUN/TAP and the auth mode are detected again from the new file; users get it on their next connect.">
                  <Input key={fileKey} type="file" accept=".ovpn,.conf,text/plain" onChange={e => { setNewFile(e.target.files?.[0] ?? null); setSaved(false) }} />
                </Field>
                <div className="flex items-center justify-end gap-2">
                  {saved && <span className="inline-flex items-center gap-1 text-xs text-muted-foreground"><Check className="w-3.5 h-3.5" /> Saved</span>}
                  <Button type="submit" disabled={busy || !dirty || !name.trim()}>{busy ? 'Saving…' : 'Save changes'}</Button>
                </div>
              </form>
            </TabsContent>

            <TabsContent value="users">
              <div className="space-y-5">
                <div>
                  <p className="text-xs font-medium text-muted-foreground mb-2">Assign to user</p>
                  <div className="flex gap-2">
                    <select value={addUserId} onChange={e => setAddUserId(e.target.value)}
                      className="flex-1 h-9 rounded-md border border-border bg-background px-2 text-sm">
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
                        <div key={a.user_id} className="rounded-md border border-border bg-card/40 px-3 py-2">
                          <div className="flex items-center justify-between gap-2">
                            <div className="min-w-0">
                              <p className="text-sm truncate">{a.username}</p>
                              <p className="text-[11px] text-muted-foreground truncate">
                                {a.email}{isTap && a.custom_ip ? <> · <span className="font-mono">{a.custom_ip}</span></> : null}
                              </p>
                            </div>
                            <div className="flex items-center gap-1 shrink-0">
                              {wantsCustomIp && ipEdit?.userId !== a.user_id && (
                                <Button size="sm" variant="ghost" onClick={() => setIpEdit({ userId: a.user_id, value: a.custom_ip ?? '' })}>
                                  <Pencil className="w-3.5 h-3.5" /> IP
                                </Button>
                              )}
                              <button onClick={() => remove(a.user_id)} className="p-1 text-muted-foreground hover:text-destructive transition-colors" aria-label={`Remove ${a.username}`}>
                                <X className="w-4 h-4" />
                              </button>
                            </div>
                          </div>
                          {ipEdit?.userId === a.user_id && (
                            <div className="flex gap-2 mt-2">
                              <Input className="font-mono h-8" autoFocus value={ipEdit.value}
                                onChange={e => setIpEdit({ userId: a.user_id, value: e.target.value })}
                                onKeyDown={e => { if (e.key === 'Enter') saveIp(); if (e.key === 'Escape') setIpEdit(null) }}
                                placeholder="Empty: the user sets it in the app" />
                              <Button size="sm" onClick={saveIp} disabled={busy}>Save</Button>
                              <Button size="sm" variant="ghost" onClick={() => setIpEdit(null)}>Cancel</Button>
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </TabsContent>
          </Tabs>
        </SheetBody>
        <SheetFooter>
          <Button type="button" variant="ghost" onClick={del} className="text-destructive hover:text-destructive"><Trash2 className="w-4 h-4" /> Delete profile</Button>
          <Button type="button" onClick={onClose}>Done</Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
