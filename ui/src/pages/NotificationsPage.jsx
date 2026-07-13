import { useEffect, useState } from 'react'
import { Heart, MessageCircle, RefreshCw } from 'lucide-react'
import { Avatar } from '../components/Avatar'
import { api } from '../api'
import { useStore } from '../store'
import { timeAgo, shortId } from '../utils'

const TYPE_ICON = {
  like: <Heart size={13} className="text-red-400" fill="currentColor" />,
  comment: <MessageCircle size={13} className="text-blue-400" />,
}

const TYPE_LABEL = {
  like: 'liked your post',
  comment: 'commented on your post',
}

export function NotificationsPage() {
  const { currentUser, notifications, setNotifications, usersById, toast } = useStore()
  const [loading, setLoading] = useState(false)

  async function load() {
    if (!currentUser) return
    setLoading(true)
    try {
      const data = await api.getNotifications(currentUser.id)
      setNotifications(data?.notifications || [])
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [currentUser?.id])

  const username = (id) => usersById[id]?.username || shortId(id)

  return (
    <div>
      <div className="sticky top-0 z-10 bg-bg/80 backdrop-blur-sm border-b border-border px-4 py-3 flex items-center justify-between">
        <h1 className="text-sm font-semibold">Notifications</h1>
        {currentUser && (
          <button
            onClick={load}
            disabled={loading}
            className="text-muted hover:text-text transition-colors disabled:opacity-50"
          >
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          </button>
        )}
      </div>

      {!currentUser && (
        <div className="py-20 text-center text-muted text-sm px-6">
          Select a user to see their notifications.
        </div>
      )}

      {currentUser && notifications.length === 0 && !loading && (
        <div className="py-20 text-center text-muted text-sm">No notifications yet.</div>
      )}

      {notifications.map(n => (
        <div
          key={n.id}
          className={`px-4 py-4 border-b border-border flex gap-3 items-start transition-colors
            ${!n.read ? 'bg-accent/5' : ''}`}
        >
          <div className="relative shrink-0 mt-0.5">
            <Avatar username={username(n.actor_id)} size="sm" />
            <span className="absolute -bottom-0.5 -right-0.5 bg-bg rounded-full p-0.5">
              {TYPE_ICON[n.type]}
            </span>
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-sm">
              <span className="font-semibold">@{username(n.actor_id)}</span>{' '}
              <span className="text-muted">{TYPE_LABEL[n.type] || n.type}</span>
            </p>
            {n.entity_id && (
              <p className="text-xs text-muted/60 mt-0.5 font-mono truncate">{n.entity_id}</p>
            )}
            <p className="text-xs text-muted mt-1">{timeAgo(n.created_at)}</p>
          </div>
          {!n.read && (
            <div className="w-2 h-2 rounded-full bg-accent shrink-0 mt-2" />
          )}
        </div>
      ))}
    </div>
  )
}
