import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type WGServer, type AdminSession } from '../api/client'
import { Plus, Search, Globe, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet'
import { PageHeader } from '@/components/PageHeader'
import { StatusPill } from '@/components/StatusPill'
import { MonoChip } from '@/components/MonoChip'
import { Empty } from '@/components/Empty'

type Form = {
  name: string; endpoint: string; port: string; interface_name: string; subnet: string; dns: string
  external: boolean; public_key?: string
}
const empty = (): Form => ({ name: '', endpoint: '', port: '51820', interface_name: '', subnet: '', dns: '', external: false, public_key: '' })

export default function ServersList() {
  const [servers, setServers] = useState<WGServer[]>([])
  const [sessions, setSessions] = useState<AdminSession[]>([])
  const [query, setQuery] = useState('')
  const [creating, setCreating] = useState(false)

  const load = () => {
    api.adminListServers().then(d => setServers(d ?? []))
    api.listAllSessions().then(d => setSessions(d ?? [])).catch(() => {})
  }
  useEffect(() => { load() }, [])

  const sessionsByServer = useMemo(() => {
    const m: Record<string, number> = {}
    for (const s of sessions) if (s.server_id) m[s.server_id] = (m[s.server_id] ?? 0) + 1
    return m
  }, [sessions])

  const filtered = useMemo(() => {
    const q = query.toLowerCase()
    if (!q) return servers
    return servers.filter(s =>
      s.name.toLowerCase().includes(q) ||
      s.endpoint.toLowerCase().includes(q) ||
      s.subnet.toLowerCase().includes(q) ||
      s.interface_name.toLowerCase().includes(q),
    )
  }, [servers, query])

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <PageHeader
        title="Servers"
        description="WireGuard servers this controller manages."
        actions={
          <Button onClick={() => setCreating(true)}><Plus className="w-4 h-4" /> New Server</Button>
        }
      />

      <div className="relative mb-4">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
        <Input value={query} onChange={e => setQuery(e.target.value)} placeholder={`Search ${servers.length} ${servers.length === 1 ? 'server' : 'servers'}…`} className="pl-9 max-w-md" />
      </div>

      {filtered.length === 0 ? (
        <Empty
          icon={Globe}
          title={servers.length === 0 ? 'No servers yet' : 'No matches'}
          hint={servers.length === 0 ? 'Create the first WireGuard server. The controller will bring up the kernel interface for you.' : 'Try a different search term.'}
          action={servers.length === 0 ? <Button onClick={() => setCreating(true)}><Plus className="w-4 h-4" /> New Server</Button> : undefined}
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
          {filtered.map(srv => {
            const live = sessionsByServer[srv.id] ?? 0
            const running = srv.running ?? srv.is_active
            return (
              <Link key={srv.id} to={`/servers/${srv.id}`}
                className="group rounded-xl border border-border bg-card hover:border-primary/40 hover:bg-card/80 transition-colors p-4 cursor-pointer">
                <div className="flex items-start justify-between gap-3 mb-3">
                  <div className="min-w-0">
                    <p className="font-semibold text-sm truncate">{srv.name}</p>
                    <p className="text-[11px] text-muted-foreground font-mono truncate mt-0.5">{srv.endpoint}:{srv.port}</p>
                  </div>
                  <StatusPill
                    kind={running ? 'ok' : srv.is_active ? 'warn' : 'idle'}
                    label={running ? 'running' : srv.is_active ? 'down' : 'inactive'}
                    pulse={running}
                  />
                </div>
                <div className="space-y-1.5 text-xs">
                  <Row label="Interface"><MonoChip value={srv.interface_name} bare copy={false} /></Row>
                  <Row label="Subnet"><MonoChip value={srv.subnet} bare copy={false} /></Row>
                  <Row label="Live peers"><span className="font-medium">{live}</span></Row>
                  {srv.external && <Row label="Type"><span className="text-warning">external</span></Row>}
                </div>
                <div className="mt-3 pt-3 border-t border-border flex items-center text-[11px] text-primary opacity-0 group-hover:opacity-100 transition-opacity">
                  Open server <ChevronRight className="w-3 h-3 ml-auto" />
                </div>
              </Link>
            )
          })}
        </div>
      )}

      <CreateServerSheet open={creating} onClose={() => setCreating(false)} onSaved={() => { load(); setCreating(false) }} />
    </div>
  )
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <span className="text-muted-foreground">{label}</span>
      {children}
    </div>
  )
}

