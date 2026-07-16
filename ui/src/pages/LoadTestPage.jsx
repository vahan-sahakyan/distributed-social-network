import { useState, useRef } from 'react'
import { Play, BarChart2, Loader } from 'lucide-react'
import { runLoadTest } from '../loadtest'
import { useStore } from '../store'

const MID_SLA = 1200
const HIGH_SLA = 3000

const PRESETS = [
  { label: 'Quick',  n: 20,  concurrency: 5  },
  { label: 'Normal', n: 50,  concurrency: 10 },
  { label: 'Heavy',  n: 150, concurrency: 30 },
]

function latencyColor(ms) {
  if (ms <= 0)   return 'text-muted'
  if (ms < MID_SLA)   return 'text-green-400'
  if (ms < HIGH_SLA)  return 'text-yellow-400'
  return 'text-red-400'
}

function barColor(ms) {
  if (ms < MID_SLA)  return 'bg-green-400/70'
  if (ms < HIGH_SLA) return 'bg-yellow-400/70'
  return 'bg-red-400/70'
}

function Bar({ value, max }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0
  return (
    <div className="flex items-center gap-2 flex-1 min-w-0">
      <div className="h-1.5 bg-border rounded-full flex-1 min-w-0">
        <div
          className={`h-full rounded-full transition-all duration-500 ${barColor(value)}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  )
}

function Stat({ label, value }) {
  return (
    <div className="text-center">
      <p className={`text-sm font-bold tabular-nums ${latencyColor(value)}`}>{value > 0 ? value : '—'}</p>
      <p className="text-[10px] text-muted mt-0.5">{label}</p>
    </div>
  )
}

function ResultRow({ row, maxP99 }) {
  const errPct = row.count > 0 ? Math.round((row.errors / row.count) * 100) : 0
  return (
    <div className="py-3 border-b border-border last:border-0">
      <div className="flex items-center gap-3 mb-2">
        <code className="text-xs font-mono text-text flex-1 min-w-0 truncate">{row.label}</code>
        <div className="flex items-center gap-3 shrink-0">
          <span className="text-[10px] text-muted tabular-nums">{row.ok}/{row.count} ok</span>
          {errPct > 0 && (
            <span className="text-[10px] text-red-400 font-semibold">{errPct}% err</span>
          )}
        </div>
      </div>
      <div className="flex items-center gap-3">
        <Bar value={row.p99} max={maxP99} />
        <div className="flex gap-4 shrink-0">
          <Stat label="p50" value={row.p50} />
          <Stat label="p95" value={row.p95} />
          <Stat label="p99" value={row.p99} />
          <Stat label="avg" value={row.avg} />
        </div>
      </div>
    </div>
  )
}

export function LoadTestPage() {
  const { currentUser } = useStore()
  const [preset, setPreset]       = useState(1)
  const [running, setRunning]     = useState(false)
  const [log, setLog]             = useState([])
  const [results, setResults]     = useState(null)
  const [elapsed, setElapsed]     = useState(null)
  const logRef                    = useRef(null)

  const { n, concurrency } = PRESETS[preset]

  async function handleRun() {
    if (running) return
    setRunning(true)
    setResults(null)
    setLog([])
    setElapsed(null)
    const t0 = performance.now()

    try {
      const rows = await runLoadTest(
        { n, concurrency, seedUserId: currentUser?.id ?? undefined },
        (msg) => {
          setLog(l => [...l, msg])
          setTimeout(() => logRef.current?.scrollTo(0, 9999), 0)
        },
      )
      setResults(rows)
      setElapsed(((performance.now() - t0) / 1000).toFixed(1))
    } finally {
      setRunning(false)
    }
  }

  const maxP99 = results ? Math.max(...results.map(r => r.p99), 1) : 1

  return (
    <div className="p-6 max-w-2xl mx-auto">
      {/* Header */}
      <div className="flex items-center gap-3 mb-6">
        <div className="w-8 h-8 rounded-xl bg-surface border border-border flex items-center justify-center shrink-0">
          <BarChart2 size={15} className="text-accent" />
        </div>
        <div>
          <h1 className="text-base font-bold text-text">Load Test</h1>
          <p className="text-xs text-muted">Benchmark every endpoint — latency in ms</p>
        </div>
      </div>

      {/* Config */}
      <div className="bg-surface border border-border rounded-xl p-4 mb-4">
        <p className="text-[10px] font-semibold text-muted uppercase tracking-widest mb-3">Configuration</p>
        <div className="flex gap-2 mb-4">
          {PRESETS.map((p, i) => (
            <button
              key={p.label}
              onClick={() => setPreset(i)}
              className={`flex-1 py-2 rounded-lg text-xs font-semibold transition-colors border ${
                preset === i
                  ? 'bg-accent border-accent text-white'
                  : 'bg-bg border-border text-muted hover:text-text hover:border-border-hover'
              }`}
            >
              {p.label}
            </button>
          ))}
        </div>
        <div className="flex gap-6 text-xs text-muted mb-4">
          <span><span className="text-text font-medium">{n}</span> requests / endpoint</span>
          <span><span className="text-text font-medium">{concurrency}</span> concurrent</span>
        </div>
        <button
          onClick={handleRun}
          disabled={running}
          className="w-full flex items-center justify-center gap-2 bg-accent hover:bg-accent-hover disabled:opacity-50 text-white text-sm font-semibold py-2.5 rounded-xl transition-colors"
        >
          {running
            ? <><Loader size={13} className="animate-spin" />Running…</>
            : <><Play size={13} fill="white" />Run Load Test</>}
        </button>
      </div>

      {/* Progress log */}
      {log.length > 0 && (
        <div
          ref={logRef}
          className="bg-surface border border-border rounded-xl p-3 mb-4 max-h-32 overflow-y-auto font-mono"
        >
          {log.map((line, i) => (
            <p key={i} className={`text-[11px] leading-5 ${i === log.length - 1 ? 'text-text' : 'text-muted'}`}>
              {line}
            </p>
          ))}
        </div>
      )}

      {/* Results */}
      {results && (
        <div className="bg-surface border border-border rounded-xl p-4">
          <div className="flex items-center justify-between mb-4">
            <p className="text-[10px] font-semibold text-muted uppercase tracking-widest">Results</p>
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-1.5 text-[10px] text-muted">
                <span className="w-2 h-2 rounded-full bg-green-400/70 inline-block" />&lt;{MID_SLA}ms
                <span className="w-2 h-2 rounded-full bg-yellow-400/70 inline-block ml-2" />&lt;{HIGH_SLA}ms
                <span className="w-2 h-2 rounded-full bg-red-400/70 inline-block ml-2" />slow
              </div>
              {elapsed && <span className="text-[10px] text-muted">{elapsed}s total</span>}
            </div>
          </div>

          {/* column headers */}
          <div className="flex items-center gap-3 mb-1 pr-1">
            <div className="flex-1" />
            <div className="flex gap-4 shrink-0">
              {['p50', 'p95', 'p99', 'avg'].map(h => (
                <div key={h} className="w-10 text-center text-[10px] font-semibold text-muted uppercase tracking-wide">{h}</div>
              ))}
            </div>
          </div>

          {results.map(row => (
            <ResultRow key={row.label} row={row} maxP99={maxP99} />
          ))}
        </div>
      )}
    </div>
  )
}
