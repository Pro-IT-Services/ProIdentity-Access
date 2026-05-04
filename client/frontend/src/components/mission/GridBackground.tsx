/**
 * Subtle grid + radial-vignette background, full-bleed. CSS only.
 * Sits behind the Mission Control content as a static layer.
 */
export function GridBackground() {
  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      {/* Square grid lines, very faint */}
      <div
        className="absolute inset-0 opacity-[0.18]"
        style={{
          backgroundImage:
            'linear-gradient(to right, hsl(var(--border)) 1px, transparent 1px),' +
            'linear-gradient(to bottom, hsl(var(--border)) 1px, transparent 1px)',
          backgroundSize: '36px 36px',
        }}
      />
      {/* Radial vignette to focus the center, fade the edges */}
      <div
        className="absolute inset-0"
        style={{
          background: 'radial-gradient(ellipse at center, transparent 30%, hsl(var(--background)) 85%)',
        }}
      />
    </div>
  )
}
