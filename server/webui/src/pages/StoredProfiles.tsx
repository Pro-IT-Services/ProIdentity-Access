import { useEffect, useMemo, useState } from 'react'
import { api, type UserConfig } from '../api/client'
import { FileLock2, Search, Trash2 } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { Empty } from '@/components/Empty'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

/**
 * WireGuard configurations users saved to the server from their apps. They
 * belong to the user (not to a VPN server) and are stored encrypted.
 */
export default function StoredProfiles() {
  const [configs, setConfigs] = useState<UserConfig[]>([])
  const [query, setQuery] = useState('')
  const [loaded, setLoaded] = useState(false)

  const load = () => api.adminListUserConfigs()
    .then(d => setConfigs(d ?? []))
    .finally(() => setLoaded(true))
  useEffect(() => { load() }, [])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return configs
    return configs.filter(c =>
      c.name.toLowerCase().includes(q) ||
      (c.username ?? '').toLowerCase().includes(q) ||
      (c.email ?? '').toLowerCase().includes(q))
  }, [configs, query])

  // Grouped by user, users in name order.
  const byUser = useMemo(() => {
    const m = new Map<string, { username: string; email: string; items: UserConfig[] }>()
    for (const c of filtered) {
      const g = m.get(c.user_id) ?? { username: c.username, email: c.email, items: [] }
      g.items.push(c)
      m.set(c.user_id, g)
    }
    return [...m.entries()].sort((a, b) => a[1].username.localeCompare(b[1].username))
  }, [filtered])

  const remove = async (c: UserConfig) => {
    if (!confirm(`Delete "${c.name}" stored by ${c.username}? They will no longer see it in their apps.`)) return
    await api.adminDeleteUserConfig(c.id)
    load()
  }

  return (
    <div className="p-6 max-w-5xl mx-auto">
      <PageHeader
        title="Stored WireGuard Profiles"
        description="WireGuard configurations users saved from their apps. They belong to the user, not to a VPN server, and are stored encrypted."
      />

      {configs.length > 0 && (
        <div className="relative mb-4">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder={`Search ${configs.length} ${configs.length === 1 ? 'profile' : 'profiles'} by name, user or email…`}
            className="pl-9"
          />
        </div>
      )}

      {loaded && configs.length === 0 ? (
        <Empty
          icon={FileLock2}
          title="No stored profiles"
          hint="When users save a WireGuard configuration to the server from their app, it shows up here."
        />
      ) : loaded && filtered.length === 0 ? (
        <Empty icon={Search} title="No matches" hint="Try a different search term." />
      ) : (
        <div className="space-y-4">
          {byUser.map(([userId, g]) => (
            <section key={userId} className="rounded-xl border border-border bg-card overflow-hidden">
              <div className="flex items-center justify-between gap-3 px-4 py-2.5 border-b border-border bg-secondary/30">
                <div className="min-w-0">
                  <p className="text-sm font-semibold truncate">{g.username}</p>
                  <p className="text-[11px] text-muted-foreground truncate">{g.email}</p>
                </div>
                <span className="text-xs text-muted-foreground shrink-0">
                  {g.items.length} {g.items.length === 1 ? 'profile' : 'profiles'}
                </span>
              </div>
              <div className="divide-y divide-border">
                {g.items.map(c => (
                  <div key={c.id} className="flex items-center gap-4 px-4 py-3">
                    <FileLock2 className="w-4 h-4 text-muted-foreground shrink-0" />
                    <p className="flex-1 min-w-0 text-sm font-medium truncate">{c.name}</p>
                    <span className="text-xs text-muted-foreground shrink-0" title={new Date(c.created_at).toLocaleString()}>
                      {relTime(c.created_at)}
                    </span>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => remove(c)}
                      className="text-destructive hover:text-destructive hover:bg-destructive/10"
                    >
                      <Trash2 className="w-4 h-4" /> Delete
                    </Button>
                  </div>
                ))}
              </div>
            </section>
          ))}
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
  return `${Math.floor(diff / 86400)}d ago`
}
