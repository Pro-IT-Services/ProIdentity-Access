import { useEffect, useMemo, useState } from 'react'
import { api, type User, type VPNEvent, type WGServer } from '../api/client'
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight, History, LogIn, LogOut, RefreshCw, Search, X } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { Empty } from '@/components/Empty'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

type EventFilter = 'all' | 'connected' | 'disconnected'
type SinceFilter = '24h' | '7d' | '30d' | 'all'

type Filters = {
  q: string
  user_id: string
  server_id: string
  event: EventFilter
  source_ip: string
  device: string
  since: SinceFilter
}

const defaultFilters: Filters = {
  q: '',
  user_id: '',
  server_id: '',
  event: 'all',
  source_ip: '',
  device: '',
  since: '7d',
}

const PAGE_SIZES = [25, 50, 100] as const

export default function ConnectionHistory() {
  const [events, setEvents] = useState<VPNEvent[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [servers, setServers] = useState<WGServer[]>([])
  const [filters, setFilters] = useState<Filters>(defaultFilters)
  const [search, setSearch] = useState('')
  const [pageSize, setPageSize] = useState<number>(50)
  const [offset, setOffset] = useState(0)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const params = useMemo(() => {
    const since = sinceDate(filters.since)
    return {
      limit: pageSize,
      offset,
      q: filters.q.trim(),
      user_id: filters.user_id,
      server_id: filters.server_id,
      event: filters.event === 'all' ? undefined : filters.event,
      source_ip: filters.source_ip.trim(),
      device: filters.device.trim(),
      since: since ? since.toISOString() : undefined,
    }
  }, [filters, offset, pageSize])

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const data = await api.listVPNEvents(params)
      setEvents(data.items ?? [])
      setTotal(data.total ?? 0)
    } catch (e: any) {
      setError(e?.message ?? 'Failed to load connection history')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    Promise.all([
      api.listUsers().then(setUsers).catch(() => {}),
      api.adminListServers().then(setServers).catch(() => {}),
    ])
  }, [])

  useEffect(() => { load() }, [params])

  // Search runs on the server over all events (not just this page); apply it
  // shortly after typing stops.
  useEffect(() => {
    const t = setTimeout(() => {
      if (search !== filters.q) update('q', search)
    }, 300)
    return () => clearTimeout(t)
  }, [search])

  const update = <K extends keyof Filters>(key: K, value: Filters[K]) => {
    setOffset(0)
    setFilters(prev => ({ ...prev, [key]: value }))
  }

  const clear = () => {
    setOffset(0)
    setSearch('')
    setFilters(defaultFilters)
  }

  // Pages are counted from the filtered total, so filters and search always
  // apply to every event, and the current page resets when they change.
  const pageCount = Math.max(1, Math.ceil(total / pageSize))
  const page = Math.floor(offset / pageSize) + 1
  const goTo = (p: number) => setOffset((Math.min(Math.max(p, 1), pageCount) - 1) * pageSize)
  const from = total === 0 ? 0 : offset + 1
  const to = Math.min(offset + pageSize, total)

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <PageHeader
        title="Connection History"
        description="VPN connect and disconnect audit trail for all users."
        actions={
          <Button variant="ghost" onClick={load} disabled={loading}>
            <RefreshCw className={loading ? 'w-4 h-4 animate-spin' : 'w-4 h-4'} />
            Refresh
          </Button>
        }
      />

      <Card className="mb-4">
        <CardContent className="p-4">
          <div className="relative mb-3">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <Input
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="pl-9"
              placeholder="Search user, email, server, device, source IP, VPN IP or reason…"
            />
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-6 gap-3">
            <Field label="User">
              <select
                value={filters.user_id}
                onChange={e => update('user_id', e.target.value)}
                className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
              >
                <option value="">All users</option>
                {users.map(u => <option key={u.id} value={u.id}>{userLabel(u)}</option>)}
              </select>
            </Field>
            <Field label="Server">
              <select
                value={filters.server_id}
                onChange={e => update('server_id', e.target.value)}
                className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
              >
                <option value="">All servers</option>
                {servers.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
              </select>
            </Field>
            <Field label="Event">
              <select
                value={filters.event}
                onChange={e => update('event', e.target.value as EventFilter)}
                className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
              >
                <option value="all">All events</option>
                <option value="connected">Connected</option>
                <option value="disconnected">Disconnected</option>
              </select>
            </Field>
            <Field label="Since">
              <select
                value={filters.since}
                onChange={e => update('since', e.target.value as SinceFilter)}
                className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
              >
                <option value="24h">Last 24 hours</option>
                <option value="7d">Last 7 days</option>
                <option value="30d">Last 30 days</option>
                <option value="all">All time</option>
              </select>
            </Field>
            <Field label="Source IP">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input value={filters.source_ip} onChange={e => update('source_ip', e.target.value)} className="pl-9" placeholder="Exact IP" />
              </div>
            </Field>
            <Field label="Device">
              <Input value={filters.device} onChange={e => update('device', e.target.value)} placeholder="Name or ID" />
            </Field>
          </div>
          <div className="mt-3 flex items-center justify-between gap-3">
            <p className="text-xs text-muted-foreground">{total} matching {total === 1 ? 'event' : 'events'}</p>
            <Button variant="outline" size="sm" onClick={clear}><X className="w-4 h-4" /> Clear filters</Button>
          </div>
        </CardContent>
      </Card>

      {error && (
        <div className="rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive mb-4">
          {error}
        </div>
      )}

      {events.length === 0 && !loading ? (
        <Empty
          icon={History}
          title="No connection events"
          hint="Try a wider time range or clear filters."
        />
      ) : (
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          <div className="hidden lg:grid grid-cols-[130px_1.2fr_1fr_120px_140px_1.2fr_150px] gap-3 px-4 py-2 border-b border-border text-[11px] font-semibold uppercase tracking-widest text-muted-foreground">
            <span>Event</span>
            <span>User</span>
            <span>Server</span>
            <span>VPN IP</span>
            <span>Source IP</span>
            <span>Device</span>
            <span>Time</span>
          </div>
          <div className="divide-y divide-border">
            {events.map(e => <EventRow key={e.id} event={e} />)}
          </div>
        </div>
      )}

      {total > 0 && (
        <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <span>{from}-{to} of {total}</span>
            <span aria-hidden="true">·</span>
            <label className="flex items-center gap-1.5">
              Per page
              <select
                value={pageSize}
                onChange={e => { setOffset(0); setPageSize(Number(e.target.value)) }}
                className="h-8 rounded-md border border-input bg-background px-2 text-xs"
              >
                {PAGE_SIZES.map(n => <option key={n} value={n}>{n}</option>)}
              </select>
            </label>
          </div>
          <nav className="flex items-center gap-1" aria-label="Pages">
            <PageButton label="First page" disabled={page === 1 || loading} onClick={() => goTo(1)}><ChevronsLeft className="w-4 h-4" /></PageButton>
            <PageButton label="Previous page" disabled={page === 1 || loading} onClick={() => goTo(page - 1)}><ChevronLeft className="w-4 h-4" /></PageButton>
            {pageNumbers(page, pageCount).map((p, i) =>
              p === null
                ? <span key={`gap-${i}`} className="px-1 text-xs text-muted-foreground">…</span>
                : (
                  <PageButton key={p} label={`Page ${p}`} current={p === page} disabled={loading} onClick={() => goTo(p)}>
                    {p}
                  </PageButton>
                ),
            )}
            <PageButton label="Next page" disabled={page === pageCount || loading} onClick={() => goTo(page + 1)}><ChevronRight className="w-4 h-4" /></PageButton>
            <PageButton label="Last page" disabled={page === pageCount || loading} onClick={() => goTo(pageCount)}><ChevronsRight className="w-4 h-4" /></PageButton>
          </nav>
        </div>
      )}
    </div>
  )
}

