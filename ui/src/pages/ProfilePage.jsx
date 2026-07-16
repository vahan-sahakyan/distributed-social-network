import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { UserPlus, UserCheck, ArrowLeft } from 'lucide-react'
import { PostCard } from '../components/PostCard'
import { Avatar } from '../components/Avatar'
import { api } from '../api'
import { useStore } from '../store'
import { normalizePost, shortId } from '../utils'
export function ProfilePage() {
  const navigate = useNavigate()
  const { userId: profileUserId } = useParams()
  const { currentUser, usersById, addUser, setFeed, toast } = useStore()
  const [user, setUser] = useState(usersById[profileUserId] || null)
  const [posts, setPosts] = useState([])
  const [loading, setLoading] = useState(true)
  const [followers, setFollowers] = useState([])
  const [following, setFollowing] = useState([])
  const [followLoading, setFollowLoading] = useState(false)

  const isFollowing = followers.includes(currentUser?.id)
  const isSelf = currentUser?.id === profileUserId

  useEffect(() => {
    if (!profileUserId) return
    setLoading(true)
    setUser(null)
    setPosts([])

    Promise.all([
      api.getUser(profileUserId).then(u => { setUser(u); addUser(u) }),
      api.getUserFeed(profileUserId)
        .then(d => setPosts((Array.isArray(d) ? d : []).map(normalizePost)))
        .catch(() => setPosts([])),
      api.getFollowers(profileUserId)
        .then(d => setFollowers(d?.followers || []))
        .catch(() => setFollowers([])),
      api.getFollowing(profileUserId)
        .then(d => setFollowing(d?.following || []))
        .catch(() => setFollowing([])),
    ])
      .catch(err => toast(err.message, 'error'))
      .finally(() => setLoading(false))
  }, [profileUserId])

  async function handleFollow() {
    if (!currentUser) return toast('Select a user first', 'error')
    setFollowLoading(true)
    try {
      if (isFollowing) {
        await api.unfollowUser(profileUserId, currentUser.id)
        setFollowers(f => f.filter(id => id !== currentUser.id))
        toast(`Unfollowed @${user?.username}`)
        // rebuild only this user's feed then clear local state so home reloads
        api.rebuildUserFeed(currentUser.id).catch(() => {})
        setFeed([])
        // rebuild only this user's feed and clear local cache
        api.rebuildUserFeed(currentUser.id).catch(() => {})
        setFeed([])
      } else {
        await api.followUser(profileUserId, currentUser.id)
        setFollowers(f => [...f, currentUser.id])
        toast(`Now following @${user?.username}!`)
      }
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setFollowLoading(false)
    }
  }

  if (loading) {
    return (
      <div className="py-20 text-center text-muted text-sm">Loading profile…</div>
    )
  }

  if (!user) {
    return (
      <div className="py-20 text-center text-muted text-sm">User not found.</div>
    )
  }

  return (
    <div>
      <div className="sticky top-0 z-10 bg-bg/80 backdrop-blur-sm border-b border-border px-4 py-3 flex items-center gap-3">
        <button
          onClick={() => navigate(-1)}
          className="text-muted hover:text-text transition-colors"
        >
          <ArrowLeft size={16} />
        </button>
        <div>
          <h1 className="text-sm font-semibold leading-tight">@{user.username}</h1>
          <p className="text-xs text-muted">{posts.length} posts</p>
        </div>
      </div>

      {/* Profile header */}
      <div className="px-6 py-6 border-b border-border">
        <div className="flex items-start justify-between gap-4 mb-4">
          <Avatar username={user.username} size="lg" />
          {!isSelf && currentUser && (
            <button
              onClick={handleFollow}
              disabled={followLoading}
              className={`flex items-center gap-2 text-sm font-semibold px-5 py-2 rounded-full transition-colors shrink-0
                ${isFollowing
                  ? 'border border-border text-muted hover:border-red-800 hover:text-red-400'
                  : 'bg-text text-bg hover:bg-text/90'
                }`}
            >
              {isFollowing
                ? <><UserCheck size={14} /> Following</>
                : <><UserPlus size={14} /> Follow</>
              }
            </button>
          )}
        </div>

        <p className="font-semibold text-text text-base">@{user.username}</p>
        {user.bio && <p className="text-sm text-muted mt-1">{user.bio}</p>}
        <p className="text-xs text-muted/40 mt-1 font-mono">{user.id}</p>

        <div className="flex gap-5 mt-4">
          <span className="text-sm">
            <strong className="text-text font-semibold">{following.length}</strong>{' '}
            <span className="text-muted">Following</span>
          </span>
          <span className="text-sm">
            <strong className="text-text font-semibold">{followers.length}</strong>{' '}
            <span className="text-muted">Followers</span>
          </span>
        </div>
      </div>

      {posts.length === 0 && (
        <div className="py-16 text-center text-muted text-sm">No posts yet.</div>
      )}

      {posts.map(p => (
        <PostCard key={p.id} post={p} />
      ))}
    </div>
  )
}
