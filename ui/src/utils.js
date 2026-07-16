export function timeAgo(dateStr) {
  if (!dateStr) return ''
  const diff = (Date.now() - new Date(dateStr)) / 1000
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`
  if (diff < 604800) return `${Math.floor(diff / 86400)}d`
  return new Date(dateStr).toLocaleDateString()
}

const COLORS = [
  '#3b82f6', '#10b981', '#8b5cf6', '#f59e0b',
  '#ec4899', '#06b6d4', '#ef4444', '#84cc16',
]

export function avatarColor(str = '') {
  let h = 0
  for (let i = 0; i < str.length; i++) h = str.charCodeAt(i) + ((h << 5) - h)
  return COLORS[Math.abs(h) % COLORS.length]
}

export function initials(name = '') {
  return (name || '?').slice(0, 2).toUpperCase()
}

export function parseTimestamp(ts) {
  if (!ts) return null
  // protobuf Timestamp: { seconds: number, nanos: number }
  if (typeof ts === 'object' && 'seconds' in ts) {
    return new Date(ts.seconds * 1000 + (ts.nanos ?? 0) / 1e6).toISOString()
  }
  return ts
}

export function normalizePost(p) {
  return {
    id: p.post_id || p.id,
    authorId: p.author_id,
    text: p.text,
    imageId: p.image_id || null,
    likesCount: p.likes_count ?? p.likes ?? 0,
    commentsCount: p.comments_count ?? p.comments ?? 0,
    createdAt: parseTimestamp(p.created_at),
  }
}

export function shortId(id = '') {
  return id.slice(0, 8) + '…'
}
