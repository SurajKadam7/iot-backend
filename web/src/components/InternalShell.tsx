import { NavLink, Outlet } from "react-router-dom";
import { Brand } from "./Brand";
import type { Me } from "../types";

export function InternalShell({ me, onLogout }: { me: Me; onLogout: () => void }) {
  return (
    <div className="app">
      <a className="skip" href="#main">
        Skip to content
      </a>
      <aside className="sidebar" aria-label="Primary">
        <Brand subtitle="Internal · all organizations" />
        <nav className="nav">
          <NavLink to="/internal" end>
            Organizations
          </NavLink>
        </nav>
        <div className="sidebar-foot">
          <div className="who">
            <span className="who-email" title={me.email}>
              {me.email}
            </span>
            <span className="pill admin">Operator</span>
          </div>
          <button type="button" className="btn ghost full" onClick={onLogout}>
            Sign out
          </button>
        </div>
      </aside>
      <main id="main" className="main">
        <Outlet />
      </main>
    </div>
  );
}
