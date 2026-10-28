import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AppInfo, FullResult, QuickResult } from "../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";

const quick: QuickResult = {
  id: "scan-1",
  status: "ok",
  mode: "quick",
  startedAt: "2026-10-27T12:00:00Z",
  completedAt: "2026-10-27T12:00:01Z",
  durationMs: 900,
  partial: false,
  warnings: [],
  overview: {
    hostname: "TEST-PC",
    windowsVersion: "Windows 11 (build 26100)",
    buildNumber: 26100,
    logicalProcessors: 16,
    totalMemoryBytes: 32 << 30,
    availableMemoryBytes: 16 << 30,
    memoryKnown: true,
    memoryUsedPercent: 50,
    cpuPercent: 12,
    cpuKnown: true,
    processCount: 2,
    devProcessCount: 1,
    devServerCount: 1,
    topProcesses: [],
    volumes: [{ root: "C:\\", label: "System", fileSystem: "NTFS", totalBytes: 1000, freeBytes: 400, usedBytes: 600, usedPercent: 60 }],
    capturedAt: "2026-10-27T12:00:01Z",
    scanMode: "quick",
  },
};

const full: FullResult = {
  ...quick,
  id: "scan-2",
  mode: "full",
  completedAt: "2026-10-27T12:00:03Z",
  durationMs: 1900,
  overview: { ...quick.overview, scanMode: "full", capturedAt: "2026-10-27T12:00:03Z" },
  processes: [
    { pid: 100, parentPid: 0, name: "node.exe", runtime: "node", category: "dev-server", memoryBytes: 100, memoryKnown: true, startTimeKnown: true, projectName: "demo", requestedPorts: [3000], listeningPorts: [3000], portOwnership: "confirmed" },
    { pid: 200, parentPid: 0, name: "Code.exe", runtime: "unknown", category: "other", memoryBytes: 200, memoryKnown: true, startTimeKnown: false },
  ],
  developer: {
    docker: { available: true, endpointLocal: true, remoteApproved: false, running: 1, stopped: 0, containers: [], diskUsage: [], images: [], imagesTruncated: false, warnings: [] },
    wsl: { available: true, distributions: [], vmMemoryBytes: 0, vmMemoryKnown: false, warnings: [] },
    listeners: [{ protocol: "tcp", family: "ipv4", address: "127.0.0.1", port: 3000, pid: 100, processName: "node.exe", wildcard: false, ownership: "confirmed" }],
    duplicates: [],
  },
  opportunities: [],
  warnings: [],
};

const info: AppInfo = { name: "stackbroom", version: "0.2.0", schemaVersion: "3", toolchain: "go1.27.0", readOnly: true, privacy: "narrow", limitations: ["read-only"] };

vi.mock("../bindings/github.com/Caseymccallum/StackBroom/apps/desktop", () => ({
  DesktopService: {
    GetAppInfo: vi.fn(async () => info),
    QuickScan: vi.fn(async () => quick),
    FullScan: vi.fn(async () => full),
    CancelScan: vi.fn(async () => false),
  },
}));

import App from "./App";

describe("App", () => {
  beforeEach(() => {
    window.localStorage.clear();
    window.location.hash = "";
    document.documentElement.dataset.theme = "dark";
  });

  it("loads real-shaped data, navigates and changes theme", async () => {
    render(<App />);
    expect(await screen.findByText("TEST-PC")).toBeInTheDocument();
    expect(screen.queryByText(/Clean Now|Fix All|Optimize PC/i)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Processes/ }));
    expect(await screen.findByText("Observed processes")).toBeInTheDocument();
    expect(screen.getByText("node.exe")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Settings/ }));
    expect(screen.getByRole("heading", { name: "Settings" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Light" }));
    await waitFor(() => expect(document.documentElement.dataset.theme).toBe("light"));
  });

  it("supports keyboard navigation to storage", async () => {
    render(<App />);
    await screen.findByText("TEST-PC");
    fireEvent.keyDown(window, { key: "3", ctrlKey: true });
    expect(await screen.findByRole("heading", { name: "Storage intelligence" })).toBeInTheDocument();
  });

  it("keeps the current destination when skipping to the keyboard scroll region", async () => {
    render(<App />);
    await screen.findByText("TEST-PC");
    fireEvent.keyDown(window, { key: "6", ctrlKey: true });
    fireEvent.click(screen.getByRole("link", { name: "Skip to main content" }));
    expect(window.location.hash).toBe("#settings");
    expect(screen.getByRole("main", { name: "Settings" })).toHaveFocus();
  });
});
