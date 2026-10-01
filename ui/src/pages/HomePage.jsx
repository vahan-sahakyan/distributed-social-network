import { useEffect, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { PostComposer } from '../components/PostComposer'
import { PostCard } from '../components/PostCard'
import { api } from '../api'
import { useStore } from '../store'
import { normalizePost } from '../utils'

export function HomePage() {
  const { currentUser, feed, setFeed, toast } = useStore()
  const [loading, setLoading] = useState(false)

  async function loadFeed() {
    if (!currentUser) return
    setLoading(true)
    try {
      const data = await api.getHomeFeed()
      setFeed((Array.isArray(data) ? data : []).map(normalizePost))
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { loadFeed() }, [currentUser?.id])

  return (
    <div>
      <div className="sticky top-0 z-10 bg-bg/80 backdrop-blur-sm border-b border-border px-4 py-3 flex items-center justify-between">
        <h1 className="text-sm font-semibold">Home</h1>
        {currentUser && (
          <button
            onClick={loadFeed}
            disabled={loading}
            className="text-muted hover:text-text transition-colors disabled:opacity-50"
            title="Refresh feed"
          >
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          </button>
        )}
      </div>

      <PostComposer onPost={loadFeed} />

      {!currentUser && (
        <div className="py-20 text-center px-6">
          <p className="text-muted text-sm">Log in to see your feed.</p>
        </div>
      )}

      {currentUser && feed.length === 0 && !loading && (
        <div className="py-16 text-center px-8 space-y-5">
          <p className="text-muted text-sm">Your feed is empty.</p>
          <p className="text-xs text-muted/60 max-w-xs mx-auto">
            Follow some users and create posts, or run <code className="text-muted">make demo</code> and log in as alice, bob or charlie.
          </p>
        </div>
      )}

      {feed.map(post => (
        <PostCard key={post.id} post={post} />
      ))}
    </div>
  )
}
