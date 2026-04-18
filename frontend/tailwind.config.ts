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
        background:       '#111110',
        surface:          '#191817',
        'surface-2':      '#201E1C',
        'surface-3':      '#2A2724',
        'text-primary':   '#F0EDE7',
        'text-secondary': '#A09890',
        'text-muted':     '#625850',
        accent:           '#DA7756',
        'accent-hover':   '#E88A68',
        success:          '#4DB88A',
        warning:          '#C8993C',
        danger:           '#D95555',
      },
      fontFamily: {
        sans: ['Inter', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'sans-serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
      },
      boxShadow: {
        card:        '0 4px 24px rgba(0,0,0,0.5)',
        'glow-warm': '0 0 24px rgba(218,119,86,0.18)',
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
