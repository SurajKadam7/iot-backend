import { FormEvent, useEffect, useState } from "react";
import { useOutletContext } from "react-router-dom";
import { api } from "../api";
import { Modal } from "../components/Modal";
import type { Me, OrgUser, Role } from "../types";

export function Users() {
  const me = useOutletContext<Me>();
  const [users, setUsers] = useState<OrgUser[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState<"create" | "edit" | "delete" | null>(null);
  const [active, setActive] = useState<OrgUser | null>(null);
  const [form, setForm] = useState({ email: "", role: "org_user" as Role, can_export: false });
  const [busy, setBusy] = useState(false);

  async function load() {
    try {
      const { users: list } = await api.users();
      setUsers(list);
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
    setForm({ email: "", role: "org_user", can_export: false });
    setOpen("create");
  }

  function startEdit(u: OrgUser) {
    setActive(u);
    setForm({ email: u.email, role: u.role, can_export: u.can_export });
    setOpen("edit");
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      if (open === "create") {
        await api.createUser({ email: form.email, role: form.role, can_export: form.can_export });
      } else if (open === "edit" && active) {
        await api.patchUser(active.id, { role: form.role, can_export: form.can_export });
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
      await api.deleteUser(active.id);
      setOpen(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Remove failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="page">
      <header className="page-head">
        <div className="page-copy">
          <h1>Users</h1>
          <p className="muted">
            Add people in this organization. Viewers see the live board. Admins also manage devices, locations, and
            users. Export permission is stored now; CSV download comes later.
          </p>
        </div>
        <button className="btn primary" type="button" onClick={startCreate}>
          Add user
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
              <th>Email</th>
              <th>Access</th>
              <th>Export</th>
              <th>Status</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {users.map((u) => {
              const self = u.id === me.user_id;
              return (
                <tr key={u.id}>
                  <td data-label="Email" title={u.email}>
                    {u.email}
                    {self ? " (you)" : ""}
                  </td>
                  <td data-label="Access">
                    <span className={`pill ${u.role === "org_admin" ? "admin" : ""}`}>
                      {u.role === "org_admin" ? "Admin" : "Viewer"}
                    </span>
                  </td>
                  <td data-label="Export">
                    <span className={`pill ${u.can_export ? "export" : ""}`}>{u.can_export ? "Allowed" : "View only"}</span>
                  </td>
                  <td data-label="Status">
                    <span className={`pill ${u.status}`}>{u.status}</span>
                  </td>
                  <td className="row-actions" data-label="Actions">
                    <button type="button" className="btn ghost" onClick={() => startEdit(u)}>
                      Edit
                    </button>
                    {!self && (
                      <button
                        type="button"
                        className="btn ghost danger"
                        onClick={() => {
                          setActive(u);
                          setOpen("delete");
                        }}
                      >
                        Remove
                      </button>
                    )}
                  </td>
                </tr>
              );
            })}
            {users.length === 0 && (
              <tr>
                <td colSpan={5} className="muted">
                  No users yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {(open === "create" || open === "edit") && (
        <Modal title={open === "create" ? "Add user" : "Edit user"} onClose={() => setOpen(null)}>
          <form className="stack" onSubmit={onSubmit}>
            <label>
              Email
              <input
                type="email"
                required
                disabled={open === "edit"}
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
                autoComplete="off"
              />
            </label>
            <label>
              Access
              <select
                value={form.role}
                disabled={open === "edit" && active?.id === me.user_id}
                onChange={(e) => setForm({ ...form, role: e.target.value as Role })}
              >
                <option value="org_user">Viewer — live board only</option>
                <option value="org_admin">Admin — manage devices, locations, and users</option>
              </select>
            </label>
            <label className="check">
              <input
                type="checkbox"
                checked={form.can_export}
                onChange={(e) => setForm({ ...form, can_export: e.target.checked })}
              />
              <span className="check-copy">
                Allow export
                <span className="muted">Grants can_export. Historical CSV download is not available yet.</span>
              </span>
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
        <Modal title="Remove user" onClose={() => setOpen(null)}>
          <p>
            Remove <strong>{active.email}</strong> from this organization? They will no longer be able to sign in.
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
