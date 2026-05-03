interface Props {
  size?: number
  className?: string
  /** Show the faint outer perimeter ring. Off by default for tight UI use. */
  perimeter?: boolean
  /** Stroke weight scale. 1 = brand default; bump up for very small icon use. */
  weight?: number
}

/**
 * ProIdentity Access aperture mark, inline SVG.
 * Uses currentColor everywhere so it inherits text color (works as a Lucide
 * stand-in inside buttons, topbars, anywhere). Pass `perimeter` to include the
 * faint outer ring (looks great large, busy when small).
 */
export function LogoMark({ size = 24, className, perimeter = false, weight = 1 }: Props) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 256 256"
      className={className}
      role="img"
      aria-label="ProIdentity Access"
    >
      <g fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round">
        {perimeter && (
          <circle cx="128" cy="128" r="100" strokeWidth={2 * weight} strokeOpacity="0.30" />
        )}
        <polygon
          points="178,128 153,171.3 103,171.3 78,128 103,84.7 153,84.7"
          strokeWidth={8 * weight}
        />
        <line x1="178"   y1="128"   x2="192.2" y2="199.3" strokeWidth={6 * weight} />
        <line x1="153"   y1="171.3" x2="98.3"  y2="219.3" strokeWidth={6 * weight} />
        <line x1="103"   y1="171.3" x2="34.1"  y2="148.0" strokeWidth={6 * weight} />
        <line x1="78"    y1="128"   x2="63.8"  y2="56.7"  strokeWidth={6 * weight} />
        <line x1="103"   y1="84.7"  x2="157.7" y2="36.7"  strokeWidth={6 * weight} />
        <line x1="153"   y1="84.7"  x2="221.9" y2="108.0" strokeWidth={6 * weight} />
      </g>
      <circle cx="128" cy="128" r="11" fill="currentColor" />
    </svg>
  )
}

/**
 * Compact mark for very small surfaces (tray icon, favicon, 16-24px).
 * Drops the radial blade lines, keeps just the hexagon + center dot.
 */
export function LogoMarkCompact({ size = 16, className }: { size?: number; className?: string }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 256 256"
      className={className}
      role="img"
      aria-label="ProIdentity Access"
    >
      <polygon
        points="178,128 153,171.3 103,171.3 78,128 103,84.7 153,84.7"
        fill="none"
        stroke="currentColor"
        strokeWidth="14"
        strokeLinejoin="round"
      />
      <circle cx="128" cy="128" r="18" fill="currentColor" />
    </svg>
  )
}
