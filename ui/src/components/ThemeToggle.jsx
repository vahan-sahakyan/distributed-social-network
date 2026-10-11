import { Monitor, Sun, Moon } from 'lucide-react'
import { useTheme } from '../theme'

const OPTIONS = [
  { value: 'system', label: 'System', Icon: Monitor },
  { value: 'light', label: 'Light', Icon: Sun },
  { value: 'dark', label: 'Dark', Icon: Moon },
]

export function ThemeToggle() {
  const [theme, setTheme] = useTheme()

  return (
    <div role="radiogroup" aria-label="Theme" className="flex items-center gap-0.5 p-0.5 rounded-lg border border-border bg-surface">
      {OPTIONS.map(({ value, label, Icon }) => (
        <button
          key={value}
          role="radio"
          aria-checked={theme === value}
          title={label}
          onClick={() => setTheme(value)}
          className={
            'flex-1 flex items-center justify-center py-1.5 rounded-md transition-colors ' +
            (theme === value ? 'bg-bg text-text shadow-sm' : 'text-muted hover:text-text')
          }
        >
          <Icon size={13} />
          <span className="sr-only">{label}</span>
        </button>
      ))}
    </div>
  )
}