function PageButton({ label, current, disabled, onClick, children }: {
  label: string; current?: boolean; disabled?: boolean; onClick: () => void; children: React.ReactNode
}) {
  return (
    <Button
      variant={current ? 'default' : 'outline'}
      size="sm"
      className="h-8 min-w-8 px-2"
      aria-label={label}
      aria-current={current ? 'page' : undefined}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </Button>
  )
}

/** Page numbers around the current one, with gaps: 1 … 4 5 6 … 12 */
function pageNumbers(page: number, count: number): (number | null)[] {
  const set = new Set([1, count, page - 1, page, page + 1].filter(p => p >= 1 && p <= count))
  const sorted = [...set].sort((a, b) => a - b)
  const out: (number | null)[] = []
  sorted.forEach((p, i) => {
    if (i > 0 && p - sorted[i - 1] > 1) out.push(null)
    out.push(p)
  })
  return out
}

function EventRow({ event }: { event: VPNEvent }) {
  const connected = event.event_type === 'connected'
  return (
    <div className="grid grid-cols-1 lg:grid-cols-[130px_1.2fr_1fr_120px_140px_1.2fr_150px] gap-2 lg:gap-3 px-4 py-3 text-sm">
      <div className="flex items-center gap-2">
        <span className={`w-8 h-8 rounded-full flex items-center justify-center ${connected ? 'bg-success/10 text-success' : 'bg-muted text-muted-foreground'}`}>
          {connected ? <LogIn className="w-4 h-4" /> : <LogOut className="w-4 h-4" />}
        </span>
        <div>
          <Badge variant={connected ? 'success' : 'secondary'}>{connected ? 'Connected' : 'Disconnected'}</Badge>
          {event.reason && <p className="text-[11px] text-muted-foreground mt-1">{event.reason}</p>}
        </div>
      </div>
      <Cell label="User" primary={event.username ?? event.email ?? 'Unknown user'} secondary={event.email && event.email !== event.username ? event.email : undefined} />
      <Cell label="Server" primary={event.server_name ?? 'Unknown server'} />
      <Cell label="VPN IP" primary={event.assigned_ip} mono />
      <Cell label="Source IP" primary={event.source_ip ?? 'unknown'} mono />
      <Cell label="Device" primary={event.device_name || event.device_id || 'unknown'} secondary={event.user_agent ?? undefined} />
      <Cell label="Time" primary={formatDate(event.created_at)} secondary={formatRelative(event.created_at)} />
    </div>
  )
}

