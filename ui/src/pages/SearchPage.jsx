import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Search } from 'lucide-react'
import { Avatar } from '../components/Avatar'
import { Highlight, RichText } from '../components/RichText'
import { api } from '../api'
import { useStore } from '../store'
import { timeAgo, parseTimestamp, shortId } from '../utils'

const TABS = [
  { id: 'posts', label: 'Posts' },
  { id: 'people', label: 'People' },
]

function PostHit({ hit }) {
  const { usersById, ensureUser } = useStore()
  useEffect(() => { ensureUser(hit.author_id) }, [hit.author_id, ensureUser])
  const username = usersById[hit.author_id]?.username || shortId(hit.author_id)

  return (
    <div className="px-4 py-3 border-b border-border flex gap-3">
      <Link to={'/profile/' + hit.author_id} className="shrink-0">
        <Avatar username={username} size="sm" />
      </Link>
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline gap-2 mb-1">
          <Link to={'/profile/' + hit.author_id} className="text-sm font-semibold text-text hover:underline">
            @{username}
          </Link>
          <span className="text-xs text-muted">{timeAgo(parseTimestamp(hit.created_at))}</span>
        </div>
        <p className="text-sm leading-relaxed text-text/90 whitespace-pre-wrap break-words">
          {hit.highlight ? <Highlight html={hit.highlight} /> : <RichText text={hit.text} />}
        </p>
      </div>
    </div>
  )
}

function UserHit({ hit }) {
  const { addUser } = useStore()
  useEffect(() => { addUser({ id: hit.id, username: hit.username, bio: hit.bio }) }, [hit, addUser])

  return (
    <Link to={'/profile/' + hit.id} className="px-4 py-3 border-b border-border flex items-center gap-3 hover:bg-surface/40 transition-colors">
      <Avatar username={hit.username} size="sm" />
      <div className="min-w-0">
        <p className="text-sm font-semibold text-text">@{hit.username}</p>
        {hit.bio && <p className="text-xs text-muted truncate">{hit.bio}</p>}
      </div>
    </Link>
  )
}

export function SearchPage() {
  const [params, setParams] = useSearchParams()
  const query = params.get('q') || ''
  const tab = params.get('tab') === 'people' ? 'people' : 'posts'
  const [input, setInput] = useState(query)
  const [result, setResult] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)

  // the box follows the URL, so hashtag links and back/forward fill it in
  useEffect(() => { setInput(query) }, [query])

  // typing updates the URL after a short pause
  useEffect(() => {
    if (input === query) return
    const timer = setTimeout(() => setParams(p => {
      const next = new URLSearchParams(p)
      if (input.trim()) next.set('q', input)
      else next.delete('q')
      return next
    }, { replace: true }), 300)
    return () => clearTimeout(timer)
  }, [input, query, setParams])

  useEffect(() => {
    if (!query.trim()) {
      setResult(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    const search = tab === 'people' ? api.searchUsers(query) : api.searchPosts(query)
    search
      .then(d => { if (!cancelled) setResult(d) })
      .catch(err => { if (!cancelled) setError(err.message) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [query, tab])

  const hits = (tab === 'people' ? result?.users : result?.posts) || []

  return (
    <div>
      <div className="sticky top-0 z-10 bg-bg/80 backdrop-blur-sm border-b border-border">
        <div className="px-4 pt-3 pb-2">
          <div className="flex items-center gap-2 bg-surface border border-border rounded-full px-3 py-2 focus-within:border-border-hover">
            <Search size={14} className="text-muted shrink-0" />
            <input
              autoFocus
              value={input}
              onChange={e => setInput(e.target.value)}
              placeholder={tab === 'people' ? 'Search people by username or bio' : 'Search posts, or #hashtag'}
              className="flex-1 bg-transparent text-sm text-text placeholder-muted outline-none"
            />
          </div>
        </div>
        <div className="flex">
          {TABS.map(t => (
            <button
              key={t.id}
              onClick={() => setParams(p => { const next = new URLSearchParams(p); next.set('tab', t.id); return next }, { replace: true })}
              className={'flex-1 py-2.5 text-sm font-medium transition-colors ' +
                (tab === t.id ? 'text-text border-b-2 border-accent' : 'text-muted hover:text-text')}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {!query.trim() && (
        <p className="py-16 text-center text-sm text-muted">Search posts by words or #hashtag, or find people.</p>
      )}
      {error && <p className="py-10 text-center text-sm text-red-400">{error}</p>}
      {query.trim() && !error && result && (
        <>
          <p className="px-4 py-2 text-xs text-muted border-b border-border">
            {result.total ?? 0} {tab === 'people' ? 'people' : 'posts'}{loading ? ' …' : ''}
          </p>
          {hits.length === 0 && <p className="py-12 text-center text-sm text-muted">No matches.</p>}
          {tab === 'people'
            ? hits.map(h => <UserHit key={h.id} hit={h} />)
            : hits.map(h => <PostHit key={h.id} hit={h} />)}
        </>
      )}
    </div>
  )
}
