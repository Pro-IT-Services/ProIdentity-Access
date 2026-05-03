import { useEffect, useMemo, useState } from 'react'
import { api, type Resource, type ResourceGroup } from '../../api/client'
import { Plus, Trash2, Pencil, Search, ChevronRight, Network } from 'lucide-react'
import { Activity } from '@/components/Activity'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet'
import { Combobox } from '@/components/ui/combobox'
import { MonoChip } from '@/components/MonoChip'
import { Chip } from '@/components/Chip'
import { Empty } from '@/components/Empty'
import { ConfirmDelete } from '@/components/ConfirmDelete'
import { DangerZone, DangerAction } from '@/components/DangerZone'

type Mode = { kind: 'closed' } | { kind: 'create' } | { kind: 'edit'; id: string }

interface ResourceForm {
  name: string; ip_address: string; type: 'host' | 'network'; mask: string; ports: string; description: string
}
const empty = (): ResourceForm => ({ name: '', ip_address: '', type: 'host', mask: '24', ports: '', description: '' })

export default function Resources() {
  const [resources, setResources] = useState<Resource[]>([])
  const [bundles, setBundles] = useState<ResourceGroup[]>([])
  const [bundleMembership, setBundleMembership] = useState<Record<string, string[]>>({}) // bundleId -> resourceIds
  const [query, setQuery] = useState('')
  const [mode, setMode] = useState<Mode>({ kind: 'closed' })
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const load = async () => {
    const [r, b] = await Promise.all([api.listResources(), api.listResourceGroups()])
    setResources(r ?? [])
    setBundles(b ?? [])
    // load membership maps in parallel
    const memberships = await Promise.all((b ?? []).map(bg => api.getResourceGroup(bg.id).then(x => [bg.id, x.resources.map(rr => rr.id)] as const)))
    setBundleMembership(Object.fromEntries(memberships))
  }
  useEffect(() => { load() }, [])

  const filtered = useMemo(() => {
    const q = query.toLowerCase()
    if (!q) return resources
    return resources.filter(r =>
      r.name.toLowerCase().includes(q) ||
      r.ip_address.toLowerCase().includes(q) ||
      (r.description?.toLowerCase().includes(q) ?? false),
    )
  }, [resources, query])

  const selected = resources.find(r => r.id === selectedId) ?? null

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder={`Search ${resources.length} ${resources.length === 1 ? 'resource' : 'resources'}…`}
            className="pl-9"
          />
        </div>
        <Button onClick={() => setMode({ kind: 'create' })}><Plus className="w-4 h-4" /> New Resource</Button>
      </div>

      {filtered.length === 0 ? (
        <Empty
          icon={Network}
          title={resources.length === 0 ? 'No resources yet' : 'No matches'}
          hint={resources.length === 0 ? 'Resources are LAN destinations (a single host or a subnet) that you can grant to roles.' : 'Try a different search term.'}
          action={resources.length === 0 ? <Button onClick={() => setMode({ kind: 'create' })}><Plus className="w-4 h-4" /> New Resource</Button> : undefined}
        />
      ) : (
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          {filtered.map(r => {
            const cidr = r.type === 'network' && r.mask != null ? `${r.ip_address}/${r.mask}` : r.ip_address
            const inBundles = bundles.filter(b => bundleMembership[b.id]?.includes(r.id))
            return (
              <button
                key={r.id}
                onClick={() => setSelectedId(r.id)}
                className="w-full grid grid-cols-[1fr_auto_auto_auto] gap-4 items-center text-left px-4 py-3 border-b border-border last:border-0 hover:bg-secondary/30 transition-colors cursor-pointer"
              >
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">{r.name}</p>
                  {r.description && <p className="text-[11px] text-muted-foreground truncate">{r.description}</p>}
                </div>
                <MonoChip value={cidr} bare />
                <span className="text-[11px] text-muted-foreground whitespace-nowrap">
                  {r.ports ? `:${r.ports}` : 'all ports'}
                </span>
                <div className="flex items-center gap-1">
                  <span className="text-[11px] text-muted-foreground">in {inBundles.length} bundle{inBundles.length === 1 ? '' : 's'}</span>
                  <ChevronRight className="w-4 h-4 text-muted-foreground" />
                </div>
              </button>
            )
          })}
        </div>
      )}

      <ResourceSheet
        mode={mode}
        onClose={() => setMode({ kind: 'closed' })}
        onSaved={() => { load(); setMode({ kind: 'closed' }) }}
        editing={mode.kind === 'edit' ? resources.find(r => r.id === mode.id) : undefined}
      />

      {selected && (
        <ResourceDrawer
          resource={selected}
          bundles={bundles}
          bundleMembership={bundleMembership}
          onClose={() => setSelectedId(null)}
          onChanged={load}
          onEdit={() => { setMode({ kind: 'edit', id: selected.id }); setSelectedId(null) }}
          onDelete={() => { load(); setSelectedId(null) }}
        />
      )}
    </div>
  )
}

