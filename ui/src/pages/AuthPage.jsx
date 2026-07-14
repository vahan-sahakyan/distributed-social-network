import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import {
  Zap,
  UserPlus,
  LogIn,
  Play,
  CheckCircle,
  Loader,
  Trash2,
  AlertTriangle,
} from "lucide-react";
import { Avatar } from "../components/Avatar";
import { api } from "../api";
import { useStore } from "../store";
import { runDemo } from "../demo";

// ── Demo progress overlay ─────────────────────────────────────────────────────
const DEMO_STEPS = [
  "Creating users…",
  "Setting up follows…",
  "Publishing posts…",
  "Adding likes & comments…",
  "Rebuilding feed cache…",
  "Done!",
];

function DemoProgress({ steps }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-bg/90 backdrop-blur-sm">
      <div className="bg-surface border border-border rounded-2xl p-8 w-80 shadow-2xl">
        <div className="flex items-center gap-3 mb-6">
          <div className="w-8 h-8 bg-accent rounded-lg flex items-center justify-center shrink-0">
            <Zap size={15} className="text-white" />
          </div>
          <div>
            <p className="text-sm font-bold text-text">Setting up demo…</p>
            <p className="text-xs text-muted">Creating sample data</p>
          </div>
        </div>
        <div className="space-y-3">
          {DEMO_STEPS.map((label, i) => {
            const done =
              steps.includes(label) && label !== steps[steps.length - 1];
            const active = steps[steps.length - 1] === label;
            const pending = !done && !active;
            return (
              <div key={label} className="flex items-center gap-3">
                {done ? (
                  <CheckCircle size={15} className="text-green-400 shrink-0" />
                ) : active ? (
                  <Loader
                    size={15}
                    className="text-accent shrink-0 animate-spin"
                  />
                ) : (
                  <div className="w-[15px] h-[15px] rounded-full border border-border shrink-0" />
                )}
                <span
                  className={`text-sm ${done ? "text-muted line-through" : active ? "text-text font-medium" : "text-muted/40"}`}
                >
                  {label}
                </span>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// ── Auth page ─────────────────────────────────────────────────────────────────
export function AuthPage() {
  const navigate = useNavigate();
  const { users, addUser, setCurrentUser, removeUser, clearUsers, toast } =
    useStore();
  const [tab, setTab] = useState(users.length > 0 ? "login" : "signup");

  // sign-up state
  const [username, setUsername] = useState("");
  const [bio, setBio] = useState("");
  const [creating, setCreating] = useState(false);

  // advanced lookup
  const [lookupId, setLookupId] = useState("");
  const [looking, setLooking] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);

  // demo state
  const [demoRunning, setDemoRunning] = useState(false);
  const [demoSteps, setDemoSteps] = useState([]);

  // stale-user detection
  const [staleCount, setStaleCount] = useState(0);
  const [validating, setValidating] = useState(false);

  // On mount: silently validate stored users against the backend
  useEffect(() => {
    if (users.length === 0) return;
    setValidating(true);
    let stale = 0;
    Promise.all(
      users.map((u) =>
        api.getUser(u.id).catch(() => {
          stale++;
          return null;
        }),
      ),
    )
      .then((results) => {
        results.forEach((result, i) => {
          if (!result) removeUser(users[i].id);
        });
        setStaleCount(stale);
      })
      .finally(() => setValidating(false));
  }, []); // run once on mount

  async function handleSignup() {
    if (!username.trim() || creating) return;
    setCreating(true);
    try {
      const user = await api.createUser(username.trim(), bio.trim());
      addUser(user);
      setCurrentUser(user);
      toast("Welcome, @" + user.username + "!");
      navigate("/home");
    } catch (err) {
      toast(err.message, "error");
    } finally {
      setCreating(false);
    }
  }

  function handleSelectUser(user) {
    setCurrentUser(user);
    navigate("/home");
  }

  async function handleLookup() {
    if (!lookupId.trim() || looking) return;
    setLooking(true);
    try {
      const user = await api.getUser(lookupId.trim());
      addUser(user);
      setCurrentUser(user);
      toast("Logged in as @" + user.username);
      navigate("/home");
    } catch {
      toast("User not found", "error");
    } finally {
      setLooking(false);
    }
  }

  async function handleRunDemo() {
    setDemoRunning(true);
    setDemoSteps([DEMO_STEPS[0]]);
    try {
      const { users: demoUsers, loginAs } = await runDemo((step) => {
        setDemoSteps((prev) => [...prev, step]);
      });
      // register all demo users in the store
      Object.values(demoUsers).forEach((u) => addUser(u));
      if (loginAs) {
        setCurrentUser(loginAs);
        toast("Demo ready! Logged in as @" + loginAs.username);
        navigate("/home");
      }
    } catch (err) {
      toast("Demo failed: " + err.message, "error");
      setDemoRunning(false);
    }
  }

  return (
    <>
      {demoRunning && <DemoProgress steps={demoSteps} />}

      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="w-full max-w-md space-y-4">
          {/* Logo */}
          <div className="text-center mb-8">
            <div className="w-14 h-14 bg-accent rounded-2xl flex items-center justify-center mx-auto mb-4 shadow-lg shadow-accent/20">
              <Zap size={24} className="text-white" />
            </div>
            <h1 className="text-2xl font-bold text-text">SocialNet</h1>
            <p className="text-sm text-muted mt-1">
              Distributed Social Network Playground
            </p>
          </div>

          {/* Demo CTA */}
          <button
            onClick={handleRunDemo}
            disabled={demoRunning}
            className="w-full flex items-center justify-center gap-5 py-4 bg-accent hover:bg-accent-hover disabled:opacity-50 text-white font-semibold rounded-2xl transition-colors shadow-lg shadow-accent/20 group"
          >
            <div className="w-7 h-7 bg-white/20 rounded-lg flex items-center justify-center group-hover:bg-white/30 transition-colors">
              <Play size={14} className="text-white" fill="white" />
            </div>
            <p className="w-3/4 me-2">Run Demo — create 4 users, posts, likes & comments</p>
          </button>

          {/* Divider */}
          <div className="flex items-center gap-3">
            <div className="flex-1 border-t border-border" />
            <span className="text-xs text-muted">or do it manually</span>
            <div className="flex-1 border-t border-border" />
          </div>

          {/* Sign up / Log in card */}
          <div className="bg-surface border border-border rounded-2xl overflow-hidden shadow-xl">
            {/* Tabs */}
            <div className="flex border-b border-border">
              <button
                onClick={() => setTab("signup")}
                className={`flex-1 flex items-center justify-center gap-2 py-3.5 text-sm font-semibold transition-colors
                  ${tab === "signup" ? "text-text bg-surface-hover border-b-2 border-accent" : "text-muted hover:text-text"}`}
              >
                <UserPlus size={14} /> Sign Up
              </button>
              <button
                onClick={() => setTab("login")}
                className={`flex-1 flex items-center justify-center gap-2 py-3.5 text-sm font-semibold transition-colors
                  ${tab === "login" ? "text-text bg-surface-hover border-b-2 border-accent" : "text-muted hover:text-text"}`}
              >
                <LogIn size={14} /> Log In
              </button>
            </div>

            <div className="p-6">
              {/* Sign Up */}
              {tab === "signup" && (
                <div className="space-y-4">
                  <div>
                    <label className="block text-xs font-medium text-muted mb-1.5">
                      Username
                    </label>
                    <input
                      autoFocus
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") handleSignup();
                      }}
                      placeholder="alice"
                      className="w-full bg-bg border border-border rounded-xl px-4 py-3 text-sm text-text placeholder-muted outline-none focus:border-border-hover transition-colors"
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-muted mb-1.5">
                      Bio <span className="text-muted/50">(optional)</span>
                    </label>
                    <textarea
                      value={bio}
                      onChange={(e) => setBio(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" && !e.shiftKey) handleSignup();
                      }}
                      placeholder="Software engineer, coffee lover…"
                      rows={2}
                      className="w-full bg-bg border border-border rounded-xl px-4 py-3 text-sm text-text placeholder-muted outline-none focus:border-border-hover transition-colors resize-none"
                    />
                  </div>
                  <button
                    onClick={handleSignup}
                    disabled={!username.trim() || creating}
                    className="w-full bg-accent hover:bg-accent-hover disabled:opacity-40 text-white font-semibold py-3 rounded-xl transition-colors"
                  >
                    {creating ? "Creating account…" : "Create Account"}
                  </button>
                  {users.length > 0 && (
                    <p className="text-xs text-center text-muted">
                      Have an account?{" "}
                      <button
                        onClick={() => setTab("login")}
                        className="text-accent hover:underline"
                      >
                        Log in
                      </button>
                    </p>
                  )}
                </div>
              )}

              {/* Log In */}
              {tab === "login" && (
                <div className="space-y-4">
                  {/* Stale-data warning */}
                  {staleCount > 0 && (
                    <div className="flex items-start gap-2.5 p-3 bg-yellow-950/40 border border-yellow-800/40 rounded-xl">
                      <AlertTriangle
                        size={14}
                        className="text-yellow-400 shrink-0 mt-0.5"
                      />
                      <div className="flex-1 min-w-0">
                        <p className="text-xs text-yellow-200">
                          {staleCount} account{staleCount > 1 ? "s" : ""} could
                          not be found in the backend — likely the services were
                          restarted. They have been removed.
                        </p>
                      </div>
                    </div>
                  )}
                  {validating && (
                    <p className="text-xs text-muted text-center">
                      Checking accounts…
                    </p>
                  )}
                  {users.length > 0 ? (
                    <div className="space-y-2">
                      {users.map((u) => (
                        <button
                          key={u.id}
                          onClick={() => handleSelectUser(u)}
                          className="w-full flex items-center gap-3 p-3 bg-bg hover:bg-surface-hover border border-border rounded-xl transition-colors text-left group"
                        >
                          <Avatar username={u.username} size="sm" />
                          <div className="flex-1 min-w-0">
                            <p className="text-sm font-semibold text-text">
                              @{u.username}
                            </p>
                            {u.bio && (
                              <p className="text-xs text-muted truncate">
                                {u.bio}
                              </p>
                            )}
                          </div>
                          <LogIn
                            size={14}
                            className="text-muted group-hover:text-accent transition-colors shrink-0"
                          />
                        </button>
                      ))}
                    </div>
                  ) : (
                    <div className="py-6 text-center">
                      <p className="text-sm text-muted">No accounts yet.</p>
                      <button
                        onClick={() => setTab("signup")}
                        className="text-xs text-accent hover:underline mt-1"
                      >
                        Create one to get started
                      </button>
                    </div>
                  )}

                  {/* Advanced: lookup by ID */}
                  <div className="pt-1">
                    <button
                      onClick={() => setShowAdvanced((v) => !v)}
                      className="text-xs text-muted/50 hover:text-muted transition-colors w-full text-center"
                    >
                      {showAdvanced ? "▲ Hide" : "▼ Advanced"}
                    </button>
                    {showAdvanced && (
                      <div className="mt-3 space-y-3">
                        <div className="space-y-2">
                          <p className="text-xs text-muted/70">
                            Look up an account by its system ID
                          </p>
                          <div className="flex gap-2">
                            <input
                              value={lookupId}
                              onChange={(e) => setLookupId(e.target.value)}
                              onKeyDown={(e) => {
                                if (e.key === "Enter") handleLookup();
                              }}
                              placeholder="User ID…"
                              className="flex-1 bg-bg border border-border rounded-xl px-3 py-2 text-xs text-text placeholder-muted outline-none focus:border-border-hover transition-colors font-mono"
                            />
                            <button
                              onClick={handleLookup}
                              disabled={!lookupId.trim() || looking}
                              className="bg-surface hover:bg-surface-hover border border-border disabled:opacity-40 text-text text-xs font-semibold px-3 py-2 rounded-xl transition-colors shrink-0"
                            >
                              {looking ? "…" : "Find"}
                            </button>
                          </div>
                        </div>
                        {users.length > 0 && (
                          <button
                            onClick={() => {
                              clearUsers();
                              setStaleCount(0);
                            }}
                            className="w-full flex items-center justify-center gap-1.5 text-xs text-red-400/70 hover:text-red-400 transition-colors py-1"
                          >
                            <Trash2 size={11} />
                            Clear all saved accounts
                          </button>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
