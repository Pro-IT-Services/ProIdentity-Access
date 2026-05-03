import { Plus } from 'lucide-react'
import { useTunnelStore } from '../stores/useTunnelStore'
import TunnelCard from './TunnelCard'
import ManagedPanel from './ManagedPanel'

export default function Sidebar({ onImport }: { onImport: () => void }) {
  const { tunnels, selectedId, setSelected } = useTunnelStore()

  return (
    <div className="flex flex-col flex-1 overflow-y-auto">
      <ManagedPanel />

      <div className="px-4 py-2.5 border-b border-bg-border">
        <p className="text-xs font-medium text-text-muted uppercase tracking-wider">
          Tunnels ({tunnels.length})
        </p>
      </div>

      <div className="flex-1 py-2 px-3 space-y-1">
        {tunnels.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-5 py-16 text-text-secondary">
            <div className="w-16 h-16 rounded-2xl bg-bg-card border border-bg-border flex items-center justify-center">
              <svg className="w-8 h-8 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5}
                  d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
            </div>
            <div className="text-center px-4">
              <p className="text-text-primary font-medium mb-1">No tunnels yet</p>
              <p className="text-sm text-text-muted">Import a WireGuard config or connect via managed server</p>
            </div>
            <button
              onClick={onImport}
              className="flex items-center gap-2 px-5 py-3 bg-accent hover:bg-accent-hover text-white text-sm font-medium rounded-xl transition-colors"
            >
              <Plus className="w-4 h-4" />
              Import Config
            </button>
          </div>
        ) : (
          tunnels.map(tunnel => (
            <TunnelCard
              key={tunnel.id}
              tunnel={tunnel}
              selected={tunnel.id === selectedId}
              onClick={() => setSelected(tunnel.id)}
            />
          ))
        )}
      </div>
    </div>
  )
}
