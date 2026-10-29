import { describe, expect, it } from "vitest";
import { categoryLabel, formatBytes, formatExpiry, formatPercent, formatPorts, listenerAddress, ownershipLabel } from "./format";

describe("format helpers", () => {
  it("formats bytes with honest unknowns", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(1024)).toBe("1.00 KB");
    expect(formatBytes(1536)).toBe("1.50 KB");
    expect(formatBytes(100 << 20)).toBe("100 MB");
    expect(formatBytes(0, false)).toBe("Unknown");
    expect(formatBytes(Number.NaN)).toBe("Unknown");
  });

  it("bounds percentages and marks unknown", () => {
    expect(formatPercent(8.25)).toBe("8.3%");
    expect(formatPercent(50)).toBe("50%");
    expect(formatPercent(140)).toBe("100%");
    expect(formatPercent(undefined, false)).toBe("Unknown");
  });

  it("formats ports and addresses", () => {
    expect(formatPorts([3000, 5173])).toBe("3000, 5173");
    expect(formatPorts([])).toBe("—");
    expect(listenerAddress("127.0.0.1")).toBe("127.0.0.1");
    expect(listenerAddress("::1")).toBe("[::1]");
  });

  it("uses clear labels", () => {
    expect(categoryLabel("dev-server")).toBe("Dev server");
    expect(categoryLabel("other")).toBe("Process");
    expect(ownershipLabel("inferred")).toBe("Inferred");
    expect(ownershipLabel("")).toBe("Not associated");
  });

  it("formats a future review expiry instead of calling it just now", () => {
    const now = Date.parse("2026-10-28T12:00:00Z");
    expect(formatExpiry("2026-10-28T12:00:30Z", now)).toBe("in 30s");
    expect(formatExpiry("2026-10-28T12:10:00Z", now)).toBe("in 10m");
    expect(formatExpiry("2026-10-28T14:00:00Z", now)).toBe("in 2h");
    expect(formatExpiry("2026-10-28T11:59:59Z", now)).toBe("Expired");
    expect(formatExpiry("not-a-date", now)).toBe("Not available");
  });
});
