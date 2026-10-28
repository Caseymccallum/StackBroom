import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { QuickResult, StorageNode, StorageScanView } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";

const state = vi.hoisted(() => ({
  root: { id: "root-1", label: "fixture", displayPath: "C:\\fixture", createdAt: "2026-10-28T00:00:00Z" },
  view: null as StorageScanView | null,
  current: null,
  children: [] as StorageNode[],
  trail: [],
  query: "",
  totalChildren: 0,
  totalLogical: 0,
  remainingLogical: 0,
  nextCursor: "",
  busy: false,
  error: null as string | null,
  chooseRoot: vi.fn(),
  startScan: vi.fn(),
  cancel: vi.fn(),
  openNode: vi.fn(),
  goToTrail: vi.fn(),
  search: vi.fn(),
  nextPage: vi.fn(),
}));

vi.mock("../hooks/useStorage", () => ({ useStorage: () => state }));

import { StoragePage } from "./Storage";

const overview: QuickResult = {
  id: "q",
  status: "complete",
  mode: "quick",
  startedAt: "2026-10-28T00:00:00Z",
  completedAt: "2026-10-28T00:00:01Z",
  durationMs: 1000,
  partial: false,
  overview: {
    hostname: "TEST",
    windowsVersion: "Windows 11",
    buildNumber: 1,
    logicalProcessors: 8,
    totalMemoryBytes: 1,
    availableMemoryBytes: 1,
    memoryKnown: true,
    memoryUsedPercent: 1,
    cpuPercent: 1,
    cpuKnown: true,
    processCount: 1,
    devProcessCount: 0,
    devServerCount: 0,
    topProcesses: [],
    volumes: [{ root: "C:\\", label: "System", fileSystem: "NTFS", totalBytes: 1000, freeBytes: 400, usedBytes: 600, usedPercent: 60 }],
    capturedAt: "2026-10-28T00:00:00Z",
    scanMode: "quick",
  },
  warnings: [],
};

function node(overrides: Partial<StorageNode>): StorageNode {
  return {
    id: "n1",
    name: "media",
    kind: "dir",
    logicalBytes: 100,
    allocatedBytes: 100,
    fileCount: 1,
    dirCount: 0,
    childCount: 0,
    ...overrides,
  };
}

beforeEach(() => {
  state.root = { id: "root-1", label: "fixture", displayPath: "C:\\fixture", createdAt: "2026-10-28T00:00:00Z" };
  state.view = null;
  state.children = [];
  state.totalChildren = 0;
  state.totalLogical = 0;
  state.remainingLogical = 0;
  state.nextCursor = "";
  state.query = "";
  state.error = null;
  vi.clearAllMocks();
});

