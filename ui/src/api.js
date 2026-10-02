import { bearer } from './auth'

const BASE = '/api/v1'

async function req(method, path, body = null, isForm = false) {
  const url = BASE + path
  const opts = { method, headers: {} }
  const token = await bearer()
  if (token) opts.headers.Authorization = `Bearer ${token}`
  if (body && !isForm) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  } else if (isForm) {
    opts.body = body
  }
  const res = await fetch(url, opts)
  if (res.status === 204) return null
  const data = await res.json().catch(() => null)
  if (!res.ok) {
    const err = new Error(data?.error || res.statusText)
    err.status = res.status
    throw err
  }
  return data
}

export const api = {
  health: () => fetch('/health').then(r => r.json()),

  // Users: writes act as the logged-in user
  me: () => req('GET', '/me'),
  createProfile: (bio) => req('POST', '/users/', { bio }),
  getUser: (id) => req('GET', `/users/${id}`),
  getUserByUsername: (username) => req('GET', `/users/by-username/${encodeURIComponent(username)}`),
  followUser: (targetId) => req('POST', `/users/${targetId}/follow`),
  unfollowUser: (targetId) => req('DELETE', `/users/${targetId}/follow`),
  getFollowers: (id) => req('GET', `/users/${id}/followers`),
  getFollowing: (id) => req('GET', `/users/${id}/following`),

  // Posts
  createPost: (text, imageId) =>
    req('POST', '/posts/', { text, ...(imageId ? { image_id: imageId } : {}) }),
  getPost: (id) => req('GET', `/posts/${id}`),

  // Feed
  getHomeFeed: () => req('GET', '/feed/home'),
  getUserFeed: (userId) => req('GET', `/feed/user/${userId}`),

  // Comments
  createComment: (entityId, text) => req('POST', '/comments/', { entity_id: entityId, text }),
  getComments: (entityId) => req('GET', `/comments/entity/${entityId}`),

  // Likes
  like: (entityId) => req('POST', '/likes/', { entity_id: entityId }),
  unlike: (entityId) => req('DELETE', '/likes/', { entity_id: entityId }),
  hasLiked: (entityId) => req('GET', `/likes/check?entity_id=${entityId}`).then(r => r.liked),

  // Notifications
  getNotifications: () => req('GET', '/notifications'),

  // Media
  uploadMedia: (file) => {
    const fd = new FormData()
    fd.append('file', file)
    return req('POST', '/media/upload', fd, true)
  },
  getMedia: (id) => req('GET', `/media/${id}`),

  // Search: public, eventually consistent with writes (about a second)
  searchPosts: (q, limit = 20) => req('GET', `/search/posts?q=${encodeURIComponent(q)}&limit=${limit}`),
  searchUsers: (q, limit = 20) => req('GET', `/search/users?q=${encodeURIComponent(q)}&limit=${limit}`),
  trendingHashtags: (hours = 24, limit = 10) => req('GET', `/search/hashtags/trending?hours=${hours}&limit=${limit}`),

  // System
  rebuildCache: () => req('POST', '/rebuild'),
  rebuildUserFeed: (userId) => req('POST', `/rebuild?user_id=${userId}`),
  resetAll: () => req('POST', '/reset'),
}
