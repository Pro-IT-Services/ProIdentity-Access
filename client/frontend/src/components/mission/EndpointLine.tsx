import { useEffect, useState } from 'react'

interface Props {
  /** Show the line as live (dashes flowing). Off when disconnected. */
  active: boolean
  /** Visual length of the line in px. */
  length?: number
  /** Color to use for the line. */
  color?: string
}

/**
 * Animated dotted line "you" → "endpoint". Pure SVG; the dash pattern
 * scrolls when active to suggest packet flow.
 */
export function EndpointLine({ active, length = 180, color = 'hsl(var(--success))' }: Props) {
  // We animate stroke-dashoffset with rAF to avoid a CSS-keyframe re-render hassle
  // and so we can pause cleanly when inactive.
  const [offset, setOffset] = useState(0)
  useEffect(() => {
    if (!active) return
    let raf = 0, last = performance.now()
    const tick = (now: number) => {
      const dt = now - last; last = now
      setOffset(o => (o - dt * 0.04) % 16)   // -16 = wraps a 8/8 dash pattern
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [active])

  const h = 28
  return (
    <svg width={length} height={h} aria-hidden="true" className="block">
      {/* Subtle base line */}
      <line x1="0" y1={h / 2} x2={length} y2={h / 2}
        stroke="hsl(var(--border))" strokeWidth="1" strokeDasharray="2 4" />
      {active && (
        <line x1="0" y1={h / 2} x2={length} y2={h / 2}
          stroke={color}
          strokeWidth="2"
          strokeDasharray="8 8"
          strokeDashoffset={offset}
          strokeLinecap="round" />
      )}
      {/* Endpoint dot */}
      <circle cx={length} cy={h / 2} r={3.5} fill={active ? color : 'hsl(var(--muted-foreground))'} />
      {/* Origin dot */}
      <circle cx={0} cy={h / 2} r={3.5} fill={active ? color : 'hsl(var(--muted-foreground))'} />
    </svg>
  )
}
