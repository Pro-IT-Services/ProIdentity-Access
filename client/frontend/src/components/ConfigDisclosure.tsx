import type { TunnelInfo } from '../types'
import { MonoChip } from './ui/MonoChip'

export function ConfigDisclosure({ tunnel }: { tunnel: TunnelInfo }) {
  const peer = tunnel.peers?.[0]
  const addresses = tunnel.addresses ?? []
  const dns = tunnel.dns ?? []

  return (
    <div className="space-y-5">
      <Section title="Interface">
        <KV label="Address">
          {addresses.length === 0
            ? <span className="text-muted-foreground">-</span>
            : <div className="flex flex-wrap gap-1">{addresses.map(a => <MonoChip key={a} value={a} />)}</div>}
        </KV>
        <KV label="MTU"><MonoChip value={String(tunnel.mtu || 1420)} copy={false} /></KV>
        {tunnel.listen_port > 0 && (
          <KV label="Listen port"><MonoChip value={String(tunnel.listen_port)} copy={false} /></KV>
        )}
        {dns.length > 0 && (
          <KV label="DNS">
            <div className="flex flex-wrap gap-1">{dns.map(d => <MonoChip key={d} value={d} />)}</div>
          </KV>
        )}
      </Section>

      {peer && (
        <Section title="Peer">
          <KV label="Endpoint">
            <MonoChip value={peer.endpoint ?? ''} display={peer.endpoint || '-'} />
          </KV>
          <KV label="Allowed IPs">
            <div className="flex flex-wrap gap-1">
              {(peer.allowed_ips ?? []).map(a => <MonoChip key={a} value={a} />)}
            </div>
          </KV>
          {peer.persistent_keepalive > 0 && (
            <KV label="Keepalive"><MonoChip value={`${peer.persistent_keepalive}s`} copy={false} /></KV>
          )}
        </Section>
      )}
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="text-[10px] uppercase tracking-wider text-muted-foreground font-semibold mb-2">{title}</p>
      <div className="space-y-1.5">{children}</div>
    </div>
  )
}

function KV({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[120px_1fr] items-start gap-3 text-xs">
      <span className="text-muted-foreground pt-1">{label}</span>
      <div className="min-w-0">{children}</div>
    </div>
  )
}
