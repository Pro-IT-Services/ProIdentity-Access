import { Shield, Plus, Settings } from 'lucide-react'

interface HeaderProps {
  onImport: () => void
  onSettings: () => void
}

export default function Header({ onImport, onSettings }: HeaderProps) {
  return (
    <header className="flex items-center justify-between px-4 py-3 border-b border-bg-border bg-bg-surface flex-shrink-0"
      style={{ paddingTop: 'max(12px, env(safe-area-inset-top))' }}>
      {/* App title */}
      <div className="flex items-center gap-2.5">
        <Shield className="w-5 h-5 text-accent" />
        <span className="font-semibold text-text-primary tracking-tight">WireGuard</span>
      </div>

      {/* Right side */}
      <div className="flex items-center gap-2">
        {/* Settings button */}
        <button
          onClick={onSettings}
          className="p-2.5 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-border transition-colors"
          title="Managed server settings"
        >
          <Settings className="w-5 h-5" />
        </button>

        {/* Import button */}
        <button
          onClick={onImport}
          className="flex items-center gap-1.5 px-4 py-2.5 bg-accent hover:bg-accent-hover text-white text-sm font-medium rounded-lg transition-colors"
        >
          <Plus className="w-4 h-4" />
          Import
        </button>
      </div>
    </header>
  )
}
