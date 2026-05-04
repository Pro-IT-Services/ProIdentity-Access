import { useEffect, useMemo, useState, useCallback } from 'react'
import {
  ReactFlow, Background, Controls, MiniMap, Handle, Position,
  type Node, type Edge, type NodeProps,
} from '@xyflow/react'
import dagre from '@dagrejs/dagre'
import '@xyflow/react/dist/style.css'
import { api, type Topology, type TrafficTopRow } from '../api/client'
import { Users, Boxes, Network, Globe, Activity as ActivityIcon, Search, X } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Combobox } from '@/components/ui/combobox'
import { Chip } from '@/components/Chip'
import { Empty } from '@/components/Empty'
import { cn } from '@/lib/utils'
import { formatBytes } from '@/components/Activity'

// Four-column access flow: Person -> Server -> Bundle -> Resource
type Kind = 'person' | 'server' | 'bundle' | 'resource'

type NData = {
  kind: Kind
  sourceId: string
  label: string
  sub?: string
  selected?: boolean
  dimmed?: boolean
}

const KIND_META: Record<Kind, { icon: React.ElementType; tone: string; label: string }> = {
  person:   { icon: Users,   tone: 'border-primary/40 bg-primary/5',         label: 'People'    },
  server:   { icon: Globe,   tone: 'border-destructive/40 bg-destructive/5', label: 'Servers'   },
  bundle:   { icon: Boxes,   tone: 'border-success/40 bg-success/5',         label: 'Bundles'   },
  resource: { icon: Network, tone: 'border-warning/40 bg-warning/5',         label: 'Resources' },
}

function NodeCard({ data }: NodeProps<Node<NData>>) {
  const { icon: Icon, tone } = KIND_META[data.kind]
  return (
    <div
      className={cn(
        'rounded-lg border bg-card text-foreground shadow-sm px-3 py-2 min-w-[150px] max-w-[200px]',
        'transition-opacity transition-colors duration-150',
        tone,
        data.selected && 'ring-2 ring-primary ring-offset-2 ring-offset-background',
        data.dimmed && 'opacity-25',
      )}
    >
      <Handle type="target" position={Position.Left} className="!bg-border !w-2 !h-2 !border-0" />
      <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-wider text-muted-foreground font-semibold mb-0.5">
        <Icon className="w-3 h-3" /> {data.kind}
      </div>
      <p className="text-sm font-medium truncate">{data.label}</p>
      {data.sub && <p className="text-[11px] text-muted-foreground font-mono truncate mt-0.5">{data.sub}</p>}
      <Handle type="source" position={Position.Right} className="!bg-border !w-2 !h-2 !border-0" />
    </div>
  )
}

const NODE_TYPES = { card: NodeCard }
const NODE_W = 200
const NODE_H = 76

function layout(nodes: Node[], edges: Edge[]) {
  if (nodes.length === 0) return nodes
  const g = new dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir: 'LR', nodesep: 14, ranksep: 100, marginx: 20, marginy: 20 })
  nodes.forEach(n => g.setNode(n.id, { width: NODE_W, height: NODE_H }))
  edges.forEach(e => g.setEdge(e.source, e.target))
  dagre.layout(g)
  return nodes.map(n => {
    const p = g.node(n.id)
    return { ...n, position: { x: p.x - NODE_W / 2, y: p.y - NODE_H / 2 } }
  })
}

const nodeId = (kind: Kind, raw: string) => `${kind}:${raw}`
const scopedId = (kind: Kind, userId: string, serverId: string, raw: string, bundleId?: string) =>
  bundleId
    ? `${kind}:${userId}:${serverId}:${bundleId}:${raw}`
    : `${kind}:${userId}:${serverId}:${raw}`

function splitUserServer(value: string): { userId: string; serverId: string } | null {
  const idx = value.indexOf(':')
  if (idx <= 0 || idx === value.length - 1) return null
  return { userId: value.slice(0, idx), serverId: value.slice(idx + 1) }
}

