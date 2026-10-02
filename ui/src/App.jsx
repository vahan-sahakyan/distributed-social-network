import { useState, useEffect } from 'react'
import { NavLink, Routes, Route, Navigate, useNavigate, useLocation } from 'react-router-dom'
import { Zap, Home, Bell, User, RefreshCw, LogIn, LogOut, Trash2, BarChart2, Search } from 'lucide-react'
import { Avatar } from './components/Avatar'
import { Toasts } from './components/Toast'
import { HomePage } from './pages/HomePage'
import { ProfilePage } from './pages/ProfilePage'
import { NotificationsPage } from './pages/NotificationsPage'
import { AuthPage } from './pages/AuthPage'
import { LoadTestPage } from './pages/LoadTestPage'
import { SearchPage } from './pages/SearchPage'
import { TrendingHashtags } from './components/TrendingHashtags'
import { api } from './api'
import { keycloak, logout } from './auth'
import { useStore } from './store'

function RequireUser({ children }) {
  const { currentUser, needsProfile } = useStore()
  if (needsProfile) return <Navigate to="/login" replace />
  if (!currentUser) return keycloak.authenticated ? null : <Navigate to="/login" replace />
  return children
}

function AppLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const { currentUser, users, notifications, setCurrentUser, setNeedsProfile, toast, clearUsers } = useStore()
  const [health, setHealth] = useState(null)
  const [rebuilding, setRebuilding] = useState(false)

  useEffect(() => {
    api.health().then(() => setHealth(true)).catch(() => setHealth(false))
  }, [])

  // the profile behind the login session; none yet means the sign-up isn't finished
  useEffect(() => {
    if (!keycloak.authenticated) return
    api.me()
      .then(setCurrentUser)
      .catch(err => {
        if (err.status === 404) {
          setNeedsProfile(true)
          navigate('/login')
        } else {
          toast(err.message, 'error')
        }
      })
  }, [])

  async function handleRebuild() {
    setRebuilding(true)
    try {
      await api.rebuildCache()
      toast('Cache rebuilt successfully!')
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setRebuilding(false)
    }
  }

  const unread = notifications.filter(n => !n.read).length
  const otherUsers = users.filter(u => u.id !== currentUser?.id)

  const navLinkClass = ({ isActive }) =>
    'w-full flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium transition-colors ' +
    (isActive ? 'bg-surface text-text' : 'text-muted hover:text-text hover:bg-surface/50')

  return (
    <div className="min-h-screen flex justify-center">
      <div className="w-full max-w-5xl flex">

        {/* Left Sidebar */}
        <aside className="w-56 shrink-0 border-r border-border flex flex-col sticky top-0 h-screen overflow-y-auto">
          <div className="p-4 flex flex-col gap-1 h-full">
            {/* Logo */}
            <div className="flex items-center gap-2.5 px-2 py-3 mb-2">
              <div className="w-7 h-7 bg-accent rounded-lg flex items-center justify-center shrink-0">
                <Zap size={13} className="text-white" />
              </div>
              <span className="text-sm font-bold">SocialNet</span>
              <span className={'w-1.5 h-1.5 rounded-full ml-auto shrink-0 ' + (health === true ? 'bg-green-400' : health === false ? 'bg-red-400' : 'bg-neutral-600')} />
            </div>

            {/* Nav */}
            <nav className="space-y-0.5 mb-5">
              <NavLink to="/home" className={navLinkClass}>
                {({ isActive }) => <><Home size={15} />Home</>}
              </NavLink>
              <NavLink to="/search" className={navLinkClass}>
                {() => <><Search size={15} />Search</>}
              </NavLink>
              <NavLink to="/notifications" className={navLinkClass}>
                {({ isActive }) => (
                  <>
                    <Bell size={15} />
                    Notifications
                    {unread > 0 && (
                      <span className="ml-auto bg-accent text-white text-[10px] font-bold px-1.5 py-0.5 rounded-full min-w-[18px] text-center">
                        {unread}
                      </span>
                    )}
                  </>
                )}
              </NavLink>
              {currentUser && (
                <NavLink to={'/profile/' + currentUser.id} className={navLinkClass}>
                  {() => <><User size={15} />My Profile</>}
                </NavLink>
              )}
              <NavLink to="/loadtest" className={navLinkClass}>
                {() => <><BarChart2 size={15} />Load Test</>}
              </NavLink>
            </nav>

            <div className="flex-1" />

            {currentUser && (
              <div className="pt-3 border-t border-border mt-2 space-y-1">
                <div className="flex items-center gap-2.5 px-2 py-1.5">
                  <Avatar username={currentUser.username} size="xs" />
                  <p className="text-xs font-medium text-text truncate flex-1">@{currentUser.username}</p>
                </div>
                <button
                  onClick={() => { clearUsers(); logout() }}
                  className="w-full flex items-center gap-2 px-2 py-1.5 text-xs text-muted hover:text-text transition-colors rounded-lg hover:bg-surface/40"
                >
                  <LogOut size={12} />
                  Log out
                </button>
              </div>
            )}
          </div>
        </aside>

        {/* Main Content */}
        <main className="flex-1 min-w-0 border-r border-border">
          <Routes>
            <Route path="/" element={<Navigate to="/home" replace />} />
            <Route path="/login" element={<AuthPage />} />
            <Route path="/home" element={<RequireUser><HomePage /></RequireUser>} />
            <Route path="/profile/:userId" element={<ProfilePage />} />
            <Route path="/notifications" element={<RequireUser><NotificationsPage /></RequireUser>} />
            <Route path="/loadtest" element={<LoadTestPage />} />
            <Route path="/search" element={<SearchPage />} />
          </Routes>
        </main>

        {/* Right Sidebar */}
        <aside className="w-72 shrink-0 hidden lg:flex flex-col p-4 gap-4 sticky top-0 h-screen overflow-y-auto">
          {currentUser ? (
            <div className="bg-surface border border-border rounded-xl p-4">
              <p className="text-[10px] font-semibold text-muted uppercase tracking-widest mb-3">Active User</p>
              <div className="flex items-center gap-3 mb-3">
                <Avatar username={currentUser.username} size="md" />
                <div className="min-w-0">
                  <p className="text-sm font-semibold text-text truncate">@{currentUser.username}</p>
                  {currentUser.bio && <p className="text-xs text-muted mt-0.5 line-clamp-2">{currentUser.bio}</p>}
                </div>
              </div>
              <NavLink
                to={'/profile/' + currentUser.id}
                className="block w-full text-center text-xs text-muted hover:text-text border border-border hover:border-border-hover rounded-lg px-3 py-1.5 transition-colors"
              >
                View Profile
              </NavLink>
            </div>
          ) : (
            <div className="bg-surface border border-border rounded-xl p-5 text-center">
              <LogIn size={20} className="text-muted mx-auto mb-2" />
              <p className="text-xs font-medium text-muted">Not logged in</p>
              <NavLink to="/login" className="block text-xs text-accent hover:underline mt-1">Sign up or log in</NavLink>
            </div>
          )}

          <TrendingHashtags />

          {otherUsers.length > 0 && (
            <div className="bg-surface border border-border rounded-xl p-4">
              <p className="text-[10px] font-semibold text-muted uppercase tracking-widest mb-3">People</p>
              <div className="space-y-3">
                {otherUsers.slice(0, 6).map(u => (
                  <div key={u.id} className="flex items-center gap-2.5">
                    <NavLink to={'/profile/' + u.id} className="shrink-0"><Avatar username={u.username} size="xs" /></NavLink>
                    <NavLink to={'/profile/' + u.id} className="flex-1 min-w-0 text-left block">
                      <p className="text-xs font-medium text-text truncate">@{u.username}</p>
                      {u.bio && <p className="text-xs text-muted truncate">{u.bio}</p>}
                    </NavLink>
                    <NavLink to={'/profile/' + u.id} className="text-xs text-accent hover:text-accent-hover font-medium shrink-0 transition-colors">
                      View
                    </NavLink>
                  </div>
                ))}
              </div>
            </div>
          )}

          <div className="bg-surface border border-border rounded-xl p-4">
            <p className="text-[10px] font-semibold text-muted uppercase tracking-widest mb-3">System</p>
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs text-muted">Gateway</span>
                <span className={'text-xs font-medium ' + (health === true ? 'text-green-400' : health === false ? 'text-red-400' : 'text-neutral-500')}>
                  {health === true ? '● Online' : health === false ? '● Offline' : '○ Checking…'}
                </span>
              </div>
              {currentUser && <button
                onClick={handleRebuild}
                disabled={rebuilding}
                className="w-full flex items-center justify-center gap-2 text-xs text-muted hover:text-text border border-border hover:border-border-hover rounded-lg px-3 py-2 transition-colors disabled:opacity-50"
              >
                <RefreshCw size={11} className={rebuilding ? 'animate-spin' : ''} />
                {rebuilding ? 'Rebuilding…' : 'Rebuild Feed Cache'}
              </button>}
              <button
                onClick={async () => {
                  try {
                    await api.resetAll()
                  } catch (err) {
                    toast(err.message, 'error')
                    return
                  }
                  // accounts live in Keycloak and survive; only the profile has to be recreated
                  clearUsers()
                  setNeedsProfile(keycloak.authenticated)
                  navigate('/login')
                  toast('All data wiped.')
                }}
                className="w-full flex items-center justify-center gap-2 text-xs text-red-400/60 hover:text-red-400 border border-transparent hover:border-red-900/40 rounded-lg px-3 py-2 transition-colors"
              >
                <Trash2 size={11} />
                Reset everything
              </button>
            </div>
          </div>
        </aside>
      </div>
      <Toasts />
    </div>
  )
}

export default function App() {
  return <AppLayout />
}
