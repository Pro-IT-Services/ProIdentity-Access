import { forwardRef } from 'react'
import { cn } from '../../lib/cn'

type Variant = 'primary' | 'destructive' | 'ghost' | 'secondary' | 'outline'
type Size    = 'sm' | 'md' | 'lg' | 'icon'

interface Props extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
}

const VARIANT: Record<Variant, string> = {
  primary:     'bg-primary text-primary-foreground hover:bg-primary/90 disabled:bg-primary/40',
  destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90 disabled:bg-destructive/40',
  ghost:       'bg-transparent hover:bg-secondary text-foreground',
  secondary:   'bg-secondary text-secondary-foreground hover:bg-secondary/80',
  outline:     'border border-border bg-transparent hover:bg-secondary text-foreground',
}

const SIZE: Record<Size, string> = {
  sm:   'h-8 px-3 text-xs gap-1.5',
  md:   'h-9 px-4 text-sm gap-2',
  lg:   'h-11 px-6 text-base gap-2',
  icon: 'h-9 w-9 p-0',
}

export const Button = forwardRef<HTMLButtonElement, Props>(function Button(
  { className, variant = 'primary', size = 'md', disabled, children, ...props }, ref,
) {
  return (
    <button
      ref={ref}
      disabled={disabled}
      className={cn(
        'inline-flex items-center justify-center font-medium rounded-md transition-colors duration-150',
        'focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
        'disabled:cursor-not-allowed disabled:opacity-70',
        !disabled && 'cursor-pointer no-drag',
        VARIANT[variant],
        SIZE[size],
        className,
      )}
      {...props}
    >
      {children}
    </button>
  )
})
