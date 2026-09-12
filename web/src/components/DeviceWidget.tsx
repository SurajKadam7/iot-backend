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
    <article
      className={`card card-border @container bg-base-100 shadow-sm transition-[box-shadow] ${waiting ? "opacity-90" : ""} ${flash ? "ring-2 ring-success" : ""}`}
    >
      <div className="card-body gap-4">
        <header className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h2 className="card-title text-base leading-snug text-pretty" title={device.name}>
              {device.name}
            </h2>
            <p className="text-sm text-base-content/60 line-clamp-2" title={place || device.device_identifier}>
              {place || device.device_identifier}
            </p>
          </div>
          <span className={`badge badge-soft shrink-0 ${waiting ? "" : "badge-success"}`}>
            <span className={`status ${waiting ? "status-neutral" : "status-success"}`}></span>
            {waiting ? "Waiting" : "Live"}
          </span>
        </header>
        {waiting ? (
          <div>
            <p className="min-h-16 text-base-content/60">Waiting for device to publish…</p>
            <div className="mt-3 flex gap-2">
              <div className="skeleton h-12 w-full"></div>
              <div className="skeleton h-12 w-full"></div>
              <div className="skeleton h-12 w-full"></div>
            </div>
          </div>
        ) : (
          <div className="stats stats-vertical @min-[18rem]:stats-horizontal w-full bg-base-200">
            <div className="stat px-3 py-3">
              <div className="stat-title">Temperature</div>
              <div className="stat-value font-mono text-xl text-warning" title={`${fmt(reading.temperature)} °C`}>
                {fmt(reading.temperature)}
              </div>
              <div className="stat-desc">°C</div>
            </div>
            <div className="stat px-3 py-3">
              <div className="stat-title">Pressure</div>
              <div className="stat-value font-mono text-xl text-info" title={`${fmt(reading.pressure)} hPa`}>
                {fmt(reading.pressure)}
              </div>
              <div className="stat-desc">hPa</div>
            </div>
            <div className="stat px-3 py-3">
              <div className="stat-title">Humidity</div>
              <div className="stat-value font-mono text-xl text-accent" title={`${fmt(reading.humidity)} %RH`}>
                {fmt(reading.humidity)}
              </div>
              <div className="stat-desc">%RH</div>
            </div>
          </div>
        )}
        <footer className="flex items-baseline justify-between gap-2 text-sm text-base-content/60">
          <kbd className="kbd kbd-sm max-w-[70%] truncate" title={device.device_identifier}>
            {device.device_identifier}
          </kbd>
          {reading && (
            <time dateTime={reading.updated_at} title={reading.ts}>
              {relative(reading.updated_at)}
            </time>
          )}
        </footer>
      </div>
    </article>
  );
}
