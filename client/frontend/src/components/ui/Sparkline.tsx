import { useMemo } from 'react'
import { cn } from '../../lib/cn'

interface SparklineProps {
  /** Data points — most recent last. Length determines x scaling. */
  data: number[]
  width?: number
  height?: number
  color?: string  // CSS color (e.g. 'hsl(var(--success))')
  className?: string
  /** Render an area fill below the line for emphasis. */
  fill?: boolean
}

/** Minimal hand-rolled SVG sparkline — no library dep. */
export function Sparkline({
  data, width = 80, height = 24, color = 'currentColor', className, fill = true,
}: SparklineProps) {
  const path = useMemo(() => {
    if (data.length === 0) return { line: '', area: '' }
    const max = Math.max(...data, 1)
    const min = 0
    const range = max - min || 1
    const stepX = data.length > 1 ? width / (data.length - 1) : 0
    const points = data.map((v, i) => {
      const x = i * stepX
      const y = height - ((v - min) / range) * (height - 2) - 1
      return [x, y] as const
    })
    const line = points.map(([x, y], i) => `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`).join(' ')
    const area = `${line} L ${(width).toFixed(1)} ${height} L 0 ${height} Z`
    return { line, area }
  }, [data, width, height])

  if (data.length < 2) {
    return (
      <svg width={width} height={height} className={cn('block', className)} aria-hidden="true">
        <line x1="0" y1={height - 1} x2={width} y2={height - 1} stroke="currentColor" className="text-border" strokeWidth="1" />
      </svg>
    )
  }

  return (
    <svg width={width} height={height} className={cn('block', className)} aria-hidden="true" viewBox={`0 0 ${width} ${height}`}>
      {fill && <path d={path.area} fill={color} fillOpacity="0.15" />}
      <path d={path.line} fill="none" stroke={color} strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round" />
    </svg>
  )
}
