import { useEffect, useMemo, useState } from "react";
import { api } from "../api";
import { DeviceWidget } from "../components/DeviceWidget";
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
    <section className="page">
      <header className="page-head">
        <div>
          <h1>Live board</h1>
          <p className="muted">
            Latest temperature, pressure, and humidity. Units as published by devices (°C, hPa, %RH).
          </p>
        </div>
        <div className={`conn ${conn}`} aria-live="polite">
          <span className="dot" />
          {labelFor(conn)}
        </div>
      </header>
      <div className="toolbar">
        <label className="grow">
          <span className="sr-only">Search devices</span>
          <input
            placeholder="Search name or identifier"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
        <label>
          <span className="sr-only">Filter by location</span>
          <select value={locationId} onChange={(e) => setLocationId(e.target.value)}>
            <option value="">All locations</option>
            {locations.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
        </label>
        <div className="stat">
          {liveCount}/{filtered.length} live
        </div>
      </div>
      {loadError && (
        <p className="error" role="alert">
          {loadError}
        </p>
      )}
      {filtered.length === 0 ? (
        <div className="empty">
          <h2>No devices to show</h2>
          <p className="muted">
            {devices.length === 0
              ? "An admin needs to register devices for this organization."
              : "Try a different search or location filter."}
          </p>
        </div>
      ) : (
        <div className="board">
          {filtered.map((d) => (
            <DeviceWidget key={d.id} device={d} reading={readings[d.id]} flash={flash[d.id]} />
          ))}
        </div>
      )}
    </section>
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
