import { FormEvent, useEffect, useState } from "react";
import { api } from "../api";
import { Modal } from "../components/Modal";
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
    <section className="page">
      <header className="page-head">
        <div>
          <h1>Locations</h1>
          <p className="muted">Optional grouping for the live board filter. Hierarchy is location → sub-location → device.</p>
        </div>
        <button
          className="btn primary"
          type="button"
          onClick={() => {
            setName("");
            setOpen("loc");
          }}
        >
          Add location
        </button>
      </header>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <div className="cards">
        {tree.map((loc) => (
          <article key={loc.id} className="card">
            <div className="card-head">
              <h2>{loc.name}</h2>
              <button
                type="button"
                className="btn ghost"
                onClick={() => {
                  setParent(loc);
                  setName("");
                  setOpen("sub");
                }}
              >
                Add sub-location
              </button>
            </div>
            <ul className="sub-list">
              {loc.sub_locations.map((s) => (
                <li key={s.id}>{s.name}</li>
              ))}
              {loc.sub_locations.length === 0 && <li className="muted">No sub-locations yet.</li>}
            </ul>
          </article>
        ))}
        {tree.length === 0 && (
          <div className="empty">
            <h2>No locations</h2>
            <p className="muted">Create a location if you want to filter the live board by plant or line.</p>
          </div>
        )}
      </div>

      {open && (
        <Modal title={open === "loc" ? "Add location" : `Add sub-location in ${parent?.name}`} onClose={() => setOpen(null)}>
          <form className="stack" onSubmit={submit}>
            <label>
              Name
              <input required value={name} onChange={(e) => setName(e.target.value)} />
            </label>
            <div className="modal-actions">
              <button type="button" className="btn ghost" onClick={() => setOpen(null)}>
                Cancel
              </button>
              <button className="btn primary" disabled={busy}>
                {busy ? "Saving…" : "Save"}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </section>
  );
}
