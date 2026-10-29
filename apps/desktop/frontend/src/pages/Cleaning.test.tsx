import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
  confirmPermanent: vi.fn(),
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
  permanentAvailable: false,
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
  it("shows a curated rule and an explicit not-available state before a review exists", () => {
    render(<CleaningPage />);
    expect(screen.getByText("Old per-user temporary files")).toBeInTheDocument();
    expect(screen.getByText(/Cleaning not available yet/i)).toBeInTheDocument();
    expect(screen.getByText(/Cleaning isn't available in this build\./i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /clean now|delete|confirm cleanup/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/independent security review|owner-authorised|execution gate|safety provider|native execution|audit approval/i)).not.toBeInTheDocument();
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
    expect(screen.getByRole("button", { name: /Review selected files/i })).toBeDisabled();
    fireEvent.click(checkbox);
    fireEvent.click(screen.getByRole("button", { name: /Review selected files/i }));
    expect(state.createPreview).toHaveBeenCalledWith(["c1"]);
  });

  it("selects individual files, clears selection and explains skipped files", () => {
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
    const checkbox = screen.getByRole("checkbox", { name: /Select old\.tmp/i });
    expect(checkbox).not.toBeChecked();
    fireEvent.click(checkbox);
    expect(checkbox).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: /Clear selection/i }));
    expect(checkbox).not.toBeChecked();
    expect(screen.getByText("Changed in the last 14 days")).toBeInTheDocument();
    expect(screen.getByText("Recycle Bin or permanent deletion?")).toBeInTheDocument();
    expect(screen.getAllByText(/Permanent deletion/i).length).toBeGreaterThan(0);
    expect(screen.getByText(/Applications may still use them/i)).toBeInTheDocument();
    expect(screen.getAllByText("100 B").length).toBeGreaterThan(0);
  });

  it("renders a plain recovery estimate and a disabled clean control in a review", () => {
    state.preview = {
      previewId: "p1", ruleId: rule.id, ruleVersion: rule.version, ruleTitle: rule.title,
      selected: [{ id: "c1", name: "old.tmp", relativePath: "old.tmp", logicalBytes: 50, allocatedBytes: 50, allocatedKnown: true, modifiedAt: "2026-10-01T00:00:00Z", uncertain: false }],
      excluded: [], fileCount: 1, totalLogical: 100, totalAllocated: 100,
      recoverableBytes: 50, recoverableKnown: false, allocatedUnknown: 1,
      deletionMode: "recycle", executionAvailable: false, permanentAvailable: false, executionMode: "recycle",
      recoveryNote: "Windows may not be able to put every file in the Recycle Bin.",
      risks: [], sideEffects: [], expiresAt: new Date(Date.now() + 10 * 60_000).toISOString(),
    };
    render(<CleaningPage />);
    expect(screen.getByText("old.tmp")).toBeInTheDocument();
    expect(screen.getByText("At least 100 B")).toBeInTheDocument();
    expect(screen.getByText("At least 50 B")).toBeInTheDocument();
    expect(screen.getByText(/1 file couldn't be measured/i)).toBeInTheDocument();
    expect(screen.getByText(/Expires in 10m/i)).toBeInTheDocument();
    expect(screen.getByText(/Cleaning isn't available in this build yet/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Clean selected files/i })).toBeDisabled();
    expect(screen.queryByRole("button", { name: /clean now|delete/i })).not.toBeInTheDocument();
  });

  it("requires an explicit permanent warning before confirmation and reports measured truth", async () => {
    state.preview = {
      previewId: "p2", ruleId: "crash-dumps-old", ruleVersion: "1.0.0", ruleTitle: "Old crash dumps",
      selected: [{ id: "c1", name: "old.dmp", relativePath: "old.dmp", logicalBytes: 100, allocatedBytes: 100, allocatedKnown: true, modifiedAt: "2026-10-01T00:00:00Z", uncertain: false }],
      excluded: [], fileCount: 1, totalLogical: 100, totalAllocated: 100,
      recoverableBytes: 100, recoverableKnown: true, allocatedUnknown: 0,
      deletionMode: "permanent", executionAvailable: true, permanentAvailable: true, executionMode: "permanent",
      recoveryNote: "Permanent deletion cannot be undone.", risks: [], sideEffects: [],
      expiresAt: new Date(Date.now() + 10 * 60_000).toISOString(),
    };
    state.confirmPermanent.mockResolvedValue({
      previewId: "p2", state: "completed", states: ["completed"], aborted: false,
      permanent: true, recoveryAvailable: false, requested: 1, processed: 1, deleted: 1,
      refused: 0, changed: 0, stillPresent: 0, outcomeUnknown: 0,
      spaceReclaimedBytes: 100, spaceReclaimedKnown: false, items: [],
      startedAt: "2026-10-01T00:00:00Z", completedAt: "2026-10-01T00:00:01Z",
    });
    render(<CleaningPage />);
    fireEvent.click(screen.getByRole("button", { name: /Delete permanently…/i }));
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(screen.getByText(/Delete these files permanently/i)).toBeInTheDocument();
    expect(screen.getAllByText(/cannot be undone/i).length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole("button", { name: /^Delete permanently$/i }));
    await waitFor(() => expect(state.confirmPermanent).toHaveBeenCalledWith("p2"));
    expect(await screen.findByText(/Cleaning complete/i)).toBeInTheDocument();
    expect(screen.getByText(/Space reclaimed: not measured/i)).toBeInTheDocument();
  });
});
