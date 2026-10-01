import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { api } from './api'

// profile lookups in flight, so a feed of one author's posts fetches it once
const loading = new Set()

export const useStore = create(
  persist(
    (set, get) => ({
      // persisted across sessions: profiles seen, for names and the People list
      users: [],
      usersById: {},
      // session-only; set from the login session via /me
      currentUser: null,
      // logged in to Keycloak but no profile created yet
      needsProfile: false,
      feed: [],
      notifications: [],
      toasts: [],

      addUser(user) {
        if (!user?.id) return
        set(s => {
          if (s.usersById[user.id]) return {}
          return {
            users: [...s.users, user],
            usersById: { ...s.usersById, [user.id]: user },
          }
        })
      },

      setCurrentUser(user) {
        if (user) get().addUser(user)
        // the same user again (e.g. a repeated /me) keeps the loaded feed
        if (user?.id && user.id === get().currentUser?.id) {
          set({ currentUser: user, needsProfile: false })
          return
        }
        set({ currentUser: user, needsProfile: false, feed: [], notifications: [] })
      },

      setNeedsProfile(needsProfile) { set({ needsProfile }) },

      /** Fetch and cache a profile not seen yet, for showing usernames. */
      async ensureUser(id) {
        if (!id || get().usersById[id] || loading.has(id)) return
        loading.add(id)
        try {
          get().addUser(await api.getUser(id))
        } catch { /* unknown or deleted: keep the short id */ }
        finally {
          loading.delete(id)
        }
      },

      setFeed(feed) { set({ feed }) },
      setNotifications(notifications) { set({ notifications }) },

      toast(message, type = 'success') {
        const id = Date.now() + Math.random()
        set(s => ({ toasts: [...s.toasts, { id, message, type }] }))
        setTimeout(() => set(s => ({ toasts: s.toasts.filter(t => t.id !== id) })), 3500)
      },

      removeToast(id) {
        set(s => ({ toasts: s.toasts.filter(t => t.id !== id) }))
      },

      /** Remove a single user from the registry (e.g. stale/deleted). */
      removeUser(id) {
        set(s => {
          const { [id]: _, ...usersById } = s.usersById
          return {
            users: s.users.filter(u => u.id !== id),
            usersById,
            currentUser: s.currentUser?.id === id ? null : s.currentUser,
          }
        })
      },

      /** Wipe the entire persisted user registry. */
      clearUsers() {
        set({ users: [], usersById: {}, currentUser: null, feed: [], notifications: [] })
      },
    }),
    {
      name: 'socialnet-store',
      partialize: s => ({ users: s.users, usersById: s.usersById }),
    }
  )
)
