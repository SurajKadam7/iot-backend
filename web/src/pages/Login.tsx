import { FormEvent, useState } from "react";
import type { LoginHandler } from "../App";
import { AppearanceController } from "../components/AppearanceController";
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
    <div className="hero min-h-screen">
      <div className="absolute right-4 top-4 w-52">
        <AppearanceController align="end" placement="bottom" />
      </div>
      <div className="hero-content w-full max-w-md p-4">
        <div className="card card-border bg-base-100 w-full shadow-xl">
          <div className="card-body gap-4">
            <Brand size="large" />
            <div>
              <h1 className="text-2xl font-semibold tracking-tight">Sign in</h1>
              <p className="mt-1 text-base-content/60">
                {cognito
                  ? "Use your Cognito email and password. The ID token must include organization_id, role, and can_export."
                  : "Local development login. Client admin: admin@example.com. Viewer: user@example.com. Operator console: ops@example.com."}
              </p>
            </div>
            <form onSubmit={submit} className="flex flex-col gap-3">
              <fieldset className="fieldset">
                <legend className="fieldset-legend">Email</legend>
                <input
                  type="email"
                  className="input w-full"
                  autoComplete="username"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </fieldset>
              {cognito && (
                <fieldset className="fieldset">
                  <legend className="fieldset-legend">Password</legend>
                  <input
                    type="password"
                    className="input w-full"
                    autoComplete="current-password"
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                  />
                </fieldset>
              )}
              {error && (
                <div role="alert" className="alert alert-error alert-soft">
                  <span>{error}</span>
                </div>
              )}
              <button className="btn btn-primary" type="submit" disabled={busy}>
                {busy ? <span className="loading loading-spinner"></span> : null}
                {busy ? "Signing in…" : "Continue"}
              </button>
            </form>
          </div>
        </div>
      </div>
    </div>
  );
}
