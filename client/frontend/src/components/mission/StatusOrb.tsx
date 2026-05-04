import { useMemo } from 'react'
import { AlertCircle, Loader2 } from 'lucide-react'
import type { Status } from '../ui/StatusDot'
import { LogoMark } from '../brand/LogoMark'
import { cn } from '../../lib/cn'

interface Props {
  status: Status
  /** 0..1 — how lit the throughput halo should be (we map total bps logarithmically) */
  intensity?: number
  size?: number
}

const COLOR: Record<Status, { core: string; ring: string; glow: string; text: string }> = {
  connected:    { core: 'hsl(142 71% 45%)',  ring: 'hsl(142 71% 45% / 0.6)',  glow: 'hsl(142 71% 45% / 0.35)', text: 'text-success' },
  connecting:   { core: 'hsl(38 92% 50%)',   ring: 'hsl(38 92% 50% / 0.6)',   glow: 'hsl(38 92% 50% / 0.35)',  text: 'text-warning' },
  reconnecting: { core: 'hsl(38 92% 50%)',   ring: 'hsl(38 92% 50% / 0.6)',   glow: 'hsl(38 92% 50% / 0.35)',  text: 'text-warning' },
  error:        { core: 'hsl(0 72% 55%)',    ring: 'hsl(0 72% 55% / 0.6)',    glow: 'hsl(0 72% 55% / 0.35)',   text: 'text-destructive' },
  disconnected: { core: 'hsl(215 14% 56%)',  ring: 'hsl(215 14% 56% / 0.4)',  glow: 'hsl(215 14% 56% / 0.15)', text: 'text-muted-foreground' },
}

export function StatusOrb({ status, intensity = 0, size = 240 }: Props) {
  const c = COLOR[status]
  const animate = status === 'connected' || status === 'connecting' || status === 'reconnecting'

  // Inner core radius and outer halo radius
  const cx = size / 2
  const cy = size / 2
  const coreR = size * 0.18

  // Build the throughput halo — a ring whose stroke opacity tracks `intensity`.
  const haloR = size * 0.32
  const haloOpacity = useMemo(() => {
    // Compress 0..1 with a soft curve so a tiny bit of traffic still feels visible.
    const v = Math.min(1, Math.max(0, intensity))
    return 0.15 + 0.45 * Math.sqrt(v)
  }, [intensity])

  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      {/* Pulse rings — pure CSS so they keep going without re-renders. */}
      {animate && (
        <>
          <span className={cn('mc-pulse', `mc-pulse-${status}`, 'mc-delay-0')} style={{ width: size * 0.55, height: size * 0.55 }} />
          <span className={cn('mc-pulse', `mc-pulse-${status}`, 'mc-delay-1')} style={{ width: size * 0.55, height: size * 0.55 }} />
          <span className={cn('mc-pulse', `mc-pulse-${status}`, 'mc-delay-2')} style={{ width: size * 0.55, height: size * 0.55 }} />
        </>
      )}

      {/* SVG: glow + halo ring + core disc */}
      <svg width={size} height={size} className="block relative">
        {/* Soft outer glow — radial gradient */}
        <defs>
          <radialGradient id={`orb-glow-${status}`} cx="50%" cy="50%" r="50%">
            <stop offset="0%" stopColor={c.glow} stopOpacity="0.9" />
            <stop offset="55%" stopColor={c.glow} stopOpacity="0.25" />
            <stop offset="100%" stopColor={c.glow} stopOpacity="0" />
          </radialGradient>
        </defs>
        <circle cx={cx} cy={cy} r={size * 0.45} fill={`url(#orb-glow-${status})`} />

        {/* Throughput halo ring — opacity-modulated by intensity */}
        <circle
          cx={cx} cy={cy} r={haloR}
          fill="none"
          stroke={c.ring}
          strokeWidth={4}
          strokeLinecap="round"
          opacity={haloOpacity}
          className="transition-opacity duration-500"
        />

        {/* Core disc */}
        <circle cx={cx} cy={cy} r={coreR} fill={c.core} />
        <circle cx={cx} cy={cy} r={coreR} fill="none" stroke="white" strokeOpacity="0.18" strokeWidth="1" />
      </svg>

      {/* Center icon */}
      <span className="absolute inset-0 flex items-center justify-center pointer-events-none">
        {status === 'connecting' || status === 'reconnecting'
          ? <Loader2 className="w-7 h-7 text-white/95 animate-spin" />
          : status === 'error'
            ? <AlertCircle className="w-8 h-8 text-white/95" />
            : <LogoMark size={Math.round(coreR * 1.5)} className="text-white/95" weight={1.4} />}
      </span>

      {/* Inline animation styles */}
      <style>{`
        .mc-pulse {
          position: absolute;
          border-radius: 9999px;
          border: 2px solid ${c.ring};
          opacity: 0;
          transform: scale(1);
          animation: mc-pulse-anim 2.8s cubic-bezier(0.22, 0.61, 0.36, 1) infinite;
        }
        .mc-delay-0 { animation-delay: 0s; }
        .mc-delay-1 { animation-delay: 0.9s; }
        .mc-delay-2 { animation-delay: 1.8s; }
        @keyframes mc-pulse-anim {
          0%   { transform: scale(1);   opacity: 0.55; }
          80%  { transform: scale(2.4); opacity: 0; }
          100% { transform: scale(2.4); opacity: 0; }
        }
        @media (prefers-reduced-motion: reduce) {
          .mc-pulse { animation: none; }
        }
      `}</style>
    </div>
  )
}
