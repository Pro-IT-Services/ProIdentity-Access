import { useEffect, useState } from 'react'
import { api, type Session, type AdminSession } from '../api/client'
import { useAuthStore } from '../stores/useAuthStore'
import { Wifi, WifiOff, RefreshCw, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'

export default function Sessions() {
  const user = useAuthStore(s => s.user)
  const [sessions, setSessions] = useState<(Session | AdminSession)[]>([])
  const [loading, setLoading] = useState(true)

  const load = async () => {
    setLoading(true)
    try {
      if (user?.is_admin) setSessions(await api.listAllSessions())
      else setSessions(await api.mySessions())
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [user])

  const terminate = async (id: string) => {
    if (!confirm('Terminate this session?')) return
    if (user?.is_admin) await api.terminateSession(id)
    else await api.deleteSession(id)
    setSessions(prev => prev.filter(s => s.id !== id))
  }

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">{user?.is_admin ? 'All Active Sessions' : 'My Sessions'}</h1>
          <p className="text-muted-foreground text-sm mt-0.5">{sessions.length} active</p>
        </div>
        <Button variant="ghost" onClick={load}>
          <RefreshCw className={loading ? 'animate-spin' : ''} />
          Refresh
        </Button>
      </div>

      {sessions.length === 0 ? (
        <Card>
          <CardContent className="p-12 flex flex-col items-center text-muted-foreground">
            <WifiOff className="w-10 h-10 mb-3 opacity-40" />
            <p className="text-sm">No active sessions</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-2">
          {sessions.map(s => (
            <Card key={s.id}>
              <CardContent className="p-4 flex items-center gap-4">
                <div className="w-9 h-9 rounded-full bg-success/10 flex items-center justify-center shrink-0">
                  <Wifi className="w-4 h-4 text-success" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    {'username' in s && <span className="font-medium text-sm">{(s as AdminSession).username}</span>}
                    <span className="font-mono text-primary text-sm">{s.assigned_ip}</span>
                    <Badge variant="success">Active</Badge>
                  </div>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    Connected {formatRelative(s.created_at)} · Last seen {formatRelative(s.last_keepalive)}
                  </p>
                </div>
                <Button variant="destructive" size="sm" onClick={() => terminate(s.id)}>
                  <Trash2 /> Terminate
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}

function formatRelative(ts: string): string {
  const diff = Math.floor((Date.now() - new Date(ts).getTime()) / 1000)
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  return `${Math.floor(diff / 3600)}h ago`
}
