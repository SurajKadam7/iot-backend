import { useEffect, useMemo, useState } from "react";
import { api } from "../api";
import { DeviceWidget } from "../components/DeviceWidget";
import { EmptyState } from "../components/EmptyState";
import { SearchIcon } from "../components/icons";
import { PageHeader } from "../components/PageHeader";
import type { ConnectionState, Device, Location, Reading } from "../types";
import { connectLive } from "../ws";

export function Dashboard() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [locations, setLocations] = useState<Location[]>([]);
  const [readings, setReadings] = useState<Record<string, Reading>>({});
  const [flash, setFlash] = useState<Record<string, boolean>>({});
  const [conn, setConn] = useState<ConnectionState>("connecting");
  const [query, setQuery] = useState("");
  const [locationId, setLocationId] = useState("");
  const [loadError, setLoadError] = useState<string | null>(null);

  async function refreshCatalog() {
    try {
      const [{ devices: d }, { locations: locs }] = await Promise.all([api.devices(), api.locations()]);
      setDevices(d);
      setLocations(locs);
      setLoadError(null);
    } catch (e) {
      setLoadError(e instanceof Error ? e.message : "Failed to load devices");
    }
  }

  useEffect(() => {
    void refreshCatalog();
    const onFocus = () => void refreshCatalog();
    window.addEventListener("focus", onFocus);
    const id = window.setInterval(() => void refreshCatalog(), 30000);
    return () => {
      window.removeEventListener("focus", onFocus);
      window.clearInterval(id);
    };
  }, []);

  useEffect(() => {
    return connectLive({
      onState: setConn,
      onSnapshot: (list) => {
        const next: Record<string, Reading> = {};
        for (const r of list) next[r.device_id] = r;
        setReadings(next);
      },
      onReading: (r) => {
        setReadings((prev) => ({ ...prev, [r.device_id]: r }));
        setFlash((prev) => ({ ...prev, [r.device_id]: true }));
        window.setTimeout(() => {
          setFlash((prev) => ({ ...prev, [r.device_id]: false }));
        }, 700);
      },
    });
  }, []);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return devices.filter((d) => {
      if (locationId && d.location_id !== locationId) return false;
      if (!q) return true;
      return (
        d.name.toLowerCase().includes(q) ||
        d.device_identifier.toLowerCase().includes(q) ||
        (d.sub_location_name || "").toLowerCase().includes(q)
      );
    });
  }, [devices, query, locationId]);

  const liveCount = filtered.filter((d) => readings[d.id]).length;

  return (
    <section className="flex flex-col gap-5">
      <PageHeader
        title="Live board"
        description="Latest temperature, pressure, and humidity. Units as published by devices (°C, hPa, %RH)."
        actions={<ConnectionBadge conn={conn} />}
      />
      <div className="flex flex-wrap items-stretch gap-3">
        <label className="input min-w-0 flex-1">
          <SearchIcon className="size-4 opacity-50" />
          <input
            type="search"
            placeholder="Search name or identifier"
            aria-label="Search devices"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
        <select
          className="select w-full sm:w-56"
          aria-label="Filter by location"
          value={locationId}
          onChange={(e) => setLocationId(e.target.value)}
        >
          <option value="">All locations</option>
          {locations.map((l) => (
            <option key={l.id} value={l.id}>
              {l.name}
            </option>
          ))}
        </select>
        <div className="flex items-center font-mono text-sm text-base-content/60">
          {liveCount}/{filtered.length} live
        </div>
      </div>
      {loadError && (
        <div role="alert" className="alert alert-error alert-soft">
          <span>{loadError}</span>
        </div>
      )}
      {filtered.length === 0 ? (
        <EmptyState title="No devices to show">
          {devices.length === 0
            ? "An admin needs to register devices for this organization."
            : "Try a different search or location filter."}
        </EmptyState>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {filtered.map((d) => (
            <DeviceWidget key={d.id} device={d} reading={readings[d.id]} flash={flash[d.id]} />
          ))}
        </div>
      )}
    </section>
  );
}

function ConnectionBadge({ conn }: { conn: ConnectionState }) {
  const statusClass =
    conn === "live" ? "status-success" : conn === "offline" ? "status-error" : "status-warning";
  return (
    <div className="badge badge-outline gap-2 py-3" aria-live="polite">
      <span className={`status ${statusClass}`}></span>
      {labelFor(conn)}
    </div>
  );
}

function labelFor(conn: ConnectionState): string {
  switch (conn) {
    case "live":
      return "Live";
    case "connecting":
      return "Connecting";
    case "reconnecting":
      return "Reconnecting";
    case "paused":
      return "Paused";
    default:
      return "Offline";
  }
}
