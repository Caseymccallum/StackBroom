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
  it("shows a curated rule and an explicit preview-only state before a preview exists", () => {
    render(<CleaningPage />);
    expect(screen.getByText("Old per-user temporary files")).toBeInTheDocument();
    expect(screen.getByText(/Preview only — cleaning not yet available/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /clean now|delete|confirm cleanup/i })).not.toBeInTheDocument();
  });

  it("does not select candidates automatically and requires conscious selection", () => {
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
    expect(checkbox).not.toBeChecked();
    expect(screen.getByRole("button", { name: /Preview selected cleanup/i })).toBeDisabled();
    fireEvent.click(checkbox);
    fireEvent.click(screen.getByRole("button", { name: /Preview selected cleanup/i }));
    expect(state.createPreview).toHaveBeenCalledWith(["c1"]);
  });

  it("supports an explicit Select all eligible action without changing exclusions", () => {
    state.discovery = {
      discoveryId: "d2", rule, status: "completed",
      progress: { phase: "complete", items: 1, bytes: 100, errors: 0, partial: false },
      candidates: [{ id: "c1", name: "old.tmp", relativePath: "old.tmp", logicalBytes: 100, allocatedBytes: 100, allocatedKnown: true, modifiedAt: "2026-10-01T00:00:00Z", uncertain: false }],
      excluded: [{ relativePath: "new.tmp", reason: "newer than 14 days" }], excludedCounts: { "newer than 14 days": 1 }, excludedTotal: 1, scannedFiles: 2, eligible: 1,
      totalLogical: 100, totalAllocated: 100, allocatedUnknown: 0, truncated: false, partial: false,
      coverage: { exhaustive: true, accessDenied: 0, hiddenExcluded: 0, systemExcluded: 0, reparseExcluded: 0, depthLimit: false, entryLimit: false, cancelled: false, timedOut: false, reasons: [] },
      skippedCount: 0, startedAt: "2026-10-01T00:00:00Z", completedAt: "2026-10-01T00:00:01Z",
    };
    render(<CleaningPage />);
    fireEvent.click(screen.getByRole("button", { name: /Select all eligible/i }));
    expect(screen.getByRole("checkbox", { name: /Select old\.tmp/i })).toBeChecked();
    expect(screen.queryByRole("checkbox", { name: /Select new\.tmp/i })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Clear selection/i }));
    expect(screen.getByRole("checkbox", { name: /Select old\.tmp/i })).not.toBeChecked();
  });

  it("renders recovery uncertainty and a disabled execution control in a preview", () => {
    state.preview = {
      previewId: "p1", ruleId: rule.id, ruleVersion: rule.version, ruleTitle: rule.title,
      selected: [], excluded: [], fileCount: 1, totalLogical: 100, totalAllocated: 100,
      recoverableBytes: 0, recoverableKnown: false, allocatedUnknown: 1,
      deletionMode: "recycle", executionAvailable: false,
      recoveryNote: "Recycle Bin availability depends on policy.",
      risks: [], sideEffects: [], expiresAt: "2026-10-01T00:10:00Z",
    };
    render(<CleaningPage />);
    expect(screen.getByText("Unknown")).toBeInTheDocument();
    expect(screen.getByText(/No cleanup method is enabled/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Confirm cleanup/i })).toBeDisabled();
    expect(screen.queryByRole("button", { name: /clean now|delete/i })).not.toBeInTheDocument();
  });
});