describe("StoragePage", () => {
  it("requires an explicitly selected root and exposes no cleanup action", () => {
    state.root = null as never;
    render(<StoragePage quick={overview} full={null} />);
    expect(screen.getByText(/Choose a folder to understand its storage/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /delete|clean|remove/i })).not.toBeInTheDocument();
  });

  it("renders measured storage evidence and accessible child rows", () => {
    const child = node({ id: "n2", name: "photos" });
    state.root = { id: "root-1", label: "fixture", displayPath: "C:\\fixture", createdAt: "2026-10-28T00:00:00Z" };
    state.children = [child];
    state.view = {
      scanId: "scan-1",
      rootId: "root-1",
      rootNodeId: "n1",
      rootLabel: "fixture",
      status: "completed",
      progress: { phase: "complete", items: 2, bytes: 100, errors: 0, partial: false },
      partial: false,
      coverage: { exhaustive: true, accessDenied: 0, hiddenExcluded: 0, systemExcluded: 0, reparseExcluded: 0, depthLimit: false, entryLimit: false, cancelled: false, timedOut: false, reasons: [] },
      duplicateCoverage: { enumerated: 1, eligible: 1, policySkipped: 0, candidateConsidered: 1, candidateCap: 4000, hashAttempted: 0, hashFailed: 0, verified: 0, changed: 0, truncated: false },
      nodesNextCursor: "",
      rootChildCount: 1,
      rootLogical: 100,
      rootRemaining: 0,
      totalLogical: 100,
      totalAllocated: 100,
      totalFiles: 1,
      totalDirs: 1,
      skippedCount: 0,
      durationMs: 20,
      hashDurationMs: 0,
      startedAt: "2026-10-28T00:00:00Z",
      completedAt: "2026-10-28T00:00:01Z",
      nodes: [child],
      largestFolders: [child],
      largestFiles: [],
      types: [{ extension: "jpg", files: 1, logicalBytes: 100 }],
      duplicates: [{ id: "dup-1", size: 100, hash: "abc", note: "Different file IDs", files: [{ nodeId: "n2", name: "a.jpg", relativePath: "a.jpg" }, { nodeId: "n3", name: "b.jpg", relativePath: "b.jpg" }] }],
      skipped: [],
      warnings: [],
    };
    render(<StoragePage quick={overview} full={null} />);
    expect(screen.getAllByText("photos").length).toBeGreaterThan(0);
    expect(screen.getByText("Largest folders")).toBeInTheDocument();
    expect(screen.getByText(/Different file IDs/)).toBeInTheDocument();
    expect(screen.getByText(/does not prove that a file is unnecessary/i)).toBeInTheDocument();
    expect(screen.getByRole("region", { name: /Storage children table/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Open folder photos/i }));
    expect(state.openNode).toHaveBeenCalledWith(child);
    expect(screen.queryByRole("button", { name: /delete|clean|remove/i })).not.toBeInTheDocument();
  });

  it("exposes cursor pagination for every indexed child", () => {
    const child = node({ id: "n2", name: "photos" });
    state.root = { id: "root-1", label: "fixture", displayPath: "C:\\fixture", createdAt: "2026-10-28T00:00:00Z" };
    state.children = [child];
    state.totalChildren = 3;
    state.nextCursor = "cursor-1";
    state.view = {
      scanId: "scan-1", rootId: "root-1", rootNodeId: "n1", rootLabel: "fixture", status: "completed",
      progress: { phase: "complete", items: 3, bytes: 100, errors: 0, partial: false }, partial: false,
      coverage: { exhaustive: true, accessDenied: 0, hiddenExcluded: 0, systemExcluded: 0, reparseExcluded: 0, depthLimit: false, entryLimit: false, cancelled: false, timedOut: false, reasons: [] },
      duplicateCoverage: { enumerated: 1, eligible: 1, policySkipped: 0, candidateConsidered: 1, candidateCap: 4000, hashAttempted: 0, hashFailed: 0, verified: 0, changed: 0, truncated: false },
      nodesNextCursor: "cursor-1", rootChildCount: 3, rootLogical: 100, rootRemaining: 50,
      totalLogical: 100, totalAllocated: 100, totalFiles: 1, totalDirs: 1, skippedCount: 0,
      durationMs: 20, hashDurationMs: 0, startedAt: "2026-10-28T00:00:00Z", completedAt: "2026-10-28T00:00:01Z",
      nodes: [child], largestFolders: [], largestFiles: [], types: [], duplicates: [], skipped: [], warnings: [],
    };
    render(<StoragePage quick={overview} full={null} />);
    expect(screen.getByText("1/3 children loaded")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(state.nextPage).toHaveBeenCalled();
  });

  it("shows partial results without inventing completion", () => {
    state.view = {
      scanId: "scan-2", rootId: "root-1", rootNodeId: "n1", rootLabel: "fixture", status: "cancelled",
      progress: { phase: "complete", items: 3, bytes: 10, errors: 1, partial: true },
      partial: true, partialReason: "cancelled", totalLogical: 10, totalAllocated: 10,
      coverage: { exhaustive: false, accessDenied: 0, hiddenExcluded: 0, systemExcluded: 0, reparseExcluded: 0, depthLimit: false, entryLimit: false, cancelled: true, timedOut: false, reasons: ["cancelled"] },
      duplicateCoverage: { enumerated: 1, eligible: 0, policySkipped: 1, candidateConsidered: 0, candidateCap: 4000, hashAttempted: 0, hashFailed: 0, verified: 0, changed: 0, truncated: false },
      nodesNextCursor: "", rootChildCount: 0, rootLogical: 0, rootRemaining: 0,
      totalFiles: 1, totalDirs: 1, skippedCount: 1, durationMs: 5, hashDurationMs: 0,
      startedAt: "2026-10-28T00:00:00Z", completedAt: "2026-10-28T00:00:01Z",
      nodes: [], largestFolders: [], largestFiles: [], types: [], duplicates: [], skipped: [], warnings: [],
    };
    render(<StoragePage quick={overview} full={null} />);
    expect(screen.getByText(/Partial result/i)).toBeInTheDocument();
    expect(screen.getAllByText(/cancelled/i).length).toBeGreaterThan(0);
  });
});
