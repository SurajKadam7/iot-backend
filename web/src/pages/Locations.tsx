import { FormEvent, useEffect, useState } from "react";
import { api } from "../api";
import { EmptyState } from "../components/EmptyState";
import { Modal } from "../components/Modal";
import { PageHeader } from "../components/PageHeader";
import type { Location, SubLocation } from "../types";

type Tree = Location & { sub_locations: SubLocation[] };

export function Locations() {
  const [tree, setTree] = useState<Tree[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState<"loc" | "sub" | null>(null);
  const [parent, setParent] = useState<Location | null>(null);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);

  async function load() {
    try {
      const { locations } = await api.locations();
      const withSubs: Tree[] = [];
      for (const loc of locations) {
        const { sub_locations } = await api.subLocations(loc.id);
        withSubs.push({ ...loc, sub_locations });
      }
      setTree(withSubs);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      if (open === "loc") await api.createLocation(name);
      if (open === "sub" && parent) await api.createSubLocation(parent.id, name);
      setOpen(null);
      setName("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="flex flex-col gap-5">
      <PageHeader
        title="Locations"
        description="Optional grouping for the live board filter. Hierarchy is location → sub-location → device."
        actions={
          <button
            className="btn btn-primary"
            type="button"
            onClick={() => {
              setName("");
              setOpen("loc");
            }}
          >
            Add location
          </button>
        }
      />
      {error && (
        <div role="alert" className="alert alert-error alert-soft">
          <span>{error}</span>
        </div>
      )}
      {tree.length === 0 ? (
        <EmptyState title="No locations">Create a location if you want to filter the live board by plant or line.</EmptyState>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {tree.map((loc) => (
            <article key={loc.id} className="card card-border bg-base-100 shadow-sm">
              <div className="card-body">
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <h2 className="card-title text-pretty" title={loc.name}>
                    {loc.name}
                  </h2>
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    onClick={() => {
                      setParent(loc);
                      setName("");
                      setOpen("sub");
                    }}
                  >
                    Add sub-location
                  </button>
                </div>
                <ul className="list">
                  {loc.sub_locations.map((s) => (
                    <li key={s.id} className="list-row px-0">
                      <span>{s.name}</span>
                    </li>
                  ))}
                  {loc.sub_locations.length === 0 && (
                    <li className="list-row px-0 text-base-content/60">No sub-locations yet.</li>
                  )}
                </ul>
              </div>
            </article>
          ))}
        </div>
      )}

      {open && (
        <Modal title={open === "loc" ? "Add location" : `Add sub-location in ${parent?.name}`} onClose={() => setOpen(null)}>
          <form className="flex flex-col gap-3" onSubmit={submit}>
            <fieldset className="fieldset">
              <legend className="fieldset-legend">Name</legend>
              <input className="input w-full" required value={name} onChange={(e) => setName(e.target.value)} />
            </fieldset>
            <div className="modal-action">
              <button type="button" className="btn btn-ghost" onClick={() => setOpen(null)}>
                Cancel
              </button>
              <button className="btn btn-primary" disabled={busy}>
                {busy ? <span className="loading loading-spinner"></span> : null}
                {busy ? "Saving…" : "Save"}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </section>
  );
}
