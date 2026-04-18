import type { Config } from 'tailwindcss'

const config: Config = {
  content: [
    './pages/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
    './app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        background:       '#08080E',
        surface:          '#0E0E1C',
        'surface-2':      '#141428',
        'surface-3':      '#1C1C38',
        'text-primary':   '#EEE8FF',
        'text-secondary': '#8B82B0',
        'text-muted':     '#52496E',
        accent:           '#C026D3',
        'accent-hover':   '#D946EF',
        purple:           '#7C3AED',
        success:          '#4ADE80',
        warning:          '#FBBF24',
        danger:           '#F87171',
      },
      fontFamily: {
        sans: ['Inter', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'sans-serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
      },
      boxShadow: {
        card:         '0 4px 24px rgba(0,0,0,0.6)',
        'glow-purple': '0 0 28px rgba(192,38,211,0.25)',
      },
      borderRadius: {
        card: '14px',
      },
      animation: {
        'fade-in':       'fadeIn 0.2s ease-out',
        'slide-up':      'slideUp 0.2s ease-out',
        'slide-in-right':'slideInRight 0.22s ease-out',
      },
    },
  },
  plugins: [],
}

export default config
