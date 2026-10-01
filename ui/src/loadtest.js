import { api } from './api'

// ─── stats helpers ────────────────────────────────────────────────────────────

function percentile(sorted, p) {
  if (!sorted.length) return 0
  const idx = Math.ceil((p / 100) * sorted.length) - 1
  return sorted[Math.max(0, idx)]
}

function calcStats(latencies) {
  if (!latencies.length) return { count: 0, ok: 0, errors: 0, min: 0, max: 0, avg: 0, p50: 0, p95: 0, p99: 0 }
  const ok = latencies.filter(l => l >= 0)
  const errors = latencies.length - ok.length
  const sorted = [...ok].sort((a, b) => a - b)
  const avg = ok.length ? Math.round(ok.reduce((s, v) => s + v, 0) / ok.length) : 0
  return {
    count:  latencies.length,
    ok:     ok.length,
    errors,
    min:    sorted[0]            ?? 0,
    max:    sorted[sorted.length - 1] ?? 0,
    avg,
    p50:    Math.round(percentile(sorted, 50)),
    p95:    Math.round(percentile(sorted, 95)),
    p99:    Math.round(percentile(sorted, 99)),
  }
}

/** Run `n` calls with `concurrency` at a time, record latency (ms) per call. */
async function bench(label, fn, n, concurrency) {
  const latencies = []
  let inflight = 0
  let started = 0

  await new Promise((resolve) => {
    const dispatch = () => {
      while (inflight < concurrency && started < n) {
        inflight++
        started++
        const t0 = performance.now()
        fn()
          .then(() => { latencies.push(Math.round(performance.now() - t0)) })
          .catch(() => { latencies.push(-1) })
          .finally(() => {
            inflight--
            if (latencies.length === n) resolve()
            else dispatch()
          })
      }
    }
    dispatch()
  })

  return { label, ...calcStats(latencies) }
}

// ─── scenarios ────────────────────────────────────────────────────────────────

/**
 * Run the load test suite.
 *
 * @param {{
 *   n?:           number   // requests per scenario (default 50)
 *   concurrency?: number   // parallel requests     (default 10)
 *   seedUserId?:  string   // existing user id for read tests
 *   seedPostId?:  string   // existing post id for read tests
 * }} config
 * @param {(msg: string) => void} onProgress
 * @returns {Promise<Array<{label, count, ok, errors, min, max, avg, p50, p95, p99}>>}
 */
export async function runLoadTest(config = {}, onProgress = () => {}) {
  const { n = 50, concurrency = 10 } = config
  let { seedUserId, seedPostId } = config

  const results = []

  // writes act as the logged-in user (seedUserId), so they need a session
  if (!seedPostId && seedUserId) {
    onProgress('Seeding a post for read tests…')
    try {
      const p = await api.createPost('Load test post – ignore me')
      seedPostId = p.id
    } catch { /* best effort */ }
  }

  // ── write scenarios ────────────────────────────────────────────────────────

  if (seedUserId && seedPostId) {
    onProgress(`POST /posts  (n=${n}, c=${concurrency})`)
    results.push(await bench(
      'POST /posts',
      () => api.createPost(`load test post ${Math.random()}`),
      n, concurrency,
    ))

    onProgress(`POST /likes  (n=${n}, c=${concurrency})`)
    results.push(await bench(
      'POST /likes',
      () => api.like(seedPostId),
      n, concurrency,
    ))

    onProgress(`POST /comments  (n=${n}, c=${concurrency})`)
    results.push(await bench(
      'POST /comments',
      () => api.createComment(seedPostId, `comment ${Math.random()}`),
      n, concurrency,
    ))
  }

  // ── read scenarios ─────────────────────────────────────────────────────────

  if (seedUserId) {
    onProgress(`GET /users/:id  (n=${n}, c=${concurrency})`)
    results.push(await bench(
      'GET /users/:id',
      () => api.getUser(seedUserId),
      n, concurrency,
    ))

    onProgress(`GET /feed/user/:id  (n=${n}, c=${concurrency})`)
    results.push(await bench(
      'GET /feed/user/:id',
      () => api.getUserFeed(seedUserId),
      n, concurrency,
    ))

    onProgress(`GET /notifications  (n=${n}, c=${concurrency})`)
    results.push(await bench(
      'GET /notifications',
      () => api.getNotifications(),
      n, concurrency,
    ))
  }

  if (seedPostId) {
    onProgress(`GET /posts/:id  (n=${n}, c=${concurrency})`)
    results.push(await bench(
      'GET /posts/:id',
      () => api.getPost(seedPostId),
      n, concurrency,
    ))

    onProgress(`GET /comments/entity/:id  (n=${n}, c=${concurrency})`)
    results.push(await bench(
      'GET /comments/entity/:id',
      () => api.getComments(seedPostId),
      n, concurrency,
    ))
  }

  onProgress('Done!')
  return results
}
