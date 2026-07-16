import { useState, useEffect } from 'react'
import { Send } from 'lucide-react'
import { Avatar } from './Avatar'
import { api } from '../api'
import { useStore } from '../store'
import { timeAgo, shortId, parseTimestamp } from '../utils'

export function CommentSection({ postId }) {
  const { currentUser, usersById, toast } = useStore()
  const [comments, setComments] = useState([])
  const [text, setText] = useState('')
  const [loading, setLoading] = useState(true)
  const [posting, setPosting] = useState(false)

  useEffect(() => {
    api.getComments(postId)
      .then(data => setComments(Array.isArray(data) ? data : []))
      .catch(() => setComments([]))
      .finally(() => setLoading(false))
  }, [postId])

  async function handleComment() {
    if (!text.trim() || !currentUser || posting) return
    setPosting(true)
    try {
      const c = await api.createComment(currentUser.id, postId, text.trim())
      setComments(prev => [...prev, c])
      setText('')
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setPosting(false)
    }
  }

  const username = (userId) => usersById[userId]?.username || shortId(userId)

  return (
    <div className="border-t border-border px-4 pt-3 pb-4 space-y-3 bg-surface/30">
      {loading && <p className="text-xs text-muted">Loading…</p>}

      {!loading && comments.length === 0 && (
        <p className="text-xs text-muted/70">No comments yet.</p>
      )}

      {comments.map(c => (
        <div key={c.id} className="flex gap-2.5">
          <Avatar username={username(c.user_id)} size="xs" />
          <div className="flex-1 min-w-0">
            <div className="flex items-baseline gap-1.5">
              <span className="text-xs font-semibold text-text">@{username(c.user_id)}</span>
              <span className="text-xs text-muted">{timeAgo(parseTimestamp(c.created_at))}</span>
            </div>
            <p className="text-xs text-text/90 mt-0.5 leading-relaxed">{c.text}</p>
          </div>
        </div>
      ))}

      {currentUser && (
        <div className="flex gap-2 pt-1">
          <Avatar username={currentUser.username} size="xs" />
          <div className="flex flex-1 gap-2">
            <input
              value={text}
              onChange={e => setText(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') handleComment() }}
              placeholder="Add a comment…"
              className="flex-1 bg-bg border border-border rounded-full text-xs px-3 py-1.5 text-text placeholder-muted outline-none focus:border-border-hover transition-colors"
            />
            <button
              onClick={handleComment}
              disabled={!text.trim() || posting}
              className="text-accent disabled:text-muted hover:text-accent-hover transition-colors shrink-0"
            >
              <Send size={14} />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