function CreateServerSheet({ open, onClose, onSaved }: { open: boolean; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState<Form>(empty())
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => { if (open) { setForm(empty()); setError('') } }, [open])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true); setError('')
    try {
      await api.adminCreateServer({
        name: form.name, endpoint: form.endpoint, port: parseInt(form.port) || 51820,
        interface_name: form.interface_name, subnet: form.subnet,
        dns: form.dns || undefined, external: form.external,
        public_key: form.external ? form.public_key : undefined,
      })
      onSaved()
    } catch (e: any) { setError(e.message ?? 'Failed') }
    finally { setBusy(false) }
  }

  return (
    <Sheet open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>New WireGuard Server</SheetTitle>
          <SheetDescription>
            The controller will create the kernel interface, generate a keypair, set up firewall rules, and start listening.
          </SheetDescription>
        </SheetHeader>
        <form onSubmit={submit} className="contents">
          <SheetBody>
            <div className="space-y-4">
              {error && <p className="text-sm text-destructive">{error}</p>}
              <div className="grid grid-cols-2 gap-3">
                <Field label="Display name">
                  <Input value={form.name} onChange={e => setForm(p => ({ ...p, name: e.target.value }))} placeholder="HQ Office" required autoFocus />
                </Field>
                <Field label="Public endpoint" hint="Hostname or IP clients will connect to">
                  <Input value={form.endpoint} onChange={e => setForm(p => ({ ...p, endpoint: e.target.value }))} placeholder="vpn.example.com" required />
                </Field>
                <Field label="UDP port">
                  <Input type="number" value={form.port} onChange={e => setForm(p => ({ ...p, port: e.target.value }))} />
                </Field>
                <Field label="Interface name" hint="Linux kernel iface, e.g. wg0">
                  <Input className="font-mono" value={form.interface_name} onChange={e => setForm(p => ({ ...p, interface_name: e.target.value }))} placeholder="wg1" required />
                </Field>
                <Field label="Subnet (CIDR)" hint="The /24 the gateway lives on, e.g. 10.8.0.0/24">
                  <Input className="font-mono" value={form.subnet} onChange={e => setForm(p => ({ ...p, subnet: e.target.value }))} placeholder="10.8.0.0/24" required />
                </Field>
                <Field label="DNS servers" hint="Optional, comma-separated">
                  <Input value={form.dns} onChange={e => setForm(p => ({ ...p, dns: e.target.value }))} placeholder="1.1.1.1,8.8.8.8" />
                </Field>
              </div>

              <div className="rounded-md border border-border bg-card/40 p-3">
                <label className="flex items-start gap-2 cursor-pointer">
                  <input type="checkbox" className="mt-1" checked={form.external} onChange={e => setForm(p => ({ ...p, external: e.target.checked }))} />
                  <div>
                    <p className="text-sm">Externally-managed interface</p>
                    <p className="text-xs text-muted-foreground">Pre-existing WireGuard interface (e.g. set up by hand or by another tool). You'll provide the public key manually.</p>
                  </div>
                </label>
                {form.external && (
                  <div className="mt-3">
                    <Field label="Public key">
                      <Input className="font-mono text-xs" value={form.public_key} onChange={e => setForm(p => ({ ...p, public_key: e.target.value }))} required />
                    </Field>
                  </div>
                )}
              </div>
            </div>
          </SheetBody>
          <SheetFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? 'Creating…' : 'Create Server'}</Button>
          </SheetFooter>
        </form>
      </SheetContent>
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
