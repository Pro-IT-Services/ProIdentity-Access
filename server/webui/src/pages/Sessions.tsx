import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type Session } from '../api/client'
import { useAuthStore } from '../stores/useAuthStore'
import { History, RefreshCw, Trash2, Wifi, WifiOff } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'

/** The signed-in user's own VPN sessions. Everyone's are in Connection History. */
export default function Sessions() {
  const user = useAuthStore(s => s.user)
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)

  const load = async () => {
    setLoading(true)
    try {
      setSessions(await api.mySessions())
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [user?.id])

  const disconnect = async (id: string) => {
    if (!confirm('End this session? The device is disconnected.')) return
    await api.deleteSession(id)
    setSessions(prev => prev.filter(s => s.id !== id))
  }

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">My Sessions</h1>
          <p className="text-muted-foreground text-sm mt-0.5">
            {sessions.length} active {sessions.length === 1 ? 'session' : 'sessions'} of {user?.username}
          </p>
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
                    {s.server_name && <span className="font-medium text-sm">{s.server_name}</span>}
                    <span className="font-mono text-primary text-sm">{s.assigned_ip}</span>
                    <Badge variant="success">Active</Badge>
                  </div>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    Connected {formatRelative(s.created_at)} - Last seen {formatRelative(s.last_keepalive)}
                  </p>
                  <p className="text-xs text-muted-foreground mt-1">{details(s)}</p>
                </div>
                <Button variant="destructive" size="sm" onClick={() => disconnect(s.id)}>
                  <Trash2 /> End session
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {user?.is_admin && (
        <p className="mt-6 text-xs text-muted-foreground flex items-center gap-1.5">
          <History className="w-3.5 h-3.5" />
          <span>
            Sessions and connection events of all users are in{' '}
            <Link to="/connection-history" className="text-primary hover:underline">Connection History</Link>.
          </span>
        </p>
      )}
    </div>
  )
}

function details(s: Session): string {
  const parts: string[] = []
  if (s.source_ip) parts.push(`Source ${s.source_ip}`)
  if (s.device_name || s.device_id) parts.push(`Device ${s.device_name || s.device_id}`)
  return parts.length > 0 ? parts.join(' - ') : 'Source and device not recorded for this session'
}

function formatRelative(ts: string): string {
  const diff = Math.floor((Date.now() - new Date(ts).getTime()) / 1000)
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}
