import { useEffect, useMemo, useState } from 'react'
import { api, type ResourceGroup, type Resource, type WGServer } from '../../api/client'
import { Plus, Trash2, Pencil, Search, ChevronRight, Boxes } from 'lucide-react'
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

export default function Bundles() {
  const [bundles, setBundles] = useState<ResourceGroup[]>([])
  const [resources, setResources] = useState<Resource[]>([])
  const [allServers, setAllServers] = useState<WGServer[]>([])
  const [memberships, setMemberships] = useState<Record<string, Resource[]>>({}) // bundleId -> resources
  const [serverAttach, setServerAttach] = useState<Record<string, { id: string; name: string; subnet: string }[]>>({}) // bundleId -> servers it's attached to
  const [query, setQuery] = useState('')
  const [mode, setMode] = useState<Mode>({ kind: 'closed' })
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const load = async () => {
    const [b, r, s] = await Promise.all([api.listResourceGroups(), api.listResources(), api.adminListServers()])
    setBundles(b ?? []); setResources(r ?? []); setAllServers(s ?? [])
    const mems = await Promise.all((b ?? []).map(bg => api.getResourceGroup(bg.id).then(x => [bg.id, x.resources] as const)))
    setMemberships(Object.fromEntries(mems))
    const attach = await Promise.all((b ?? []).map(bg => api.adminBundleServers(bg.id).then(srvs => [bg.id, srvs] as const)))
    setServerAttach(Object.fromEntries(attach))
  }
  useEffect(() => { load() }, [])

  const filtered = useMemo(() => {
    const q = query.toLowerCase()
    if (!q) return bundles
    return bundles.filter(b =>
      b.name.toLowerCase().includes(q) ||
      (b.description?.toLowerCase().includes(q) ?? false),
    )
  }, [bundles, query])

  const selected = bundles.find(b => b.id === selectedId) ?? null

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder={`Search ${bundles.length} ${bundles.length === 1 ? 'bundle' : 'bundles'}…`}
            className="pl-9"
          />
        </div>
        <Button onClick={() => setMode({ kind: 'create' })}><Plus className="w-4 h-4" /> New Bundle</Button>
      </div>

      {filtered.length === 0 ? (
        <Empty
          icon={Boxes}
          title={bundles.length === 0 ? 'No bundles yet' : 'No matches'}
          hint={bundles.length === 0
            ? 'A bundle is a named group of resources. Roles grant access to bundles, not to individual resources — so a bundle is the unit of permission.'
            : 'Try a different search term.'}
          action={bundles.length === 0 ? <Button onClick={() => setMode({ kind: 'create' })}><Plus className="w-4 h-4" /> New Bundle</Button> : undefined}
        />
      ) : (
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          {filtered.map(b => {
            const mems = memberships[b.id] ?? []
            const srvs = serverAttach[b.id] ?? []
            return (
              <button
                key={b.id}
                onClick={() => setSelectedId(b.id)}
                className="w-full grid grid-cols-[1fr_auto_auto_auto] gap-4 items-center text-left px-4 py-3 border-b border-border last:border-0 hover:bg-secondary/30 transition-colors cursor-pointer"
              >
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">{b.name}</p>
                  {b.description && <p className="text-[11px] text-muted-foreground truncate">{b.description}</p>}
                </div>
                <span className="text-[11px] text-muted-foreground whitespace-nowrap">
                  {mems.length} resource{mems.length === 1 ? '' : 's'}
                </span>
                <span className="text-[11px] text-muted-foreground whitespace-nowrap">
                  {srvs.length === 0 ? 'not on any server' : `on ${srvs.length} server${srvs.length === 1 ? '' : 's'}`}
                </span>
                <ChevronRight className="w-4 h-4 text-muted-foreground" />
              </button>
            )
          })}
        </div>
      )}

      <BundleSheet
        mode={mode}
        onClose={() => setMode({ kind: 'closed' })}
        onSaved={() => { load(); setMode({ kind: 'closed' }) }}
        editing={mode.kind === 'edit' ? bundles.find(b => b.id === mode.id) : undefined}
      />

      {selected && (
        <BundleDrawer
          bundle={selected}
          allResources={resources}
          allServers={allServers}
          members={memberships[selected.id] ?? []}
          attachedServers={serverAttach[selected.id] ?? []}
          onClose={() => setSelectedId(null)}
          onChanged={load}
          onEdit={() => { setMode({ kind: 'edit', id: selected.id }); setSelectedId(null) }}
          onDelete={() => { load(); setSelectedId(null) }}
        />
      )}
    </div>
  )
}

function BundleSheet({
  mode, onClose, onSaved, editing,
}: { mode: Mode; onClose: () => void; onSaved: () => void; editing?: ResourceGroup }) {
  const open = mode.kind === 'create' || mode.kind === 'edit'
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
      if (isEdit && editing) await api.updateResourceGroup(editing.id, form)
      else await api.createResourceGroup(form)
      onSaved()
    } catch (e: any) { setError(e.message ?? 'Failed') }
    finally { setBusy(false) }
  }

  return (
    <Sheet open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>{isEdit ? `Edit ${editing?.name}` : 'New Bundle'}</SheetTitle>
          <SheetDescription>
            A named group of resources. Add resources to it from the Resources tab, then grant the bundle to one or more roles.
          </SheetDescription>
        </SheetHeader>
        <form onSubmit={submit} className="contents">
          <SheetBody>
            <div className="space-y-4">
              {error && <p className="text-sm text-destructive">{error}</p>}
              <div className="space-y-1.5">
                <Label className="text-xs">Name</Label>
                <Input value={form.name} onChange={e => setForm(p => ({ ...p, name: e.target.value }))} placeholder="e.g. Internal services" required autoFocus />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Description <span className="text-muted-foreground">(optional)</span></Label>
                <Input value={form.description} onChange={e => setForm(p => ({ ...p, description: e.target.value }))} />
              </div>
            </div>
          </SheetBody>
          <SheetFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? 'Saving…' : isEdit ? 'Save' : 'Create Bundle'}</Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}

