import { X, CheckCircle, AlertCircle } from 'lucide-react'
import { useStore } from '../store'

export function Toasts() {
  const { toasts, removeToast } = useStore()
  if (!toasts.length) return null

  return (
    <div className="fixed bottom-6 right-6 z-50 flex flex-col gap-2 items-end">
      {toasts.map(t => (
        <div
          key={t.id}
          className={`flex items-center gap-3 px-4 py-3 rounded-xl border text-sm font-medium shadow-2xl max-w-xs
            ${t.type === 'error'
              ? 'bg-red-950/90 border-red-800/60 text-red-200'
              : 'bg-surface border-border text-text'
            }`}
          style={{ backdropFilter: 'blur(8px)' }}
        >
          {t.type === 'error'
            ? <AlertCircle size={14} className="text-red-400 shrink-0" />
            : <CheckCircle size={14} className="text-green-400 shrink-0" />
          }
          <span className="flex-1">{t.message}</span>
          <button
            onClick={() => removeToast(t.id)}
            className="text-muted hover:text-text transition-colors shrink-0"
          >
            <X size={13} />
          </button>
        </div>
      ))}
    </div>
  )
}
