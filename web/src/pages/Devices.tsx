import { FormEvent, useEffect, useState } from "react";
import { api } from "../api";
import { EmptyState } from "../components/EmptyState";
import { Modal } from "../components/Modal";
import { PageHeader } from "../components/PageHeader";
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
    <section className="flex flex-col gap-5">
      <PageHeader
        title="Devices"
        description="Register devices that already have certificates in IoT Core. Identifier is the MQTT topic id."
        actions={
          <button className="btn btn-primary" type="button" onClick={startCreate}>
            Add device
          </button>
        }
      />
      {error && (
        <div role="alert" className="alert alert-error alert-soft">
          <span>{error}</span>
        </div>
      )}
      {devices.length === 0 ? (
        <EmptyState title="No devices yet">Register a device to start receiving live telemetry.</EmptyState>
      ) : (
        <div className="overflow-x-auto rounded-box border border-base-300 bg-base-100">
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Identifier</th>
                <th>Location</th>
                <th>Status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {devices.map((d) => (
                <tr key={d.id}>
                  <td className="font-medium" title={d.name}>
                    {d.name}
                  </td>
                  <td>
                    <kbd className="kbd kbd-sm" title={d.device_identifier}>
                      {d.device_identifier}
                    </kbd>
                  </td>
                  <td className="text-base-content/60" title={[d.location_name, d.sub_location_name].filter(Boolean).join(" / ") || "—"}>
                    {[d.location_name, d.sub_location_name].filter(Boolean).join(" / ") || "—"}
                  </td>
                  <td>
                    <span className={`badge badge-soft ${d.status === "active" ? "badge-success" : ""}`}>
                      {d.status}
                    </span>
                  </td>
                  <td className="text-end">
                    <div className="flex flex-wrap justify-end gap-2">
                      <button type="button" className="btn btn-ghost btn-sm" onClick={() => startEdit(d)}>
                        Edit
                      </button>
                      <button
                        type="button"
                        className="btn btn-ghost btn-error btn-sm"
                        onClick={() => {
                          setActive(d);
                          setOpen("delete");
                        }}
                      >
                        Remove
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {(open === "create" || open === "edit") && (
        <Modal title={open === "create" ? "Add device" : "Edit device"} onClose={() => setOpen(null)}>
          <form className="flex flex-col gap-3" onSubmit={onSubmit}>
            <fieldset className="fieldset">
              <legend className="fieldset-legend">Display name</legend>
              <input
                className="input w-full"
                required
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </fieldset>
            <fieldset className="fieldset">
              <legend className="fieldset-legend">Device identifier</legend>
              <input
                className="input w-full"
                required
                disabled={open === "edit"}
                value={form.device_identifier}
                onChange={(e) => setForm({ ...form, device_identifier: e.target.value })}
              />
            </fieldset>
            <fieldset className="fieldset">
              <legend className="fieldset-legend">Sub-location</legend>
              <select
                className="select w-full"
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
            </fieldset>
            <fieldset className="fieldset">
              <legend className="fieldset-legend">Status</legend>
              <select
                className="select w-full"
                value={form.status}
                onChange={(e) => setForm({ ...form, status: e.target.value })}
              >
                <option value="active">active</option>
                <option value="inactive">inactive</option>
              </select>
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

      {open === "delete" && active && (
        <Modal title="Remove device" onClose={() => setOpen(null)}>
          <p>
            Remove <strong>{active.name}</strong> from this organization? It will leave the live board. Certificate
            provisioning is managed outside this app.
          </p>
          <div className="modal-action">
            <button type="button" className="btn btn-ghost" onClick={() => setOpen(null)}>
              Cancel
            </button>
            <button type="button" className="btn btn-error" disabled={busy} onClick={() => void onDelete()}>
              {busy ? <span className="loading loading-spinner"></span> : null}
              {busy ? "Removing…" : "Remove"}
            </button>
          </div>
        </Modal>
      )}
    </section>
  );
}
