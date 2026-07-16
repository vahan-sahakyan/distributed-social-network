import { useEffect, useState } from 'react'
import { RefreshCw, Play } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { PostComposer } from '../components/PostComposer'
import { PostCard } from '../components/PostCard'
import { api } from '../api'
import { useStore } from '../store'
import { normalizePost } from '../utils'
import { runDemo } from '../demo'

export function HomePage() {
  const navigate = useNavigate()
  const { currentUser, feed, setFeed, addUser, setCurrentUser, toast } = useStore()
  const [loading, setLoading] = useState(false)
  const [demoRunning, setDemoRunning] = useState(false)

  async function loadFeed() {
    if (!currentUser) return
    setLoading(true)
    try {
      const data = await api.getHomeFeed(currentUser.id)
      setFeed((Array.isArray(data) ? data : []).map(normalizePost))
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setLoading(false)
    }
  }

  async function handleDemo() {
    setDemoRunning(true)
    try {
      const { users, loginAs } = await runDemo(() => {})
      Object.values(users).forEach(u => addUser(u))
      if (loginAs) {
        setCurrentUser(loginAs)
        toast('Demo ready! Feed loaded as @' + loginAs.username)
      }
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setDemoRunning(false)
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
          <p className="text-muted text-sm">Select or create a user in the sidebar to get started.</p>
        </div>
      )}

      {currentUser && feed.length === 0 && !loading && (
        <div className="py-16 text-center px-8 space-y-5">
          <p className="text-muted text-sm">Your feed is empty.</p>
          <p className="text-xs text-muted/60 max-w-xs mx-auto">
            Follow some users and create posts, then hit <strong className="text-muted">Rebuild Cache</strong> in the right panel — or just run the demo.
          </p>
          <button
            onClick={handleDemo}
            disabled={demoRunning}
            className="inline-flex items-center gap-2 bg-accent hover:bg-accent-hover disabled:opacity-50 text-white text-sm font-semibold px-5 py-2.5 rounded-full transition-colors"
          >
            <Play size={13} fill="white" />
            {demoRunning ? 'Setting up…' : 'Run Demo Setup'}
          </button>
        </div>
      )}

      {feed.map(post => (
        <PostCard key={post.id} post={post} />
      ))}
    </div>
  )
}
