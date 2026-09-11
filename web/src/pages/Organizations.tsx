import { useEffect, useState } from "react";
import { api } from "../api";
import { EmptyState } from "../components/EmptyState";
import { PageHeader } from "../components/PageHeader";
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
    <section className="flex flex-col gap-5">
      <PageHeader
        title="Organizations"
        description="Read-only view of every tenant: people, locations, and device counts. Clients do not see this console."
        actions={<span className="font-mono text-sm text-base-content/60">{orgs.length} org{orgs.length === 1 ? "" : "s"}</span>}
      />
      {error && (
        <div role="alert" className="alert alert-error alert-soft">
          <span>{error}</span>
        </div>
      )}
      <div className="flex flex-col gap-4">
        {orgs.map((org) => (
          <article key={org.id} className="card card-border bg-base-100 shadow-sm">
            <div className="card-body gap-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <h2 className="card-title text-pretty" title={org.name}>
                    {org.name}
                  </h2>
                  <p className="font-mono text-sm text-base-content/60 break-all">{org.id}</p>
                </div>
                <span className={`badge badge-soft ${org.status === "active" ? "badge-success" : ""}`}>
                  {org.status}
                </span>
              </div>
              <div className="stats stats-vertical bg-base-200 shadow-none sm:stats-horizontal">
                <div className="stat">
                  <div className="stat-title">Users</div>
                  <div className="stat-value text-2xl">{org.user_count}</div>
                  <div className="stat-desc">limit {org.user_limit}</div>
                </div>
                <div className="stat">
                  <div className="stat-title">Devices</div>
                  <div className="stat-value text-2xl">{org.device_count}</div>
                </div>
                <div className="stat">
                  <div className="stat-title">Locations</div>
                  <div className="stat-value text-2xl">{org.locations.length}</div>
                </div>
              </div>
              <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
                <div>
                  <h3 className="mb-2 text-xs font-semibold tracking-wide text-base-content/60 uppercase">Users</h3>
                  {org.users.length === 0 ? (
                    <p className="text-base-content/60">No users.</p>
                  ) : (
                    <ul className="list bg-base-200 rounded-box">
                      {org.users.map((u) => (
                        <li key={u.id} className="list-row">
                          <span className="list-col-grow min-w-0 truncate" title={u.email}>
                            {u.email}
                          </span>
                          <span
                            className={`badge badge-soft ${u.role === "org_admin" || u.role === "platform_admin" ? "badge-info" : ""}`}
                          >
                            {roleLabel(u.role)}
                          </span>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
                <div>
                  <h3 className="mb-2 text-xs font-semibold tracking-wide text-base-content/60 uppercase">
                    Locations and devices
                  </h3>
                  {org.locations.length === 0 && org.unassigned_device_count === 0 ? (
                    <p className="text-base-content/60">No locations or devices.</p>
                  ) : (
                    <ul className="flex flex-col gap-2">
                      {org.locations.map((loc) => (
                        <li key={loc.id}>
                          <div className="flex items-center justify-between gap-2">
                            <span className="min-w-0 text-pretty">{loc.name}</span>
                            <span className="badge badge-ghost">{loc.device_count}</span>
                          </div>
                          <ul className="mt-1 ml-3 flex flex-col gap-1 border-l border-base-300 pl-3">
                            {loc.sub_locations.map((sub) => (
                              <li key={sub.id} className="flex items-center justify-between gap-2">
                                <span className="min-w-0 text-pretty">{sub.name}</span>
                                <span className="text-sm text-base-content/60">{sub.device_count}</span>
                              </li>
                            ))}
                            {loc.sub_locations.length === 0 && (
                              <li className="text-sm text-base-content/60">No sub-locations</li>
                            )}
                          </ul>
                        </li>
                      ))}
                      {org.unassigned_device_count > 0 && (
                        <li className="flex items-center justify-between gap-2">
                          <span>Unassigned</span>
                          <span className="text-sm text-base-content/60">{org.unassigned_device_count}</span>
                        </li>
                      )}
                      <li className="flex items-center justify-between gap-2 border-t border-base-300 pt-2 font-semibold">
                        <span>Organization total</span>
                        <span>{org.device_count}</span>
                      </li>
                    </ul>
                  )}
                </div>
              </div>
            </div>
          </article>
        ))}
        {orgs.length === 0 && !error && (
          <EmptyState title="No organizations">Nothing is seeded yet.</EmptyState>
        )}
      </div>
    </section>
  );
}
