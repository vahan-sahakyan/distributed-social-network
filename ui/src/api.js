const BASE = '/api/v1'

async function req(method, path, body = null, isForm = false) {
  const url = BASE + path
  const opts = { method }
  if (body && !isForm) {
    opts.headers = { 'Content-Type': 'application/json' }
    opts.body = JSON.stringify(body)
  } else if (isForm) {
    opts.body = body
  }
  const res = await fetch(url, opts)
  if (res.status === 204) return null
  const data = await res.json().catch(() => null)
  if (!res.ok) throw new Error(data?.error || res.statusText)
  return data
}

export const api = {
  health: () => fetch('/health').then(r => r.json()),

  // Users
  createUser: (username, bio) => req('POST', '/users/', { username, bio }),
  getUser: (id) => req('GET', `/users/${id}`),
  getUserByUsername: (username) => req('GET', `/users/by-username/${encodeURIComponent(username)}`),
  followUser: (targetId, followerId) =>
    req('POST', `/users/${targetId}/follow`, { follower_id: followerId }),
  unfollowUser: (targetId, followerId) =>
    req('DELETE', `/users/${targetId}/follow`, { follower_id: followerId }),
  getFollowers: (id) => req('GET', `/users/${id}/followers`),
  getFollowing: (id) => req('GET', `/users/${id}/following`),

  // Posts
  createPost: (authorId, text, imageId) =>
    req('POST', '/posts/', { author_id: authorId, text, ...(imageId ? { image_id: imageId } : {}) }),
  getPost: (id) => req('GET', `/posts/${id}`),

  // Feed
  getHomeFeed: (userId) => req('GET', `/feed/home?user_id=${userId}`),
  getUserFeed: (userId) => req('GET', `/feed/user/${userId}`),

  // Comments
  createComment: (userId, entityId, text) =>
    req('POST', '/comments/', { user_id: userId, entity_id: entityId, text }),
  getComments: (entityId) => req('GET', `/comments/entity/${entityId}`),

  // Likes
  like: (userId, entityId) => req('POST', '/likes/', { user_id: userId, entity_id: entityId }),

  // Notifications
  getNotifications: (userId) => req('GET', `/notifications/${userId}`),

  // Media
  uploadMedia: (file) => {
    const fd = new FormData()
    fd.append('file', file)
    return req('POST', '/media/upload', fd, true)
  },
  getMedia: (id) => req('GET', `/media/${id}`),

  // System
  rebuildCache: () => req('POST', '/rebuild'),
}
