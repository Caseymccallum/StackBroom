import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { CleanupDiscoveryView, CleanupPreviewView, CleanupRule } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";

const state = vi.hoisted(() => ({
  rules: [] as CleanupRule[],
  rule: null as CleanupRule | null,
  discovery: null as CleanupDiscoveryView | null,
  preview: null as CleanupPreviewView | null,
  busy: false,
  error: null as string | null,
  loadRules: vi.fn(),
  start: vi.fn(),
  cancel: vi.fn(),
  createPreview: vi.fn(),
  clearPreview: vi.fn(),
}));

vi.mock("../hooks/useCleanup", () => ({ useCleanup: () => state }));

import { CleaningPage } from "./Cleaning";

const rule: CleanupRule = {
  id: "user-temp-old",
  version: "1.0.0",
  title: "Old per-user temporary files",
  description: "Old temp files.",
  owner: "Current Windows user",
  category: "Temporary files",
  scopeDisplay: "%TEMP%",
  supportedWindows: "Windows 10+",
  minAgeDays: 14,
  deletionMode: "recycle",
  recoverable: "uncertain",
  risks: ["Applications may still use them."],
  evidence: ["Only regular files."],
  whySafe: "Scoped to user temp and old age.",
};

beforeEach(() => {
  state.rules = [rule];
  state.rule = null;
  state.discovery = null;
  state.preview = null;
  state.busy = false;
  state.error = null;
  vi.clearAllMocks();
});

describe("CleaningPage", () => {
  it("shows a curated rule and an explicit preview-only state with no Clean Now control", () => {
    render(<CleaningPage />);
    expect(screen.getByText("Old per-user temporary files")).toBeInTheDocument();
    expect(screen.getByText(/Preview only — cleaning not yet available/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /clean now|delete|confirm cleanup/i })).not.toBeInTheDocument();
  });

  it("lets the user select candidates and request an expiring preview", () => {
    state.discovery = {
      discoveryId: "d1", rule, status: "completed",
      progress: { phase: "complete", items: 1, bytes: 100, errors: 0, partial: false },
      candidates: [{ id: "c1", name: "old.tmp", relativePath: "old.tmp", logicalBytes: 100, allocatedBytes: 100, allocatedKnown: true, modifiedAt: "2026-10-01T00:00:00Z", uncertain: false }],
      excluded: [], excludedCounts: {}, excludedTotal: 0, scannedFiles: 1, eligible: 1,
      totalLogical: 100, totalAllocated: 100, allocatedUnknown: 0, truncated: false, partial: false,
      coverage: { exhaustive: true, accessDenied: 0, hiddenExcluded: 0, systemExcluded: 0, reparseExcluded: 0, depthLimit: false, entryLimit: false, cancelled: false, timedOut: false, reasons: [] },
      skippedCount: 0, startedAt: "2026-10-01T00:00:00Z", completedAt: "2026-10-01T00:00:01Z",
    };
    render(<CleaningPage />);
    const checkbox = screen.getByRole("checkbox", { name: /Select old\.tmp/i });
    expect(checkbox).toBeChecked();
    fireEvent.click(checkbox);
    fireEvent.click(checkbox);
    fireEvent.click(screen.getByRole("button", { name: /Preview selected cleanup/i }));
    expect(state.createPreview).toHaveBeenCalledWith(["c1"]);
  });

  it("renders recovery uncertainty and no execution control in a preview", () => {
    state.preview = {
      previewId: "p1", ruleId: rule.id, ruleVersion: rule.version, ruleTitle: rule.title,
      selected: [], excluded: [], fileCount: 1, totalLogical: 100, totalAllocated: 100,
      recoverableBytes: 0, recoverableKnown: false, allocatedUnknown: 1,
      deletionMode: "recycle", recoveryNote: "Recycle Bin availability depends on policy.",
      risks: [], sideEffects: [], expiresAt: "2026-10-01T00:10:00Z",
    };
    render(<CleaningPage />);
    expect(screen.getByText("Unknown")).toBeInTheDocument();
    expect(screen.getByText(/No cleanup method exists/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /clean now|delete|confirm cleanup/i })).not.toBeInTheDocument();
  });
});
