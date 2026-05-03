/** @type {import('tailwindcss').Config} */
export default {
  darkMode: ['class'],
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
      },
      colors: {
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        card:        { DEFAULT: 'hsl(var(--card))',        foreground: 'hsl(var(--card-foreground))' },
        popover:     { DEFAULT: 'hsl(var(--popover))',     foreground: 'hsl(var(--popover-foreground))' },
        muted:       { DEFAULT: 'hsl(var(--muted))',       foreground: 'hsl(var(--muted-foreground))' },
        secondary:   { DEFAULT: 'hsl(var(--secondary))',   foreground: 'hsl(var(--secondary-foreground))' },
        accent:      { DEFAULT: 'hsl(var(--accent))',      foreground: 'hsl(var(--accent-foreground))' },
        primary:     { DEFAULT: 'hsl(var(--primary))',     foreground: 'hsl(var(--primary-foreground))' },
        destructive: { DEFAULT: 'hsl(var(--destructive))', foreground: 'hsl(var(--destructive-foreground))' },
        success:  'hsl(var(--success))',
        warning:  'hsl(var(--warning))',
        info:     'hsl(var(--info))',
        border:   'hsl(var(--border))',
        input:    'hsl(var(--input))',
        ring:     'hsl(var(--ring))',

        // Legacy aliases — let old components keep compiling during migration.
        bg: {
          base:    'hsl(var(--background))',
          surface: 'hsl(var(--background))',
          card:    'hsl(var(--card))',
          hover:   'hsl(var(--secondary))',
          border:  'hsl(var(--border))',
        },
        text: {
          primary:   'hsl(var(--foreground))',
          secondary: 'hsl(var(--muted-foreground))',
          muted:     'hsl(var(--muted-foreground) / 0.7)',
        },
        danger: {
          DEFAULT: 'hsl(var(--destructive))',
          light:   'hsl(var(--destructive))',
          bg:      'hsl(var(--destructive) / 0.1)',
        },
      },
      borderRadius: {
        lg: 'var(--radius)',
        md: 'calc(var(--radius) - 2px)',
        sm: 'calc(var(--radius) - 4px)',
      },
      keyframes: {
        fadeIn:        { from: { opacity: '0' },                                  to: { opacity: '1' } },
        slideUp:       { from: { opacity: '0', transform: 'translateY(8px)' },    to: { opacity: '1', transform: 'translateY(0)' } },
        slideInRight:  { from: { transform: 'translateX(100%)' },                 to: { transform: 'translateX(0)' } },
        slideOutRight: { from: { transform: 'translateX(0)' },                    to: { transform: 'translateX(100%)' } },
      },
      animation: {
        'fade-in':    'fadeIn 150ms ease-out',
        'slide-up':   'slideUp 200ms ease-out',
        'slide-in':   'slideInRight 220ms cubic-bezier(0.32, 0.72, 0, 1)',
        'slide-out':  'slideOutRight 200ms cubic-bezier(0.32, 0.72, 0, 1)',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
      },
    },
  },
  plugins: [],
}
