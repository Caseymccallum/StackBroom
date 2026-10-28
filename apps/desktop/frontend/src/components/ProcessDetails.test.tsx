import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ProcessRow } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";

const preview = vi.fn();
const confirm = vi.fn();
const details = vi.fn();

vi.mock("../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/processcontrolservice", () => ({
  GetProcessDetails: (...args: unknown[]) => details(...args),
  PreviewProcessAction: (...args: unknown[]) => preview(...args),
  ConfirmProcessAction: (...args: unknown[]) => confirm(...args),
}));

import { ProcessDetails } from "./ProcessDetails";

const process: ProcessRow = {
  pid: 1234,
  parentPid: 1,
  name: "disposable.exe",
  runtime: "unknown",
  category: "other",
  memoryBytes: 100,
  memoryKnown: true,
  startTimeKnown: true,
  targetToken: "opaque-token",
};

describe("ProcessDetails", () => {
  beforeEach(() => {
    details.mockResolvedValue({ pid: 1234, name: "disposable.exe", isProtected: false, isCritical: false });
    preview.mockResolvedValue({ operationId: "op-1", pid: 1234, action: "terminate" });
    confirm.mockResolvedValue({ operationId: "op-1", success: true, outcome: "success", details: "process exit verified after termination" });
  });

  it("requires confirmation and displays the backend result", async () => {
    const onClose = vi.fn();
    const onCompleted = vi.fn();
    render(<ProcessDetails process={process} onClose={onClose} onCompleted={onCompleted} />);

    fireEvent.click(screen.getByRole("button", { name: /End Process/i }));
    expect(screen.getByText(/Unsaved application data will be lost/i)).toBeInTheDocument();
    expect(confirm).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /Confirm End Process/i }));
    expect(await screen.findByText("process exit verified after termination")).toBeInTheDocument();
    expect(details).toHaveBeenCalledWith(1234, "opaque-token");
    expect(preview).toHaveBeenCalledWith(1234, "terminate", 0, "opaque-token");
    expect(confirm).toHaveBeenCalledTimes(1);
    expect(onCompleted).toHaveBeenCalled();
  });

  it("cancels without executing", () => {
    render(<ProcessDetails process={process} onClose={() => undefined} />);
    fireEvent.click(screen.getByRole("button", { name: /End Process/i }));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(confirm).not.toHaveBeenCalled();
  });

  it("closes on Escape and reports backend errors honestly", async () => {
    const onClose = vi.fn();
    confirm.mockResolvedValue({ operationId: "op-1", success: false, outcome: "refused", details: "target identity changed" });
    render(<ProcessDetails process={process} onClose={onClose} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /End Process/i }));
    fireEvent.click(screen.getByRole("button", { name: /Confirm End Process/i }));
    await waitFor(() => expect(screen.getByText("target identity changed")).toBeInTheDocument());
    expect(screen.queryByText(/success/i)).not.toBeInTheDocument();
  });
});