function BundleDrawer({
  bundle, allResources, allServers, members, attachedServers, onClose, onChanged, onEdit, onDelete,
}: {
  bundle: ResourceGroup
  allResources: Resource[]
  allServers: WGServer[]
  members: Resource[]
  attachedServers: { id: string; name: string; subnet: string }[]
  onClose: () => void
  onChanged: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const [confirm, setConfirm] = useState(false)
  const candidates = allResources.filter(r => !members.some(m => m.id === r.id))
  const serverCandidates = allServers.filter(s => !attachedServers.some(a => a.id === s.id))

  const addMember = async (rid: string) => { await api.addResourceGroupMember(bundle.id, rid); onChanged() }
  const removeMember = async (rid: string) => { await api.removeResourceGroupMember(bundle.id, rid); onChanged() }
  const attachServer = async (sid: string) => { await api.adminAttachBundleServer(bundle.id, sid); onChanged() }
  const detachServer = async (sid: string) => { await api.adminDetachBundleServer(bundle.id, sid); onChanged() }

  return (
    <Sheet open onOpenChange={(v) => { if (!v) onClose() }}>
      <SheetContent width="w-full sm:max-w-2xl">
        <SheetHeader>
          <div className="flex items-start gap-3">
            <div className="w-10 h-10 rounded-lg bg-success/15 border border-success/25 flex items-center justify-center shrink-0">
              <Boxes className="w-5 h-5 text-success" />
            </div>
            <div className="flex-1 min-w-0">
              <SheetTitle>{bundle.name}</SheetTitle>
              {bundle.description && <SheetDescription>{bundle.description}</SheetDescription>}
            </div>
          </div>
        </SheetHeader>
        <SheetBody>
          <div className="space-y-6">
            <Section title="Resources in this bundle" hint={`${members.length} included`}>
              <div className="space-y-1.5">
                {members.length === 0 ? (
                  <p className="text-sm text-muted-foreground italic">Empty bundle. Add resources below.</p>
                ) : (
                  members.map(r => {
                    const cidr = r.type === 'network' && r.mask != null ? `${r.ip_address}/${r.mask}` : r.ip_address
                    return (
                      <div key={r.id} className="flex items-center gap-2 px-3 py-2 rounded-md border border-border bg-card/60">
                        <div className="flex-1 min-w-0">
                          <p className="text-sm truncate">{r.name}</p>
                          <p className="text-[11px] text-muted-foreground">
                            {r.ports ? `ports: ${r.ports}` : 'all ports'}
                          </p>
                        </div>
                        <MonoChip value={cidr} bare />
                        <Button size="icon" variant="ghost" onClick={() => removeMember(r.id)}
                          aria-label="Remove from bundle"
                          className="h-7 w-7 text-muted-foreground hover:text-destructive hover:bg-destructive/10">
                          <Trash2 className="w-3.5 h-3.5" />
                        </Button>
                      </div>
                    )
                  })
                )}
                <div className="pt-1">
                  <Combobox
                    placeholder="Add resource"
                    options={candidates.map(r => ({
                      value: r.id,
                      label: r.name,
                      hint: r.type === 'network' && r.mask != null ? `${r.ip_address}/${r.mask}` : r.ip_address,
                    }))}
                    onSelect={addMember}
                    emptyText="No more resources to add"
                  />
                </div>
              </div>
            </Section>

            <Section title="Attached to servers" hint={attachedServers.length === 0 ? 'not on any server' : `${attachedServers.length} server${attachedServers.length === 1 ? '' : 's'}`}>
              <div className="flex flex-wrap items-center gap-1.5">
                {attachedServers.map(s => <Chip key={s.id} label={s.name} hint={s.subnet} tone="warning" onRemove={() => detachServer(s.id)} />)}
                <Combobox
                  placeholder="Attach to server"
                  options={serverCandidates.map(s => ({ value: s.id, label: s.name, hint: s.subnet }))}
                  onSelect={attachServer}
                  emptyText="No more servers to attach"
                />
              </div>
              <p className="text-[11px] text-muted-foreground mt-2">
                When attached to a server, this bundle's resources become reachable to anyone who can connect to that server.
              </p>
            </Section>

            <Section title="Edit">
              <Button variant="ghost" onClick={onEdit}><Pencil className="w-4 h-4" /> Edit name/description</Button>
            </Section>

            <DangerZone>
              <DangerAction
                title="Delete this bundle"
                description="Removes the bundle and revokes its resources from every role that granted it. The resources themselves are not deleted."
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
        title={`Delete bundle "${bundle.name}"`}
        description={<>Resources in this bundle stay; they just stop being granted via this name. Roles that referenced it lose access to its members.</>}
        confirmText={bundle.name}
        actionLabel="Delete bundle"
        onConfirm={async () => { await api.deleteResourceGroup(bundle.id); onDelete() }}
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
