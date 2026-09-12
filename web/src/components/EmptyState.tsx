import type { ReactNode } from "react";

export function EmptyState({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="card card-border border-dashed bg-base-100">
      <div className="card-body items-center text-center">
        <h2 className="card-title">{title}</h2>
        <p className="text-base-content/60">{children}</p>
      </div>
    </div>
  );
}
