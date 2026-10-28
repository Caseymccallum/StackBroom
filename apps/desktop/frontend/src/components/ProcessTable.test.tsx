import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ProcessRow } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { ProcessTable } from "./ProcessTable";

const row: ProcessRow = {
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

describe("ProcessTable", () => {
  it("exposes process details as a keyboard-focusable button", () => {
    const onProcessClick = vi.fn();
    render(
      <ProcessTable
        rows={[row]}
        page={0}
        sort="memory"
        onSort={() => undefined}
        onPage={() => undefined}
        onProcessClick={onProcessClick}
      />,
    );
    const button = screen.getByRole("button", { name: /Open details for disposable\.exe, PID 1234/i });
    button.focus();
    expect(button).toHaveFocus();
    fireEvent.click(button);
    expect(onProcessClick).toHaveBeenCalledWith(row);
  });
});
