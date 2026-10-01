import { useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { Zap, UserPlus, LogIn, LogOut } from "lucide-react";
import { api } from "../api";
import { keycloak, login, logout, register } from "../auth";
import { useStore } from "../store";

function Logo() {
  return (
    <div className="text-center mb-8">
      <div className="w-14 h-14 bg-accent rounded-2xl flex items-center justify-center mx-auto mb-4 shadow-lg shadow-accent/20">
        <Zap size={24} className="text-white" />
      </div>
      <h1 className="text-2xl font-bold text-text">SocialNet</h1>
      <p className="text-sm text-muted mt-1">Distributed Social Network Playground</p>
    </div>
  );
}

// FinishSignup creates the profile for a Keycloak account that has none yet.
function FinishSignup() {
  const navigate = useNavigate();
  const { setCurrentUser, toast } = useStore();
  const [bio, setBio] = useState("");
  const [creating, setCreating] = useState(false);
  const username = keycloak.tokenParsed?.preferred_username;

  async function handleCreate() {
    if (creating) return;
    setCreating(true);
    try {
      const user = await api.createProfile(bio.trim());
      setCurrentUser(user);
      toast("Welcome, @" + user.username + "!");
      navigate("/home");
    } catch (err) {
      toast(err.message, "error");
    } finally {
      setCreating(false);
    }
  }

  return (
    <div className="bg-surface border border-border rounded-2xl p-6 shadow-xl space-y-4">
      <div>
        <p className="text-sm font-semibold text-text">Finish signing up</p>
        <p className="text-xs text-muted mt-1">
          Logged in as <span className="text-text">@{username}</span>. Add a bio to create your profile.
        </p>
      </div>
      <textarea
        autoFocus
        value={bio}
        onChange={(e) => setBio(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !e.shiftKey) handleCreate();
        }}
        placeholder="Software engineer, coffee lover… (optional)"
        rows={2}
        className="w-full bg-bg border border-border rounded-xl px-4 py-3 text-sm text-text placeholder-muted outline-none focus:border-border-hover transition-colors resize-none"
      />
      <button
        onClick={handleCreate}
        disabled={creating}
        className="w-full bg-accent hover:bg-accent-hover disabled:opacity-40 text-white font-semibold py-3 rounded-xl transition-colors"
      >
        {creating ? "Creating profile…" : "Create profile"}
      </button>
      <button
        onClick={logout}
        className="w-full flex items-center justify-center gap-1.5 text-xs text-muted hover:text-text transition-colors"
      >
        <LogOut size={11} /> Use another account
      </button>
    </div>
  );
}

export function AuthPage() {
  const { currentUser, needsProfile } = useStore();

  if (currentUser) return <Navigate to="/home" replace />;

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <div className="w-full max-w-md space-y-4">
        <Logo />
        {needsProfile ? (
          <FinishSignup />
        ) : (
          <>
            <div className="bg-surface border border-border rounded-2xl p-6 shadow-xl space-y-3">
              <button
                onClick={login}
                className="w-full flex items-center justify-center gap-2 bg-accent hover:bg-accent-hover text-white font-semibold py-3 rounded-xl transition-colors"
              >
                <LogIn size={15} /> Log in
              </button>
              <button
                onClick={register}
                className="w-full flex items-center justify-center gap-2 bg-bg hover:bg-surface-hover border border-border text-text font-semibold py-3 rounded-xl transition-colors"
              >
                <UserPlus size={15} /> Sign up
              </button>
              <p className="text-xs text-center text-muted pt-1">Accounts are managed by Keycloak.</p>
            </div>
            <div className="bg-surface/50 border border-border rounded-2xl p-4 text-xs text-muted space-y-1">
              <p className="font-semibold text-text">Demo data</p>
              <p>
                Run <code className="text-text">make demo</code>, then log in as <span className="text-text">alice</span>,{" "}
                <span className="text-text">bob</span> or <span className="text-text">charlie</span> with password{" "}
                <span className="text-text">password</span>.
              </p>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
