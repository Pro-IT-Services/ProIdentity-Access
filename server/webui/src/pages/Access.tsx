import { useState } from 'react'
import { Users, Network, Boxes } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { cn } from '@/lib/utils'
import People from './access/People'
import Resources from './access/Resources'
import Bundles from './access/Bundles'

type Tab = 'people' | 'resources' | 'bundles'

const TABS: { id: Tab; label: string; icon: React.ElementType; hint: string }[] = [
  { id: 'people',    label: 'People',    icon: Users,    hint: 'Users and what they can reach' },
  { id: 'resources', label: 'Resources', icon: Network,  hint: 'LAN destinations — single hosts or subnets' },
  { id: 'bundles',   label: 'Bundles',   icon: Boxes,    hint: 'Named groups of resources, attached to servers' },
]

export default function Access() {
  const [tab, setTab] = useState<Tab>(() => {
    const h = window.location.hash.replace('#', '')
    return (TABS.find(t => t.id === h)?.id) ?? 'people'
  })

  const onTab = (t: Tab) => {
    setTab(t)
    window.history.replaceState(null, '', `#${t}`)
  }

  const active = TABS.find(t => t.id === tab)!

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <PageHeader
        title="Access"
        description={active.hint}
      />

      <div className="border-b border-border mb-6 -mx-6 px-6">
        <div className="flex gap-1">
          {TABS.map(t => (
            <button
              key={t.id}
              onClick={() => onTab(t.id)}
              className={cn(
                'inline-flex items-center gap-2 px-4 py-2.5 text-sm border-b-2 transition-colors cursor-pointer -mb-px',
                tab === t.id
                  ? 'border-primary text-foreground font-medium'
                  : 'border-transparent text-muted-foreground hover:text-foreground hover:border-border',
              )}
            >
              <t.icon className="w-4 h-4" />
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {tab === 'people'    && <People />}
      {tab === 'resources' && <Resources />}
      {tab === 'bundles'   && <Bundles />}
    </div>
  )
}