function buildGraph(t: Topology, trafficByPath?: Map<string, number>) {
  // Defensive: backend should always send arrays, but never trust the wire.
  const people    = t.people    ?? []
  const bundles   = t.bundles   ?? []
  const resources = t.resources ?? []
  const servers   = t.servers   ?? []
  const e_ps      = t.person_server   ?? []
  const e_ub      = t.user_bundle     ?? []
  const e_br      = t.bundle_resource ?? []

  // First, figure out which nodes participate in any edge → those are the only
  // ones we render. Orphans are hidden.
  const personById = new Map(people.map(p => [p.id, p]))
  const serverById = new Map(servers.map(s => [s.id, s]))
  const bundleById = new Map(bundles.map(b => [b.id, b]))
  const resourceById = new Map(resources.map(r => [r.id, r]))
  const resourcesByBundle = new Map<string, typeof resources>()
  e_br.forEach(e => {
    const resource = resourceById.get(e.to)
    if (!resource) return
    const list = resourcesByBundle.get(e.from) ?? []
    list.push(resource)
    resourcesByBundle.set(e.from, list)
  })

  const nodes: Node<NData>[] = []
  const seenNodes = new Set<string>()
  const push = (id: string, kind: Kind, sourceId: string, label: string, sub?: string) => {
    if (seenNodes.has(id)) return
    seenNodes.add(id)
    nodes.push({ id, type: 'card', position: { x: 0, y: 0 }, data: { kind, sourceId, label, sub } })
  }

  const maxBytes = trafficByPath
    ? Array.from(trafficByPath.values()).reduce((m, v) => Math.max(m, v), 0)
    : 0

  const styleEdge = (trafficKey?: string): Partial<Edge> => {
    if (!trafficKey || maxBytes === 0) {
      return { style: { strokeWidth: 1, stroke: 'hsl(var(--border))' } }
    }
    const bytes = trafficByPath!.get(trafficKey) ?? 0
    if (bytes === 0) return { style: { strokeWidth: 1, stroke: 'hsl(var(--border))' } }
    const w = 1 + Math.round((bytes / maxBytes) * 6)
    return {
      style: { strokeWidth: w, stroke: 'hsl(var(--primary))', opacity: 0.85 },
      label: formatBytes(bytes),
      labelStyle: { fontSize: 10, fill: 'hsl(var(--foreground))' },
      labelBgStyle: { fill: 'hsl(var(--card))', fillOpacity: 0.9 },
      labelBgPadding: [4, 2] as [number, number],
    }
  }

  const edges: Edge[] = []
  const seenEdges = new Set<string>()
  const pushEdge = (id: string, source: string, target: string, extra?: Partial<Edge>) => {
    if (seenEdges.has(id)) return
    seenEdges.add(id)
    edges.push({ id, source, target, ...extra })
  }

  e_ps.forEach(e => {
    const person = personById.get(e.from)
    const server = serverById.get(e.to)
    if (!person || !server) return
    const personId = nodeId('person', person.id)
    const serverId = scopedId('server', person.id, server.id, server.id)
    push(personId, 'person', person.id, person.username, person.is_admin ? 'admin' : '')
    push(serverId, 'server', server.id, server.name, server.subnet)
    pushEdge(`ps-${person.id}-${server.id}`, personId, serverId, styleEdge())
  })

  e_ub.forEach(e => {
    const pair = splitUserServer(e.from)
    if (!pair) return
    const person = personById.get(pair.userId)
    const server = serverById.get(pair.serverId)
    const bundle = bundleById.get(e.to)
    if (!person || !server || !bundle) return

    const personId = nodeId('person', person.id)
    const serverId = scopedId('server', person.id, server.id, server.id)
    const bundleId = scopedId('bundle', person.id, server.id, bundle.id)
    push(personId, 'person', person.id, person.username, person.is_admin ? 'admin' : '')
    push(serverId, 'server', server.id, server.name, server.subnet)
    push(bundleId, 'bundle', bundle.id, bundle.name)
    pushEdge(`ps-${person.id}-${server.id}`, personId, serverId, styleEdge())
    pushEdge(`sb-${person.id}-${server.id}-${bundle.id}`, serverId, bundleId, styleEdge())

    for (const resource of resourcesByBundle.get(bundle.id) ?? []) {
      const resourceId = scopedId('resource', person.id, server.id, resource.id, bundle.id)
      const cidr = resource.type === 'network' && resource.mask != null ? `${resource.ip_address}/${resource.mask}` : resource.ip_address
      push(resourceId, 'resource', resource.id, resource.name, cidr)
      pushEdge(
        `br-${person.id}-${server.id}-${bundle.id}-${resource.id}`,
        bundleId,
        resourceId,
        styleEdge(`${person.id}:${server.id}:${resource.id}`),
      )
    }
  })

  return { nodes, edges }
}

