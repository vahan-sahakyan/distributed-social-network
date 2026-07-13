import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export const useStore = create(
  persist(
    (set, get) => ({
      // persisted across sessions
      users: [],
      usersById: {},
      // session-only
      currentUser: null,
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
        set({ currentUser: user, feed: [], notifications: [] })
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
      // only persist user registry — session state is always fresh
      partialize: s => ({ users: s.users, usersById: s.usersById }),
    }
  )
)
