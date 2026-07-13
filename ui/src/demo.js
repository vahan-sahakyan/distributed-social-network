import { api } from './api'

const SEED_USERS = [
  { username: 'alice',   bio: 'Senior engineer @ DistributedSystems Co. • ScyllaDB fan' },
  { username: 'bob',     bio: 'Full-stack dev • coffee enthusiast ☕ • he/him' },
  { username: 'carol',   bio: 'Designer & open source contributor 🎨' },
  { username: 'dan',     bio: 'DevOps wizard • k8s all the things 🐳' },
]

const SEED_POSTS = [
  {
    author: 'alice',
    text: 'Just deployed our new distributed cache layer using ScyllaDB + Memcached. The fan-out-on-write pattern is incredibly fast for feed generation. Latency dropped 80% 🚀',
  },
  {
    author: 'alice',
    text: 'Hot take: microservices are only worth the complexity if your team has clear ownership boundaries. Otherwise just use a well-structured monolith and sleep at night.',
  },
  {
    author: 'bob',
    text: 'Finally got Redpanda set up for our event streaming pipeline. First broker startup took under 2 seconds. Kafka who? 😄',
  },
  {
    author: 'carol',
    text: 'Redesigning our notification system UI. The challenge: showing just enough context without overwhelming the user. Less really is more.',
  },
  {
    author: 'dan',
    text: 'PSA: Always put health checks in Kubernetes readiness probes, not liveness probes. Got paged at 3am because of this last week 😅 Learn from my pain.',
  },
  {
    author: 'alice',
    text: 'Benchmark result that surprised me: Memcached at 500k ops/sec vs Redis at 450k ops/sec for pure get/set. For simple cache workloads Memcached still wins on throughput.',
  },
  {
    author: 'bob',
    text: 'Reminder that ClickHouse can ingest millions of rows per second and still serve analytical queries in milliseconds. Absolute unit of a database.',
  },
]

const SEED_COMMENTS = [
  { author: 'bob',   postIdx: 0, text: 'What keyspace strategy are you using in ScyllaDB? Curious about the partition layout.' },
  { author: 'carol', postIdx: 0, text: 'The fan-out approach is so clean. Did you evaluate fan-out on read as a fallback for power users?' },
  { author: 'dan',   postIdx: 2, text: 'Agreed, Redpanda is a game-changer. We replaced Kafka last quarter, zero regrets.' },
  { author: 'alice', postIdx: 2, text: 'The single-binary deployment is what sold me honestly 😂' },
  { author: 'alice', postIdx: 4, text: 'Ouch, been there. We now have a separate /ready endpoint that checks downstream deps.' },
  { author: 'bob',   postIdx: 3, text: 'Progressive disclosure is the answer here. Start minimal, let the user expand.' },
]

// bob & carol & dan follow alice; carol & dan follow bob
const SEED_FOLLOWS = [
  ['alice', 'bob'],
  ['alice', 'carol'],
  ['alice', 'dan'],
  ['bob',   'carol'],
  ['bob',   'dan'],
]

const SEED_LIKES = [
  ['bob',   0],
  ['carol', 0],
  ['dan',   1],
  ['carol', 2],
  ['dan',   2],
  ['alice', 4],
  ['bob',   4],
]

/**
 * Run the full demo seed.
 * @param {(step: string, done?: boolean) => void} onStep  progress callback
 * @returns {{ users, loginAs }} users map + suggested login user
 */
export async function runDemo(onStep) {
  const users = {}

  onStep('Creating users…')
  for (const u of SEED_USERS) {
    try {
      const user = await api.createUser(u.username, u.bio)
      users[u.username] = user
    } catch {
      // already exists — look up by username
      try {
        const existing = await api.getUserByUsername(u.username)
        users[u.username] = existing
      } catch {
        // skip
      }
    }
  }

  onStep('Setting up follows…')
  for (const [target, follower] of SEED_FOLLOWS) {
    if (users[target] && users[follower]) {
      await api.followUser(users[target].id, users[follower].id).catch(() => {})
    }
  }

  onStep('Publishing posts…')
  const posts = []
  for (const p of SEED_POSTS) {
    if (!users[p.author]) continue
    try {
      const post = await api.createPost(users[p.author].id, p.text)
      posts.push(post)
    } catch {
      posts.push(null)
    }
  }

  onStep('Adding likes & comments…')
  for (const [liker, idx] of SEED_LIKES) {
    if (users[liker] && posts[idx]) {
      await api.like(users[liker].id, posts[idx].id).catch(() => {})
    }
  }
  for (const c of SEED_COMMENTS) {
    if (users[c.author] && posts[c.postIdx]) {
      await api.createComment(users[c.author].id, posts[c.postIdx].id, c.text).catch(() => {})
    }
  }

  onStep('Rebuilding feed cache…')
  await api.rebuildCache().catch(() => {})

  onStep('Done!')
  // Log in as bob — follows alice, has his own posts, has notifications
  return { users, loginAs: users['bob'] ?? Object.values(users)[0] }
}
