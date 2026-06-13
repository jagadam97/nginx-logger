// Pure formatting helpers — no framework dependency.

export function fmtBytes(n) {
  if (!n) return { val: "0", unit: "B" };
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(n) / Math.log(1024));
  const val = (n / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1);
  return { val, unit: units[i] };
}

export function fmtNum(n) {
  return (n || 0).toLocaleString();
}

export function fmtMs(seconds) {
  return ((seconds || 0) * 1000).toFixed(1);
}

export function fmtTime(iso) {
  const d = new Date(iso);
  const pad = (x) => String(x).padStart(2, "0");
  return `${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

// Returns the CSS class suffix for a status chip: s2/s3/s4/s5/s0.
export function statusClass(code) {
  const c = Math.floor(code / 100);
  return c >= 2 && c <= 5 ? `s${c}` : "s0";
}

// datetime-local input value for a Date (local timezone, minute precision).
export function toLocalInput(d) {
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