function reachableFromSeeds(seeds: Iterable<string>, edges: Edge[]): Set<string> {
  const adj = new Map<string, Set<string>>()
  edges.forEach(e => {
    if (!adj.has(e.source)) adj.set(e.source, new Set())
    if (!adj.has(e.target)) adj.set(e.target, new Set())
    adj.get(e.source)!.add(e.target)
    adj.get(e.target)!.add(e.source)
  })
  const seen = new Set<string>()
  const queue: string[] = []
  for (const s of seeds) { if (!seen.has(s)) { seen.add(s); queue.push(s) } }
  while (queue.length) {
    const cur = queue.shift()!
    for (const next of adj.get(cur) ?? []) {
      if (!seen.has(next)) { seen.add(next); queue.push(next) }
    }
  }
  return seen
}

type Filters = Record<Kind, string[]>
const emptyFilters = (): Filters => ({ person: [], server: [], bundle: [], resource: [] })

export default function TopologyPage() {
  const [data, setData] = useState<Topology | null>(null)
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState('')
  const [overlay, setOverlay] = useState(false)
  const [traffic, setTraffic] = useState<TrafficTopRow[]>([])
  const [focus, setFocus] = useState<string | null>(null)
  const [filters, setFilters] = useState<Filters>(emptyFilters())
  const [search, setSearch] = useState('')

  useEffect(() => {
    setLoading(true)
    api.adminTopology()
      .then(setData)
      .catch(e => setErr(e.message ?? 'Failed to load topology'))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    if (!overlay) return
    api.trafficTop({ by: 'user_resource', limit: 1000 }).then(setTraffic).catch(() => {})
  }, [overlay])

  const trafficByPath = useMemo(() => {
    if (!overlay) return undefined
    const m = new Map<string, number>()
    traffic.forEach(r => { if (r.key) m.set(r.key, r.bytes_tx + r.bytes_rx) })
    return m
  }, [overlay, traffic])

  const base = useMemo(() => data ? buildGraph(data, trafficByPath) : { nodes: [], edges: [] }, [data, trafficByPath])

  const filterSeeds = useMemo(() => {
    const selected = new Map<Kind, Set<string>>()
    ;(Object.keys(filters) as Kind[]).forEach(k => {
      if (filters[k].length) selected.set(k, new Set(filters[k]))
    })
    if (selected.size === 0) return []
    return base.nodes
      .filter(n => selected.get((n.data as NData).kind)?.has((n.data as NData).sourceId))
      .map(n => n.id)
  }, [filters, base.nodes])
  const hasFilters = filterSeeds.length > 0

  const visible = useMemo(() => {
    if (!hasFilters) return null
    return reachableFromSeeds(filterSeeds, base.edges)
  }, [filterSeeds, base.edges, hasFilters])

  const filtered = useMemo(() => {
    if (!visible) return base
    const ns = base.nodes.filter(n => visible.has(n.id))
    const es = base.edges.filter(e => visible.has(e.source) && visible.has(e.target))
    return { nodes: layout(ns, es), edges: es }
  }, [base, visible])

  const laidOut = useMemo(() => {
    if (visible) return filtered
    return { nodes: layout(base.nodes, base.edges), edges: base.edges }
  }, [base, filtered, visible])

  const focusReach = useMemo(() => focus ? reachableFromSeeds([focus], laidOut.edges) : null, [focus, laidOut.edges])

  const matchedIds = useMemo(() => {
    if (!search.trim()) return new Set<string>()
    const q = search.toLowerCase()
    return new Set(
      laidOut.nodes
        .filter(n => (n.data as NData).label.toLowerCase().includes(q) ||
                     ((n.data as NData).sub?.toLowerCase().includes(q) ?? false))
        .map(n => n.id),
    )
  }, [laidOut.nodes, search])

  const renderedNodes = useMemo(() => {
    return laidOut.nodes.map(n => ({
      ...n,
      data: {
        ...n.data,
        selected: n.id === focus || matchedIds.has(n.id),
        dimmed: focusReach ? !focusReach.has(n.id) : false,
      },
    }))
  }, [laidOut.nodes, focus, focusReach, matchedIds])

  const renderedEdges = useMemo(() => {
    if (!focusReach) return laidOut.edges
    return laidOut.edges.map(e => {
      const lit = focusReach.has(e.source) && focusReach.has(e.target)
      return { ...e, style: { ...(e.style ?? {}), opacity: lit ? 1 : 0.1 } }
    })
  }, [laidOut.edges, focusReach])

  const onNodeClick = useCallback((_: any, n: Node) => setFocus(prev => prev === n.id ? null : n.id), [])
  const onPaneClick = useCallback(() => setFocus(null), [])

  const addFilter = (kind: Kind, id: string) =>
    setFilters(f => f[kind].includes(id) ? f : { ...f, [kind]: [...f[kind], id] })
  const removeFilter = (kind: Kind, id: string) =>
    setFilters(f => ({ ...f, [kind]: f[kind].filter(x => x !== id) }))
  const clearFilters = () => { setFilters(emptyFilters()); setFocus(null) }

  const totalNodes = base.nodes.length
  const visibleNodes = laidOut.nodes.length

  return (
    <div className="flex flex-col h-full">
      <div className="px-6 pt-6">
        <PageHeader
          title="Topology"
          description="Per-user access flow: Person -> Server -> Bundle -> Resource. Shared servers and bundles are shown inside each user's chain."
          actions={
            <div className="flex items-center gap-2">
              <Button
                variant={overlay ? 'default' : 'ghost'}
                size="sm"
                onClick={() => setOverlay(o => !o)}
                title="Overlay edge thickness with last 24h traffic"
              >
                <ActivityIcon className="w-4 h-4" /> {overlay ? 'Hide traffic' : 'Show traffic'}
              </Button>
            </div>
          }
        />

        {data && (
          <div className="rounded-xl border border-border bg-card/40 p-3 mb-3 space-y-2.5">
            <div className="flex flex-wrap items-center gap-2">
              <div className="relative flex-1 min-w-[220px]">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input value={search} onChange={e => setSearch(e.target.value)} placeholder="Highlight nodes by name or address…" className="pl-9 h-9" />
              </div>

              <FilterPicker
                kind="person"
                options={data.people.map(p => ({ value: p.id, label: p.username, hint: p.is_admin ? 'admin' : undefined }))}
                selected={filters.person}
                onAdd={(id) => addFilter('person', id)}
              />
              <FilterPicker
                kind="server"
                options={data.servers.map(s => ({ value: s.id, label: s.name, hint: s.subnet }))}
                selected={filters.server}
                onAdd={(id) => addFilter('server', id)}
              />
              <FilterPicker
                kind="bundle"
                options={data.bundles.map(b => ({ value: b.id, label: b.name }))}
                selected={filters.bundle}
                onAdd={(id) => addFilter('bundle', id)}
              />
              <FilterPicker
                kind="resource"
                options={data.resources.map(r => ({
                  value: r.id,
                  label: r.name,
                  hint: r.type === 'network' && r.mask != null ? `${r.ip_address}/${r.mask}` : r.ip_address,
                }))}
                selected={filters.resource}
                onAdd={(id) => addFilter('resource', id)}
              />
            </div>

            {hasFilters && (
              <div className="flex flex-wrap items-center gap-1.5 pt-1">
                {(Object.keys(filters) as Kind[]).flatMap(k =>
                  filters[k].map(id => (
                    <Chip key={`${k}:${id}`} label={labelFor(data, k, id)} hint={k} tone="primary" onRemove={() => removeFilter(k, id)} />
                  )),
                )}
                <button
                  onClick={clearFilters}
                  className="inline-flex items-center gap-1 px-2 py-0.5 text-[11px] text-muted-foreground hover:text-destructive transition-colors cursor-pointer"
                >
                  <X className="w-3 h-3" /> clear all
                </button>
              </div>
            )}
          </div>
        )}

        <div className="flex flex-wrap gap-3 text-[11px] mb-3 items-center">
          {(['person','server','bundle','resource'] as Kind[]).map(k => {
            const { icon: Icon, tone } = KIND_META[k]
            return (
              <span key={k} className={cn('inline-flex items-center gap-1.5 px-2 py-0.5 rounded border', tone)}>
                <Icon className="w-3 h-3" /> {k}
              </span>
            )
          })}
          <span className="text-muted-foreground ml-auto">
            {hasFilters
              ? `Showing ${visibleNodes} of ${totalNodes} connected nodes`
              : `${totalNodes} connected node${totalNodes === 1 ? '' : 's'}`}
            {focus && <> · click empty space to clear focus</>}
          </span>
        </div>
      </div>

      <div className="flex-1 mx-6 mb-6 rounded-xl border border-border bg-card overflow-hidden">
        {loading ? (
          <div className="h-full flex items-center justify-center text-muted-foreground text-sm">Loading topology…</div>
        ) : err ? (
          <div className="h-full flex items-center justify-center text-destructive text-sm">{err}</div>
        ) : laidOut.nodes.length === 0 ? (
          <div className="p-6">
            <Empty
              title={hasFilters ? 'Nothing matches' : 'No connected access yet'}
              hint={hasFilters
                ? 'The selected items aren\'t connected to anything else. Try removing a filter.'
                : 'Nothing is wired up end-to-end yet. Create a server, a bundle with at least one resource, assign the server to a person, then assign that bundle to the person on that server.'}
              action={hasFilters ? <Button variant="ghost" size="sm" onClick={clearFilters}>Clear filters</Button> : undefined}
            />
          </div>
        ) : (
          <ReactFlow
            key={hasFilters ? 'filtered' : 'all'}
            nodes={renderedNodes}
            edges={renderedEdges}
            nodeTypes={NODE_TYPES}
            onNodeClick={onNodeClick}
            onPaneClick={onPaneClick}
            fitView
            minZoom={0.2}
            maxZoom={2}
            proOptions={{ hideAttribution: true }}
          >
            <Background gap={20} size={1} color="hsl(var(--border))" />
            <Controls showInteractive={false} className="!bg-card !border !border-border [&_button]:!bg-card [&_button]:!border-border [&_button:hover]:!bg-secondary" />
            <MiniMap pannable zoomable className="!bg-card !border !border-border" nodeStrokeWidth={2}
              nodeColor={n => {
                const k = (n.data as NData)?.kind
                switch (k) {
                  case 'person':   return 'hsl(var(--primary))'
                  case 'server':   return 'hsl(var(--destructive))'
                  case 'bundle':   return 'hsl(var(--success))'
                  case 'resource': return 'hsl(var(--warning))'
                  default:         return 'hsl(var(--muted-foreground))'
                }
              }}
            />
          </ReactFlow>
        )}
      </div>
    </div>
  )
}

function FilterPicker({
  kind, options, selected, onAdd,
}: {
  kind: Kind
  options: { value: string; label: string; hint?: string }[]
  selected: string[]
  onAdd: (id: string) => void
}) {
  const { icon: Icon, label } = KIND_META[kind]
  const remaining = options.filter(o => !selected.includes(o.value))
  return (
    <Combobox
      placeholder={
        <span className="inline-flex items-center gap-1.5">
          <Icon className="w-3.5 h-3.5" /> {label}
          {selected.length > 0 && <span className="text-primary">· {selected.length}</span>}
        </span>
      }
      options={remaining}
      onSelect={onAdd}
      buttonClassName="h-9"
    />
  )
}

function labelFor(t: Topology, kind: Kind, id: string): string {
  switch (kind) {
    case 'person':   return t.people.find(p => p.id === id)?.username ?? id
    case 'server':   return t.servers.find(s => s.id === id)?.name ?? id
    case 'bundle':   return t.bundles.find(b => b.id === id)?.name ?? id
    case 'resource': return t.resources.find(r => r.id === id)?.name ?? id
  }
}
