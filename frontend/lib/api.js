// Thin API client for the nginx-logger Go backend.

const BASE = "/api";

export const DURATIONS = {
  "1h": 3600e3,
  "6h": 6 * 3600e3,
  "24h": 24 * 3600e3,
  "2d": 2 * 24 * 3600e3,
  "7d": 7 * 24 * 3600e3,
};

async function getJSON(path) {
  const r = await fetch(BASE + path);
  if (!r.ok) throw new Error(`${path} → ${r.status}`);
  return r.json();
}

// Resolve {from,to} as RFC3339 strings from the current range selection.
// When durationMs is null, the explicit custom from/to (local datetime) is used.
export function resolveRange({ durationMs, customFrom, customTo }) {
  if (durationMs === null && customFrom && customTo) {
    return {
      from: new Date(customFrom).toISOString(),
      to: new Date(customTo).toISOString(),
    };
  }
  const to = new Date();
  const from = new Date(to.getTime() - durationMs);
  return { from: from.toISOString(), to: to.toISOString() };
}

// Sets a multi-value param as a comma-separated list, skipping empty selections.
function setMulti(p, key, vals) {
  if (Array.isArray(vals) ? vals.length : vals) {
    p.set(key, Array.isArray(vals) ? vals.join(",") : vals);
  }
}

function buildQuery(range, { host, status, client_ip } = {}, extra = {}) {
  const p = new URLSearchParams({ from: range.from, to: range.to });
  setMulti(p, "host", host);
  setMulti(p, "status", status);
  setMulti(p, "client_ip", client_ip);
  for (const [k, v] of Object.entries(extra)) if (v) p.set(k, v);
  return p.toString();
}

export const api = {
  filters: () => getJSON("/filters"),
  health: () => fetch(BASE + "/health").then((r) => r.ok).catch(() => false),
  stats: (range, f) => getJSON(`/stats?${buildQuery(range, f)}`),
  timeseries: (range, f) => getJSON(`/timeseries?${buildQuery(range, f)}`),
  // stub_status is proxy-wide, so it takes the range but no tag filters.
  stub: (range) => getJSON(`/stub?${buildQuery(range)}`),
  logs: (range, f, limit) => getJSON(`/logs?${buildQuery(range, f, { limit })}`),
};
