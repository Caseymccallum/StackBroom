export function formatBytes(value: number | null | undefined, known = true): string {
  if (!known || value === null || value === undefined) return "Unknown";
  if (!Number.isFinite(value) || value < 0) return "Unknown";
  if (value < 1024) return `${Math.round(value)} B`;
  const units = ["KB", "MB", "GB", "TB", "PB"];
  let amount = value / 1024;
  let index = 0;
  while (amount >= 1024 && index < units.length - 1) {
    amount /= 1024;
    index += 1;
  }
  const digits = amount >= 100 ? 0 : amount >= 10 ? 1 : 2;
  return `${amount.toFixed(digits)} ${units[index]}`;
}

export function formatPercent(value: number | null | undefined, known = true): string {
  if (!known || value === null || value === undefined || !Number.isFinite(value)) return "Unknown";
  return `${Math.min(100, Math.max(0, value)).toFixed(value < 10 ? 1 : 0)}%`;
}

export function formatDuration(ms: number | null | undefined): string {
  if (ms === null || ms === undefined || !Number.isFinite(ms)) return "Unknown";
  if (ms < 1000) return `${Math.max(0, Math.round(ms))} ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`;
  const minutes = Math.floor(ms / 60_000);
  const seconds = Math.round((ms % 60_000) / 1000);
  return `${minutes}m ${seconds}s`;
}

export function formatWhen(iso: string | null | undefined): string {
  if (!iso) return "Not yet";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "Unknown";
  const delta = Date.now() - date.getTime();
  if (delta < 15_000) return "Just now";
  if (delta < 60_000) return `${Math.floor(delta / 1000)}s ago`;
  if (delta < 3_600_000) return `${Math.floor(delta / 60_000)}m ago`;
  if (delta < 86_400_000) return `${Math.floor(delta / 3_600_000)}h ago`;
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

export function formatExpiry(iso: string | null | undefined, now = Date.now()): string {
  if (!iso) return "Not available";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "Not available";
  const delta = date.getTime() - now;
  if (delta <= 0) return "Expired";
  if (delta < 60_000) return `in ${Math.ceil(delta / 1000)}s`;
  if (delta < 3_600_000) return `in ${Math.ceil(delta / 60_000)}m`;
  if (delta < 86_400_000) return `in ${Math.ceil(delta / 3_600_000)}h`;
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

export function formatClock(iso: string | null | undefined): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

export function formatPorts(ports: number[] | null | undefined): string {
  if (!ports || ports.length === 0) return "—";
  return ports.join(", ");
}

export function listenerAddress(address: string): string {
  return address.includes(":") && !address.startsWith("[") ? `[${address}]` : address;
}

export function categoryLabel(category: string): string {
  switch (category) {
    case "dev-server":
      return "Dev server";
    case "dev-tool":
      return "Dev tool";
    case "runtime":
      return "Runtime";
    default:
      return "Process";
  }
}

export function ownershipLabel(ownership: string | null | undefined): string {
  switch (ownership) {
    case "confirmed":
      return "Confirmed";
    case "inferred":
      return "Inferred";
    case "unknown":
      return "Uncertain";
    default:
      return "Not associated";
  }
}
