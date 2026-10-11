import { useEffect, useState } from 'react'

// the user's choice, per browser; 'system' follows the OS and its changes.
// index.html applies the stored one before first paint, so keep the key and values in sync
const KEY = 'socialnet-theme'
export const THEMES = ['system', 'light', 'dark']
const prefersDark = window.matchMedia('(prefers-color-scheme: dark)')

function stored() {
  try {
    const theme = localStorage.getItem(KEY)
    return THEMES.includes(theme) ? theme : 'system'
  } catch {
    return 'system'
  }
}

function apply(theme) {
  const resolved = theme === 'system' ? (prefersDark.matches ? 'dark' : 'light') : theme
  document.documentElement.dataset.theme = resolved
}

/** The theme choice and its setter; applies it to <html> and remembers it. */
export function useTheme() {
  const [theme, setTheme] = useState(stored)

  useEffect(() => {
    apply(theme)
    try { localStorage.setItem(KEY, theme) } catch { /* private mode: applies for this visit only */ }
    if (theme !== 'system') return
    const follow = () => apply('system')
    prefersDark.addEventListener('change', follow)
    return () => prefersDark.removeEventListener('change', follow)
  }, [theme])

  return [theme, setTheme]
}
