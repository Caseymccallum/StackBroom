import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { FullResult, ProcessRow } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { ProcessesPage } from "./Processes";

const process: ProcessRow = {
  pid: 1234,
  parentPid: 1,
  name: "disposable.exe",
  runtime: "unknown",
  category: "other",
  memoryBytes: 100,
  memoryKnown: true,
  startTimeKnown: true,
  targetToken: "token",
};

function result(processes: ProcessRow[]): FullResult {
  return {
    id: "full",
    status: "complete",
    mode: "full",
    startedAt: "2026-10-28T00:00:00Z",
    completedAt: "2026-10-28T00:00:01Z",
    durationMs: 10,
    partial: false,
    overview: {
      hostname: "TEST",
      windowsVersion: "Windows 11",
      buildNumber: 1,
      logicalProcessors: 8,
      totalMemoryBytes: 100,
      availableMemoryBytes: 50,
      memoryKnown: true,
      memoryUsedPercent: 50,
      cpuPercent: 1,
      cpuKnown: true,
      processCount: processes.length,
      devProcessCount: 0,
      devServerCount: 0,
      topProcesses: [],
      volumes: [],
      capturedAt: "2026-10-28T00:00:00Z",
      scanMode: "full",
    },
    processes,
    developer: { docker: { available: false, containers: [], images: [] }, wsl: { available: false, distributions: [] }, listeners: [], duplicates: [] },
    opportunities: [],
    warnings: [],
  } as unknown as FullResult;
}

describe("ProcessesPage", () => {
  it("closes a selected drawer when a refreshed inventory no longer contains the PID", () => {
    const onScan = vi.fn();
    const { rerender } = render(<ProcessesPage full={result([process])} busy={null} onScan={onScan} />);
    fireEvent.click(screen.getByRole("button", { name: /Open details for disposable\.exe, PID 1234/i }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    rerender(<ProcessesPage full={result([])} busy={null} onScan={onScan} />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
