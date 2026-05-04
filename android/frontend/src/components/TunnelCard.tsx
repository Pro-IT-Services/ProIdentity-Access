import { Wifi, WifiOff, Loader2, AlertCircle } from 'lucide-react'
import type { TunnelInfo } from '../types'

interface TunnelCardProps {
  tunnel: TunnelInfo
  selected: boolean
  onClick: () => void
}

export default function TunnelCard({ tunnel, selected, onClick }: TunnelCardProps) {
  return (
    <button
      onClick={onClick}
      className={`w-full text-left px-4 py-4 rounded-xl transition-colors group ${
        selected
          ? 'bg-bg-hover border border-accent/30'
          : 'hover:bg-bg-hover border border-transparent'
      }`}
    >
      <div className="flex items-center gap-3">
        <StatusIcon status={tunnel.status} />
        <div className="flex-1 min-w-0">
          <p className={`text-base font-medium truncate ${
            selected ? 'text-text-primary' : 'text-text-secondary group-hover:text-text-primary'
          } transition-colors`}>
            {tunnel.name}
          </p>
          <p className="text-sm text-text-muted truncate mt-0.5">
            {tunnel.addresses?.[0] ?? 'No address'}
          </p>
        </div>
      </div>
    </button>
  )
}

function StatusIcon({ status }: { status: TunnelInfo['status'] }) {
  switch (status) {
    case 'connected':
      return <Wifi className="w-5 h-5 text-success flex-shrink-0" />
    case 'connecting':
      return <Loader2 className="w-5 h-5 text-warning animate-spin-slow flex-shrink-0" />
    case 'error':
      return <AlertCircle className="w-5 h-5 text-danger flex-shrink-0" />
    default:
      return <WifiOff className="w-5 h-5 text-text-muted flex-shrink-0" />
  }
}
