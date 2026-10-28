import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ProcessRow } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";

const preview = vi.fn();
const confirm = vi.fn();

vi.mock("../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/processcontrolservice", () => ({
  GetProcessDetails: vi.fn(),
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
    vi.clearAllMocks();
    preview.mockResolvedValue({
      operationId: "op-1",
      pid: 1234,
      action: "terminate",
      priority: "Normal",
      targetName: "disposable.exe",
      expiresAt: new Date(Date.now() + 60_000).toISOString(),
    });
    confirm.mockResolvedValue({ operationId: "op-1", success: true, outcome: "success", details: "process exit verified after termination" });
  });

  it("requests the native preview before presenting confirmation", async () => {
    const onClose = vi.fn();
    const onCompleted = vi.fn();
    render(<ProcessDetails process={process} onClose={onClose} onCompleted={onCompleted} />);

    fireEvent.click(screen.getByRole("button", { name: /End Process/i }));
    expect(await screen.findByText(/Native preview: disposable\.exe/i)).toBeInTheDocument();
    expect(preview).toHaveBeenCalledWith(1234, "terminate", 0, "opaque-token");
    expect(confirm).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /Confirm End Process for disposable\.exe, PID 1234/i }));
    expect(await screen.findByText("process exit verified after termination")).toBeInTheDocument();
    expect(confirm).toHaveBeenCalledWith("op-1");
    expect(onCompleted).toHaveBeenCalled();
  });

  it("cancels without executing", async () => {
    render(<ProcessDetails process={process} onClose={() => undefined} />);
    fireEvent.click(screen.getByRole("button", { name: /End Process/i }));
    await screen.findByText(/Native preview:/i);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(confirm).not.toHaveBeenCalled();
  });

  it("does not present confirmation when preview creation fails", async () => {
    preview.mockRejectedValue(new Error("identity verification failed"));
    render(<ProcessDetails process={process} onClose={() => undefined} />);
    fireEvent.click(screen.getByRole("button", { name: /End Process/i }));
    expect(await screen.findByText("identity verification failed")).toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(confirm).not.toHaveBeenCalled();
  });

  it("requires a new preview when the native preview expires", async () => {
    preview.mockResolvedValue({
      operationId: "op-expired",
      pid: 1234,
      action: "terminate",
      priority: "Normal",
      targetName: "disposable.exe",
      expiresAt: new Date(Date.now() - 1_000).toISOString(),
    });
    render(<ProcessDetails process={process} onClose={() => undefined} />);
    fireEvent.click(screen.getByRole("button", { name: /End Process/i }));
    expect(await screen.findByText(/This preview has expired/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Confirm End Process for/i })).toBeDisabled();
    expect(confirm).not.toHaveBeenCalled();
  });

  it("closes on Escape and reports backend errors honestly", async () => {
    const onClose = vi.fn();
    confirm.mockResolvedValue({ operationId: "op-1", success: false, outcome: "refused", details: "target identity changed" });
    render(<ProcessDetails process={process} onClose={onClose} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /End Process/i }));
    await screen.findByText(/Native preview:/i);
    fireEvent.click(screen.getByRole("button", { name: /Confirm End Process for/i }));
    await waitFor(() => expect(screen.getByText("target identity changed")).toBeInTheDocument());
  });

  it("does not restore focus outside the modal on a parent rerender", () => {
    const outside = document.createElement("button");
    outside.textContent = "outside";
    document.body.appendChild(outside);
    outside.focus();

    const { rerender, unmount } = render(<ProcessDetails process={process} onClose={vi.fn()} />);
    const close = screen.getByRole("button", { name: "Close process details" });
    expect(close).toHaveFocus();

    rerender(<ProcessDetails process={process} onClose={vi.fn()} />);
    expect(close).toHaveFocus();

    unmount();
    expect(outside).toHaveFocus();
    outside.remove();
  });

  it("keeps preview-only mode incapable of requesting or executing actions", () => {
    render(<ProcessDetails process={process} onClose={() => undefined} previewOnly />);
    expect(screen.getByRole("button", { name: /End Process/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /Close Gracefully/i })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: /End Process/i }));
    expect(preview).not.toHaveBeenCalled();
    expect(confirm).not.toHaveBeenCalled();
  });
});
