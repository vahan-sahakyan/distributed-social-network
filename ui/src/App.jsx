import { useState, useEffect } from 'react'
import { NavLink, Routes, Route, Navigate, useNavigate, useLocation } from 'react-router-dom'
import { Zap, Home, Bell, User, Plus, RefreshCw, LogIn, Play } from 'lucide-react'
import { runDemo } from './demo'
import { Avatar } from './components/Avatar'
import { Toasts } from './components/Toast'
import { HomePage } from './pages/HomePage'
import { ProfilePage } from './pages/ProfilePage'
import { NotificationsPage } from './pages/NotificationsPage'
import { AuthPage } from './pages/AuthPage'
import { api } from './api'
import { useStore } from './store'

function RequireUser({ children }) {
  const { currentUser } = useStore()
  if (!currentUser) return <Navigate to="/auth" replace />
  return children
}

function AppLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const { currentUser, users, notifications, setCurrentUser, addUser, toast } = useStore()
  const [health, setHealth] = useState(null)
  const [showNewUser, setShowNewUser] = useState(false)
  const [newUsername, setNewUsername] = useState('')
  const [newBio, setNewBio] = useState('')
  const [creating, setCreating] = useState(false)
  const [rebuilding, setRebuilding] = useState(false)
  const [demoing, setDemoing] = useState(false)

  useEffect(() => {
    api.health().then(() => setHealth(true)).catch(() => setHealth(false))
  }, [])

  async function handleCreateUser() {
    if (!newUsername.trim() || creating) return
    setCreating(true)
    try {
      const user = await api.createUser(newUsername.trim(), newBio.trim())
      addUser(user)
      setCurrentUser(user)
      setShowNewUser(false)
      setNewUsername('')
      setNewBio('')
      toast('Created @' + user.username + '!')
      navigate('/home')
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setCreating(false)
    }
  }

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

  async function handleDemo() {
    setDemoing(true)
    try {
      const { users: demoUsers, loginAs } = await runDemo(() => {})
      Object.values(demoUsers).forEach(u => addUser(u))
      if (loginAs) {
        setCurrentUser(loginAs)
        toast('Demo ready! Logged in as @' + loginAs.username)
        navigate('/home')
      }
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setDemoing(false)
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
            </nav>

            {/* Users section */}
            <div className="flex items-center justify-between px-1 mb-2">
              <span className="text-[10px] font-semibold text-muted uppercase tracking-widest">Users</span>
              <button onClick={() => setShowNewUser(v => !v)} className="text-muted hover:text-text transition-colors" title="Create user">
                <Plus size={13} />
              </button>
            </div>

            {showNewUser && (
              <div className="mb-3 p-3 bg-surface border border-border rounded-xl space-y-2">
                <input
                  autoFocus
                  value={newUsername}
                  onChange={e => setNewUsername(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter') handleCreateUser(); if (e.key === 'Escape') { setShowNewUser(false); setNewUsername(''); setNewBio('') } }}
                  placeholder="Username"
                  className="w-full bg-bg border border-border rounded-lg px-3 py-1.5 text-xs text-text placeholder-muted outline-none focus:border-border-hover transition-colors"
                />
                <input
                  value={newBio}
                  onChange={e => setNewBio(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter') handleCreateUser() }}
                  placeholder="Bio (optional)"
                  className="w-full bg-bg border border-border rounded-lg px-3 py-1.5 text-xs text-text placeholder-muted outline-none focus:border-border-hover transition-colors"
                />
                <button
                  onClick={handleCreateUser}
                  disabled={!newUsername.trim() || creating}
                  className="w-full bg-accent hover:bg-accent-hover disabled:opacity-40 text-white text-xs font-semibold py-1.5 rounded-lg transition-colors"
                >
                  {creating ? 'Creating…' : 'Create User'}
                </button>
              </div>
            )}

            <div className="space-y-0.5 flex-1 overflow-y-auto min-h-0">
              {users.map(u => (
                <button
                  key={u.id}
                  onClick={() => setCurrentUser(u)}
                  className={'w-full flex items-center gap-2.5 px-2 py-1.5 rounded-lg text-left transition-colors group ' + (currentUser?.id === u.id ? 'bg-surface' : 'hover:bg-surface/40')}
                >
                  <Avatar username={u.username} size="xs" />
                  <p className={'text-xs font-medium truncate flex-1 ' + (currentUser?.id === u.id ? 'text-text' : 'text-muted group-hover:text-text')}>
                    @{u.username}
                  </p>
                  {currentUser?.id === u.id && <div className="w-1.5 h-1.5 rounded-full bg-accent shrink-0" />}
                </button>
              ))}
              {users.length === 0 && <p className="text-xs text-muted/50 px-2 py-1">No users yet.</p>}
            </div>

            {/* Log out / switch user */}
            {currentUser && (
              <div className="pt-3 border-t border-border mt-2">
                <button
                  onClick={() => { setCurrentUser(null); navigate('/auth') }}
                  className="w-full flex items-center gap-2 px-2 py-1.5 text-xs text-muted hover:text-text transition-colors rounded-lg hover:bg-surface/40"
                >
                  <LogIn size={12} className="rotate-180" />
                  Switch user
                </button>
              </div>
            )}
          </div>
        </aside>

        {/* Main Content */}
        <main className="flex-1 min-w-0 border-r border-border">
          <Routes>
            <Route path="/" element={<Navigate to="/home" replace />} />
            <Route path="/auth" element={<AuthPage />} />
            <Route path="/home" element={<RequireUser><HomePage /></RequireUser>} />
            <Route path="/profile/:userId" element={<ProfilePage />} />
            <Route path="/notifications" element={<RequireUser><NotificationsPage /></RequireUser>} />
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
              <NavLink to="/auth" className="block text-xs text-accent hover:underline mt-1">Sign up or log in</NavLink>
            </div>
          )}

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
              <button
                onClick={handleDemo}
                disabled={demoing}
                className="w-full flex items-center justify-center gap-2 text-xs font-semibold bg-accent hover:bg-accent-hover disabled:opacity-50 text-white rounded-lg px-3 py-2 transition-colors"
              >
                <Play size={11} fill="white" />
                {demoing ? 'Running demo…' : 'Run Demo'}
              </button>
              <button
                onClick={handleRebuild}
                disabled={rebuilding}
                className="w-full flex items-center justify-center gap-2 text-xs text-muted hover:text-text border border-border hover:border-border-hover rounded-lg px-3 py-2 transition-colors disabled:opacity-50"
              >
                <RefreshCw size={11} className={rebuilding ? 'animate-spin' : ''} />
                {rebuilding ? 'Rebuilding…' : 'Rebuild Feed Cache'}
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
