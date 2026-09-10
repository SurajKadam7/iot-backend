import { FormEvent, useEffect, useState } from "react";
import { api } from "../api";
import { Modal } from "../components/Modal";
import type { Device, Location, SubLocation } from "../types";

export function Devices() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [locations, setLocations] = useState<Location[]>([]);
  const [subs, setSubs] = useState<SubLocation[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState<"create" | "edit" | "delete" | null>(null);
  const [active, setActive] = useState<Device | null>(null);
  const [form, setForm] = useState({ name: "", device_identifier: "", sub_location_id: "", status: "active" });
  const [busy, setBusy] = useState(false);

  async function load() {
    try {
      const [{ devices: d }, { locations: locs }] = await Promise.all([api.devices(), api.locations()]);
      setDevices(d);
      setLocations(locs);
      const nested = await Promise.all(locs.map((l) => api.subLocations(l.id)));
      setSubs(nested.flatMap((n) => n.sub_locations));
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }

  useEffect(() => {
    void load();
  }, []);

  function startCreate() {
    setActive(null);
    setForm({ name: "", device_identifier: "", sub_location_id: "", status: "active" });
    setOpen("create");
  }

  function startEdit(d: Device) {
    setActive(d);
    setForm({
      name: d.name,
      device_identifier: d.device_identifier,
      sub_location_id: d.sub_location_id || "",
      status: d.status,
    });
    setOpen("edit");
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      if (open === "create") {
        await api.createDevice({
          name: form.name,
          device_identifier: form.device_identifier,
          status: form.status,
          sub_location_id: form.sub_location_id || null,
        });
      } else if (open === "edit" && active) {
        await api.patchDevice(active.id, {
          name: form.name,
          status: form.status,
          sub_location_id: form.sub_location_id || "",
        });
      }
      setOpen(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  async function onDelete() {
    if (!active) return;
    setBusy(true);
    try {
      await api.deleteDevice(active.id);
      setOpen(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="page">
      <header className="page-head">
        <div>
          <h1>Devices</h1>
          <p className="muted">Register devices that already have certificates in IoT Core. Identifier is the MQTT topic id.</p>
        </div>
        <button className="btn primary" type="button" onClick={startCreate}>
          Add device
        </button>
      </header>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Identifier</th>
              <th>Location</th>
              <th>Status</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {devices.map((d) => (
              <tr key={d.id}>
                <td>{d.name}</td>
                <td>
                  <code>{d.device_identifier}</code>
                </td>
                <td className="muted">
                  {[d.location_name, d.sub_location_name].filter(Boolean).join(" / ") || "—"}
                </td>
                <td>
                  <span className={`pill ${d.status}`}>{d.status}</span>
                </td>
                <td className="row-actions">
                  <button type="button" className="btn ghost" onClick={() => startEdit(d)}>
                    Edit
                  </button>
                  <button
                    type="button"
                    className="btn ghost danger"
                    onClick={() => {
                      setActive(d);
                      setOpen("delete");
                    }}
                  >
                    Remove
                  </button>
                </td>
              </tr>
            ))}
            {devices.length === 0 && (
              <tr>
                <td colSpan={5} className="muted">
                  No devices yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {(open === "create" || open === "edit") && (
        <Modal title={open === "create" ? "Add device" : "Edit device"} onClose={() => setOpen(null)}>
          <form className="stack" onSubmit={onSubmit}>
            <label>
              Display name
              <input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </label>
            <label>
              Device identifier
              <input
                required
                disabled={open === "edit"}
                value={form.device_identifier}
                onChange={(e) => setForm({ ...form, device_identifier: e.target.value })}
              />
            </label>
            <label>
              Sub-location
              <select
                value={form.sub_location_id}
                onChange={(e) => setForm({ ...form, sub_location_id: e.target.value })}
              >
                <option value="">None</option>
                {subs.map((s) => (
                  <option key={s.id} value={s.id}>
                    {(locations.find((l) => l.id === s.location_id)?.name || "") + " / " + s.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Status
              <select value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value })}>
                <option value="active">active</option>
                <option value="inactive">inactive</option>
              </select>
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

      {open === "delete" && active && (
        <Modal title="Remove device" onClose={() => setOpen(null)}>
          <p>
            Remove <strong>{active.name}</strong> from this organization? It will leave the live board. Certificate
            provisioning is managed outside this app.
          </p>
          <div className="modal-actions">
            <button type="button" className="btn ghost" onClick={() => setOpen(null)}>
              Cancel
            </button>
            <button type="button" className="btn danger" disabled={busy} onClick={() => void onDelete()}>
              {busy ? "Removing…" : "Remove"}
            </button>
          </div>
        </Modal>
      )}
    </section>
  );
}