function ResourceSheet({
  mode, onClose, onSaved, editing,
}: { mode: Mode; onClose: () => void; onSaved: () => void; editing?: Resource }) {
  const open = mode.kind !== 'closed'
  const isEdit = mode.kind === 'edit'
  const [form, setForm] = useState<ResourceForm>(empty())
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (mode.kind === 'create') setForm(empty())
    else if (mode.kind === 'edit' && editing) setForm({
      name: editing.name, ip_address: editing.ip_address, type: editing.type ?? 'host',
      mask: editing.mask != null ? String(editing.mask) : '24',
      ports: editing.ports ?? '', description: editing.description ?? '',
    })
    setError('')
  }, [mode.kind, editing?.id])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true); setError('')
    try {
      const payload = {
        name: form.name,
        ip_address: form.ip_address,
        type: form.type,
        mask: form.type === 'network' ? (form.mask !== '' ? parseInt(form.mask) : null) : null,
        ports: form.ports.trim() || null,
        description: form.description || undefined,
      }
      if (isEdit && editing) await api.updateResource(editing.id, payload)
      else await api.createResource(payload)
      onSaved()
    } catch (e: any) { setError(e.message ?? 'Failed') }
    finally { setBusy(false) }
  }

  return (
    <Sheet open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>{isEdit ? `Edit ${editing?.name}` : 'New Resource'}</SheetTitle>
          <SheetDescription>
            A reachable LAN destination — a single host (e.g. <span className="font-mono">192.168.1.10</span>) or a subnet.
          </SheetDescription>
        </SheetHeader>
        <form onSubmit={submit} className="contents">
          <SheetBody>
            <div className="space-y-4">
              {error && <p className="text-sm text-destructive">{error}</p>}
              <div className="space-y-1.5">
                <Label className="text-xs">Name</Label>
                <Input value={form.name} onChange={e => setForm(p => ({ ...p, name: e.target.value }))} placeholder="e.g. Office NAS" required autoFocus />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Type</Label>
                <div className="grid grid-cols-2 gap-2">
                  {(['host', 'network'] as const).map(t => (
                    <button key={t} type="button" onClick={() => setForm(p => ({ ...p, type: t }))}
                      className={`px-3 py-2 rounded-md border text-sm cursor-pointer transition-colors ${form.type === t ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-card hover:bg-secondary'}`}>
                      {t === 'host' ? 'Single host' : 'Subnet (CIDR)'}
                    </button>
                  ))}
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5 col-span-2">
                  <Label className="text-xs">IP address</Label>
                  <Input className="font-mono" value={form.ip_address} onChange={e => setForm(p => ({ ...p, ip_address: e.target.value }))} placeholder="192.168.1.10" required />
                </div>
                {form.type === 'network' && (
                  <div className="space-y-1.5">
                    <Label className="text-xs">Mask (prefix length)</Label>
                    <Input type="number" min={8} max={32} value={form.mask} onChange={e => setForm(p => ({ ...p, mask: e.target.value }))} required />
                  </div>
                )}
                <div className="space-y-1.5 col-span-2">
                  <Label className="text-xs">Ports <span className="text-muted-foreground">(optional)</span></Label>
                  <Input className="font-mono" value={form.ports} onChange={e => setForm(p => ({ ...p, ports: e.target.value }))} placeholder="80,443,8080-8090" />
                  <p className="text-[11px] text-muted-foreground">Empty = all protocols + ports.</p>
                </div>
                <div className="space-y-1.5 col-span-2">
                  <Label className="text-xs">Description <span className="text-muted-foreground">(optional)</span></Label>
                  <Input value={form.description} onChange={e => setForm(p => ({ ...p, description: e.target.value }))} />
                </div>
              </div>
            </div>
          </SheetBody>
          <SheetFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? 'Saving…' : isEdit ? 'Save' : 'Create Resource'}</Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}

