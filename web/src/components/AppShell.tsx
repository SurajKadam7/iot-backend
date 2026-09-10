import { NavLink, Outlet } from "react-router-dom";
import type { Me } from "../types";

export function AppShell({ me, onLogout }: { me: Me; onLogout: () => void }) {
  return (
    <div className="app">
      <a className="skip" href="#main">
        Skip to content
      </a>
      <aside className="sidebar" aria-label="Primary">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true" />
          <div>
            <div className="brand-name">Live Feed</div>
            <div className="brand-org">{me.organization_name}</div>
          </div>
        </div>
        <nav className="nav">
          <NavLink to="/" end>
            Live board
          </NavLink>
          {me.role === "org_admin" && (
            <>
              <NavLink to="/devices">Devices</NavLink>
              <NavLink to="/locations">Locations</NavLink>
            </>
          )}
        </nav>
        <div className="sidebar-foot">
          <div className="who">
            <span className="who-email">{me.email}</span>
            <span className="pill">{me.role === "org_admin" ? "Admin" : "Viewer"}</span>
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
