import { Sun, Moon, Monitor } from 'lucide-react'
import { useThemeStore } from '../stores/useThemeStore'
import { cn } from '@/lib/utils'

export function ThemeToggle({ className }: { className?: string }) {
  const { theme, toggle } = useThemeStore()
  const Icon = theme === 'light' ? Sun : theme === 'dark' ? Moon : Monitor
  const label = theme === 'light' ? 'Light' : theme === 'dark' ? 'Dark' : 'System'

  return (
    <button
      onClick={toggle}
      title={`Theme: ${label} — click to cycle`}
      aria-label={`Switch theme (currently ${label})`}
      className={cn(
        'inline-flex items-center justify-center w-8 h-8 rounded-md text-muted-foreground',
        'hover:text-foreground hover:bg-secondary transition-colors cursor-pointer',
        className,
      )}
    >
      <Icon className="w-4 h-4" />
    </button>
  )
}
