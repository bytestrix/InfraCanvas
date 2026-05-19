'use client'
import { useEffect, useState } from 'react'

type Theme = 'dark' | 'light'

export function useTheme() {
  const [theme, setTheme] = useState<Theme>('dark')

  useEffect(() => {
    const saved = localStorage.getItem('ic-theme') as Theme | null
    const preferred = window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
    const initial = saved ?? preferred
    apply(initial)
    setTheme(initial)
  }, [])

  function apply(t: Theme) {
    document.documentElement.dataset.theme = t
    localStorage.setItem('ic-theme', t)
  }

  const toggle = () => setTheme(t => {
    const next: Theme = t === 'dark' ? 'light' : 'dark'
    apply(next)
    return next
  })

  return { theme, toggle, isDark: theme === 'dark' }
}
