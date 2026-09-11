import { useEffect, useState } from "react";
import { api } from "../api";
import type { AuthRole, OrganizationOverview } from "../types";

function roleLabel(role: AuthRole): string {
  switch (role) {
    case "org_admin":
      return "Admin";
    case "platform_admin":
      return "Operator";
    default:
      return "Viewer";
  }
}

export function Organizations() {
  const [orgs, setOrgs] = useState<OrganizationOverview[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .internalOrganizations()
      .then((res) => {
        setOrgs(res.organizations);
        setError(null);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, []);

  return (
    <section className="page">
      <header className="page-head org-head">
        <div className="page-copy">
          <h1>Organizations</h1>
        </div>
        <div className="stat">
          {orgs.length} org{orgs.length === 1 ? "" : "s"}
        </div>
        <p className="muted">
          Read-only view of every tenant: people, locations, and device counts. Clients do not see this console.
        </p>
      </header>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <div className="org-list">
        {orgs.map((org) => (
          <article key={org.id} className="card org-card">
            <div className="card-head">
              <div className="page-copy">
                <h2 title={org.name}>{org.name}</h2>
                <p className="muted">{org.id}</p>
              </div>
              <span className={`pill ${org.status}`}>{org.status}</span>
            </div>
            <div className="stat-row">
              <span>
                <strong>{org.user_count}</strong> users
              </span>
              <span>
                <strong>{org.device_count}</strong> devices
              </span>
              <span>
                <strong>{org.locations.length}</strong> location{org.locations.length === 1 ? "" : "s"}
              </span>
              <span>
                limit <strong>{org.user_limit}</strong>
              </span>
            </div>
            <div className="split">
              <div>
                <h3 className="section-label">Users</h3>
                {org.users.length === 0 ? (
                  <p className="muted">No users.</p>
                ) : (
                  <ul className="plain-list">
                    {org.users.map((u) => (
                      <li key={u.id}>
                        <span title={u.email}>{u.email}</span>
                        <span className={`pill ${u.role === "org_admin" || u.role === "platform_admin" ? "admin" : ""}`}>
                          {roleLabel(u.role)}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
              <div>
                <h3 className="section-label">Locations and devices</h3>
                {org.locations.length === 0 && org.unassigned_device_count === 0 ? (
                  <p className="muted">No locations or devices.</p>
                ) : (
                  <ul className="tree">
                    {org.locations.map((loc) => (
                      <li key={loc.id}>
                        <div className="tree-row">
                          <span>{loc.name}</span>
                          <span className="muted">{loc.device_count}</span>
                        </div>
                        <ul>
                          {loc.sub_locations.map((sub) => (
                            <li key={sub.id} className="tree-row">
                              <span>{sub.name}</span>
                              <span className="muted">{sub.device_count}</span>
                            </li>
                          ))}
                          {loc.sub_locations.length === 0 && <li className="muted">No sub-locations</li>}
                        </ul>
                      </li>
                    ))}
                    {org.unassigned_device_count > 0 && (
                      <li className="tree-row">
                        <span>Unassigned</span>
                        <span className="muted">{org.unassigned_device_count}</span>
                      </li>
                    )}
                    <li className="tree-row tree-total">
                      <span>Organization total</span>
                      <span>{org.device_count}</span>
                    </li>
                  </ul>
                )}
              </div>
            </div>
          </article>
        ))}
        {orgs.length === 0 && !error && (
          <div className="empty">
            <h2>No organizations</h2>
            <p className="muted">Nothing is seeded yet.</p>
          </div>
        )}
      </div>
    </section>
  );
}