function ResourceDrawer({
  resource, bundles, bundleMembership, onClose, onChanged, onEdit, onDelete,
}: {
  resource: Resource
  bundles: ResourceGroup[]
  bundleMembership: Record<string, string[]>
  onClose: () => void
  onChanged: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const [confirm, setConfirm] = useState(false)
  const cidr = resource.type === 'network' && resource.mask != null ? `${resource.ip_address}/${resource.mask}` : resource.ip_address
  const inBundles = bundles.filter(b => bundleMembership[b.id]?.includes(resource.id))
  const otherBundles = bundles.filter(b => !bundleMembership[b.id]?.includes(resource.id))

  const addToBundle = async (bid: string) => { await api.addResourceGroupMember(bid, resource.id); onChanged() }
  const removeFromBundle = async (bid: string) => { await api.removeResourceGroupMember(bid, resource.id); onChanged() }

  return (
    <Sheet open onOpenChange={(v) => { if (!v) onClose() }}>
      <SheetContent width="w-full sm:max-w-2xl">
        <SheetHeader>
          <SheetTitle>{resource.name}</SheetTitle>
          <SheetDescription className="flex items-center gap-3">
            <MonoChip value={cidr} />
            <span className="text-xs">{resource.ports ? `ports: ${resource.ports}` : 'all ports'}</span>
          </SheetDescription>
        </SheetHeader>
        <SheetBody>
          <div className="space-y-6">
            {resource.description && (
              <p className="text-sm text-foreground/85">{resource.description}</p>
            )}

            <Section title="Activity" hint="Last 24 hours">
              <Activity resourceId={resource.id} topBy="user" topTitle="Top users hitting this resource" emptyLabel="Nobody has hit this resource in the last 24h." />
            </Section>

            <Section title="Bundles" hint={`In ${inBundles.length} bundle${inBundles.length === 1 ? '' : 's'}`}>
              <div className="flex flex-wrap items-center gap-1.5">
                {inBundles.map(b => <Chip key={b.id} label={b.name} tone="success" onRemove={() => removeFromBundle(b.id)} />)}
                {otherBundles.length > 0 && (
                  <Combobox
                    placeholder="Add to bundle"
                    options={otherBundles.map(b => ({ value: b.id, label: b.name }))}
                    onSelect={addToBundle}
                  />
                )}
              </div>
              <p className="text-[11px] text-muted-foreground mt-2">
                Resources are reachable to a role only after being included in a bundle that the role has access to.
              </p>
            </Section>

            <Section title="Edit">
              <Button variant="ghost" onClick={onEdit}><Pencil className="w-4 h-4" /> Edit details</Button>
            </Section>

            <DangerZone>
              <DangerAction
                title="Delete this resource"
                description="Removes the resource from all bundles and revokes access for everyone."
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
        title={`Delete resource "${resource.name}"`}
        description={<>This removes the resource and unlinks it from every bundle. Anyone whose access depended on it will lose reachability.</>}
        confirmText={resource.name}
        actionLabel="Delete resource"
        onConfirm={async () => { await api.deleteResource(resource.id); onDelete() }}
      />
    </Sheet>
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
