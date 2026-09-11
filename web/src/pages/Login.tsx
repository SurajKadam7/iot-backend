import { FormEvent, useState } from "react";
import type { LoginHandler } from "../App";
import { Brand } from "../components/Brand";

export function Login({ onSubmit, error }: { onSubmit: LoginHandler; error: string | null }) {
  const cognito = (import.meta.env.VITE_AUTH_MODE || "local").toLowerCase() === "cognito";
  const [email, setEmail] = useState(cognito ? "" : "admin@example.com");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await onSubmit(email.trim(), password);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="login-page">
      <div className="login-panel">
        <Brand size="large" />
        <h1>Sign in</h1>
        <p className="muted">
          {cognito
            ? "Use your Cognito email and password. The ID token must include organization_id, role, and can_export."
            : "Local development login. Client admin: admin@example.com. Viewer: user@example.com. Operator console: ops@example.com."}
        </p>
        <form onSubmit={submit} className="stack">
          <label>
            Email
            <input
              type="email"
              autoComplete="username"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </label>
          {cognito && (
            <label>
              Password
              <input
                type="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </label>
          )}
          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}
          <button className="btn primary" type="submit" disabled={busy}>
            {busy ? "Signing in…" : "Continue"}
          </button>
        </form>
      </div>
    </div>
  );
}