function Cell({ label, primary, secondary, mono }: { label: string; primary: string; secondary?: string; mono?: boolean }) {
  return (
    <div className="min-w-0">
      <p className="lg:hidden text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/70">{label}</p>
      <p className={mono ? 'font-mono text-xs truncate' : 'font-medium truncate'}>{primary}</p>
      {secondary && <p className="text-xs text-muted-foreground truncate mt-0.5">{secondary}</p>}
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <Label className="text-xs text-muted-foreground">{label}</Label>
      <div className="mt-1">{children}</div>
    </div>
  )
}

function userLabel(user: User): string {
  const name = `${user.first_name ?? ''} ${user.last_name ?? ''}`.trim()
  if (name) return `${name} (${user.username})`
  return user.username || user.email
}

function sinceDate(value: SinceFilter): Date | undefined {
  const d = new Date()
  if (value === '24h') d.setHours(d.getHours() - 24)
  else if (value === '7d') d.setDate(d.getDate() - 7)
  else if (value === '30d') d.setDate(d.getDate() - 30)
  else return undefined
  return d
}

function formatDate(ts: string): string {
  return new Date(ts).toLocaleString()
}

function formatRelative(ts: string): string {
  const diff = Math.floor((Date.now() - new Date(ts).getTime()) / 1000)
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}
