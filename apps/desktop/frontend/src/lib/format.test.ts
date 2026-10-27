import { describe, expect, it } from "vitest";
import { categoryLabel, formatBytes, formatPercent, formatPorts, listenerAddress, ownershipLabel } from "./format";

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
});
