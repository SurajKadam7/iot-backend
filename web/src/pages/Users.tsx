import { FormEvent, useEffect, useState } from "react";
import { useOutletContext } from "react-router-dom";
import { api } from "../api";
import { EmptyState } from "../components/EmptyState";
import { Modal } from "../components/Modal";
import { PageHeader } from "../components/PageHeader";
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
    <section className="flex flex-col gap-5">
      <PageHeader
        title="Users"
        description="Add people in this organization. Viewers see the live board. Admins also manage devices, locations, and users. Export permission is stored now; CSV download comes later."
        actions={
          <button className="btn btn-primary" type="button" onClick={startCreate}>
            Add user
          </button>
        }
      />
      {error && (
        <div role="alert" className="alert alert-error alert-soft">
          <span>{error}</span>
        </div>
      )}
      {users.length === 0 ? (
        <EmptyState title="No users yet">Add a teammate so they can sign in to this organization.</EmptyState>
      ) : (
        <div className="overflow-x-auto rounded-box border border-base-300 bg-base-100">
          <table className="table">
            <thead>
              <tr>
                <th>Email</th>
                <th>Access</th>
                <th>Export</th>
                <th>Status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => {
                const self = u.id === me.user_id;
                return (
                  <tr key={u.id}>
                    <td className="font-medium" title={u.email}>
                      {u.email}
                      {self ? <span className="text-base-content/60"> (you)</span> : ""}
                    </td>
                    <td>
                      <span className={`badge badge-soft ${u.role === "org_admin" ? "badge-info" : ""}`}>
                        {u.role === "org_admin" ? "Admin" : "Viewer"}
                      </span>
                    </td>
                    <td>
                      <span className={`badge badge-soft ${u.can_export ? "badge-success" : ""}`}>
                        {u.can_export ? "Allowed" : "View only"}
                      </span>
                    </td>
                    <td>
                      <span className={`badge badge-soft ${u.status === "active" ? "badge-success" : ""}`}>
                        {u.status}
                      </span>
                    </td>
                    <td className="text-end">
                      <div className="flex flex-wrap justify-end gap-2">
                        <button type="button" className="btn btn-ghost btn-sm" onClick={() => startEdit(u)}>
                          Edit
                        </button>
                        {!self && (
                          <button
                            type="button"
                            className="btn btn-ghost btn-error btn-sm"
                            onClick={() => {
                              setActive(u);
                              setOpen("delete");
                            }}
                          >
                            Remove
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {(open === "create" || open === "edit") && (
        <Modal title={open === "create" ? "Add user" : "Edit user"} onClose={() => setOpen(null)}>
          <form className="flex flex-col gap-3" onSubmit={onSubmit}>
            <fieldset className="fieldset">
              <legend className="fieldset-legend">Email</legend>
              <input
                type="email"
                className="input w-full"
                required
                disabled={open === "edit"}
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
                autoComplete="off"
              />
            </fieldset>
            <fieldset className="fieldset">
              <legend className="fieldset-legend">Access</legend>
              <select
                className="select w-full"
                value={form.role}
                disabled={open === "edit" && active?.id === me.user_id}
                onChange={(e) => setForm({ ...form, role: e.target.value as Role })}
              >
                <option value="org_user">Viewer — live board only</option>
                <option value="org_admin">Admin — manage devices, locations, and users</option>
              </select>
            </fieldset>
            <label className="label cursor-pointer items-start justify-start gap-3 py-2">
              <input
                type="checkbox"
                className="checkbox mt-0.5"
                checked={form.can_export}
                onChange={(e) => setForm({ ...form, can_export: e.target.checked })}
              />
              <span>
                Allow export
                <span className="block text-sm text-base-content/60">
                  Grants can_export. Historical CSV download is not available yet.
                </span>
              </span>
            </label>
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
        <Modal title="Remove user" onClose={() => setOpen(null)}>
          <p>
            Remove <strong>{active.email}</strong> from this organization? They will no longer be able to sign in.
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
