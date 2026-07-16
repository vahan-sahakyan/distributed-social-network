import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Heart, MessageCircle } from 'lucide-react'
import { Avatar } from './Avatar'
import { CommentSection } from './CommentSection'
import { api } from '../api'
import { useStore } from '../store'
import { timeAgo, shortId } from '../utils'

export function PostCard({ post }) {
  const navigate = useNavigate()
  const { currentUser, usersById, toast } = useStore()
  const [liked, setLiked] = useState(false)
  const [likes, setLikes] = useState(post.likesCount)
  const [showComments, setShowComments] = useState(false)

  const author = usersById[post.authorId]
  const username = author?.username || shortId(post.authorId)

  useEffect(() => {
    if (!currentUser) return
    api.hasLiked(currentUser.id, post.id)
      .then(v => setLiked(v))
      .catch(() => {})
  }, [currentUser?.id, post.id])

  async function handleLike() {
    if (!currentUser) return toast('Select a user first', 'error')
    if (liked) return
    setLiked(true)
    setLikes(l => l + 1)
    try {
      await api.like(currentUser.id, post.id)
    } catch (err) {
      setLiked(false)
      setLikes(l => l - 1)
      toast(err.message, 'error')
    }
  }

  return (
    <article className="border-b border-border hover:bg-surface/40 transition-colors cursor-default">
      <div className="px-4 pt-4 pb-3 flex gap-3">
        <button
          onClick={() => navigate('profile', post.authorId)}
          className="shrink-0"
        >
          <Avatar username={username} size="sm" />
        </button>

        <div className="flex-1 min-w-0">
          <div className="flex items-baseline gap-2 mb-1.5">
            <button
              onClick={() => navigate('/profile/' + post.authorId)}
              className="text-sm font-semibold text-text hover:underline"
            >
              @{username}
            </button>
            <span className="text-xs text-muted">{timeAgo(post.createdAt)}</span>
          </div>

          <p className="text-sm leading-relaxed text-text/90 whitespace-pre-wrap break-words">
            {post.text}
          </p>

          {post.imageId && <PostImage imageId={post.imageId} />}

          <div className="flex items-center gap-5 mt-3">
            <button
              onClick={handleLike}
              className={`flex items-center gap-1.5 text-xs transition-colors group ${
                liked ? 'text-red-400' : 'text-muted hover:text-red-400'
              }`}
            >
              <Heart
                size={14}
                fill={liked ? 'currentColor' : 'none'}
                className="transition-transform group-active:scale-110"
              />
              <span>{likes}</span>
            </button>

            <button
              onClick={() => setShowComments(v => !v)}
              className={`flex items-center gap-1.5 text-xs transition-colors ${
                showComments ? 'text-accent' : 'text-muted hover:text-accent'
              }`}
            >
              <MessageCircle size={14} />
              <span>{post.commentsCount}</span>
            </button>
          </div>
        </div>
      </div>

      {showComments && <CommentSection postId={post.id} />}
    </article>
  )
}

function PostImage({ imageId }) {
  const [src, setSrc] = useState(null)

  useEffect(() => {
    api.getMedia(imageId)
      .then(m => { if (m?.url) setSrc(`http://localhost:8080${m.url}`) })
      .catch(() => {})
  }, [imageId])

  if (!src) return null
  return (
    <img
      src={src}
      alt=""
      className="mt-2 rounded-xl max-h-72 object-cover w-full border border-border"
    />
  )
}
