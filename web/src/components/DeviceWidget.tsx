import type { Device, Reading } from "../types";

function fmt(n: number, digits = 1): string {
  return n.toLocaleString(undefined, { maximumFractionDigits: digits, minimumFractionDigits: 0 });
}

function relative(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "—";
  const s = Math.max(0, Math.round((Date.now() - then) / 1000));
  if (s < 5) return "just now";
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  return `${h}h ago`;
}

export function DeviceWidget({
  device,
  reading,
  flash,
}: {
  device: Device;
  reading?: Reading;
  flash?: boolean;
}) {
  const waiting = !reading;
  const place = [device.location_name, device.sub_location_name].filter(Boolean).join(" / ");
  return (
    <article className={`widget${waiting ? " waiting" : ""}${flash ? " flash" : ""}`}>
      <header className="widget-head">
        <div className="widget-title">
          <h2 title={device.name}>{device.name}</h2>
          <p className="muted" title={place || device.device_identifier}>
            {place || device.device_identifier}
          </p>
        </div>
        <span className={`status-dot ${waiting ? "idle" : "live"}`}>
          {waiting ? "Waiting" : "Live"}
        </span>
      </header>
      {waiting ? (
        <p className="wait-copy">Waiting for device to publish…</p>
      ) : (
        <div className="metrics">
          <div className="metric">
            <div className="metric-label">Temperature</div>
            <div className="metric-value temp" title={`${fmt(reading.temperature)} °C`}>
              <span className="metric-num">{fmt(reading.temperature)}</span>
              <span className="unit">°C</span>
            </div>
          </div>
          <div className="metric">
            <div className="metric-label">Pressure</div>
            <div className="metric-value pressure" title={`${fmt(reading.pressure)} hPa`}>
              <span className="metric-num">{fmt(reading.pressure)}</span>
              <span className="unit">hPa</span>
            </div>
          </div>
          <div className="metric">
            <div className="metric-label">Humidity</div>
            <div className="metric-value humidity" title={`${fmt(reading.humidity)} %RH`}>
              <span className="metric-num">{fmt(reading.humidity)}</span>
              <span className="unit">%RH</span>
            </div>
          </div>
        </div>
      )}
      <footer className="widget-foot">
        <code title={device.device_identifier}>{device.device_identifier}</code>
        {reading && (
          <time dateTime={reading.updated_at} title={reading.ts}>
            {relative(reading.updated_at)}
          </time>
        )}
      </footer>
    </article>
  );
}
