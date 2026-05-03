/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        bg: {
          base:    '#0a0c12',
          surface: '#0f1117',
          card:    '#161b22',
          hover:   '#1c2128',
          border:  '#21262d',
        },
        text: {
          primary:   '#e6edf3',
          secondary: '#8b949e',
          muted:     '#484f58',
        },
        accent: {
          DEFAULT: '#7c3aed',
          hover:   '#6d28d9',
          light:   '#a78bfa',
        },
        success: {
          DEFAULT: '#3fb950',
          light:   '#56d364',
          bg:      '#0d2a18',
        },
        danger: {
          DEFAULT: '#f85149',
          light:   '#ff7b72',
          bg:      '#2d1216',
        },
        warning: {
          DEFAULT: '#d29922',
          light:   '#e3b341',
          bg:      '#2d2110',
        },
      },
      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'Consolas', 'monospace'],
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'spin-slow':  'spin 2s linear infinite',
        'fade-in':    'fadeIn 0.15s ease-out',
        'slide-up':   'slideUp 0.2s ease-out',
      },
      keyframes: {
        fadeIn: {
          from: { opacity: '0' },
          to:   { opacity: '1' },
        },
        slideUp: {
          from: { opacity: '0', transform: 'translateY(8px)' },
          to:   { opacity: '1', transform: 'translateY(0)' },
        },
      },
    },
  },
  plugins: [],
}
