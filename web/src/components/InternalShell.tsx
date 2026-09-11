import type { Me } from "../types";
import { Shell } from "./Shell";

export function InternalShell({ me, onLogout }: { me: Me; onLogout: () => void }) {
  return (
    <Shell
      me={me}
      subtitle="Internal · all organizations"
      nav={[{ to: "/internal", label: "Organizations", end: true }]}
      roleLabel="Operator"
      onLogout={onLogout}
    />
  );
}
