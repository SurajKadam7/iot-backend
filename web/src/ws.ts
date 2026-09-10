import type { ConnectionState, Reading } from "./types";
import { getToken } from "./api";

type Handlers = {
  onReading: (r: Reading) => void;
  onSnapshot: (readings: Reading[]) => void;
  onState: (s: ConnectionState) => void;
};

export function connectLive(handlers: Handlers): () => void {
  let closed = false;
  let ws: WebSocket | null = null;
  let attempt = 0;
  let timer: number | undefined;
  let paused = document.hidden;

  const protocol = location.protocol === "https:" ? "wss" : "ws";
  const url = `${protocol}://${location.host}/ws`;

  const stop = () => {
    closed = true;
    window.clearTimeout(timer);
    document.removeEventListener("visibilitychange", onVis);
    ws?.close();
    ws = null;
  };

  const schedule = () => {
    if (closed || paused) return;
    const delay = Math.min(15000, 1000 * 2 ** attempt);
    attempt += 1;
    handlers.onState(attempt === 1 ? "connecting" : "reconnecting");
    timer = window.setTimeout(open, delay);
  };

  const open = () => {
    if (closed || paused) return;
    const token = getToken();
    if (!token) {
      handlers.onState("offline");
      return;
    }
    ws = new WebSocket(url);
    ws.onopen = () => {
      ws?.send(JSON.stringify({ type: "auth", token }));
    };
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data as string) as {
          type: string;
          reading?: Reading;
          readings?: Reading[];
        };
        if (msg.type === "auth_ok") {
          attempt = 0;
          handlers.onState("live");
        } else if (msg.type === "snapshot") {
          handlers.onSnapshot(msg.readings || []);
        } else if (msg.type === "reading" && msg.reading) {
          handlers.onReading(msg.reading);
        }
      } catch {
        /* ignore malformed */
      }
    };
    ws.onclose = () => {
      ws = null;
      if (!closed && !paused) schedule();
    };
    ws.onerror = () => {
      ws?.close();
    };
  };

  const onVis = () => {
    if (document.hidden) {
      paused = true;
      window.clearTimeout(timer);
      ws?.close();
      ws = null;
      handlers.onState("paused");
    } else {
      paused = false;
      attempt = 0;
      handlers.onState("connecting");
      open();
    }
  };

  document.addEventListener("visibilitychange", onVis);
  handlers.onState("connecting");
  open();
  return stop;
}
