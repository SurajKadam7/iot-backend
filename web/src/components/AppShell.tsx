import type { Me } from "../types";
import { Shell } from "./Shell";

export function AppShell({ me, onLogout }: { me: Me; onLogout: () => void }) {
  return (
    <Shell
      me={me}
      subtitle={me.organization_name}
      nav={[
        { to: "/", label: "Live board", end: true },
        ...(me.role === "org_admin"
          ? [
              { to: "/devices", label: "Devices" },
              { to: "/locations", label: "Locations" },
              { to: "/users", label: "Users" },
            ]
          : []),
      ]}
      roleLabel={me.role === "org_admin" ? "Admin" : "Viewer"}
      onLogout={onLogout}
    />
  );
}
