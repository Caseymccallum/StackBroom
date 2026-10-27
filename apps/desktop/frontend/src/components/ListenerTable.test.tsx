import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { ListenerRow } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { ListenerTable } from "./ListenerTable";

const listeners: ListenerRow[] = Array.from({ length: 60 }, (_, index) => ({
  protocol: "tcp",
  family: index % 2 === 0 ? "ipv4" : "ipv6",
  address: index % 2 === 0 ? "127.0.0.1" : "::1",
  port: 3000 + index,
  pid: 1000 + index,
  processName: `proc-${index}.exe`,
  projectName: index === 42 ? "demo" : "",
  wildcard: false,
  ownership: index === 42 ? "inferred" : "confirmed",
}));

describe("ListenerTable", () => {
  it("paginates large listener sets", () => {
    render(<ListenerTable listeners={listeners} />);
    expect(screen.getByText("60 of 60")).toBeInTheDocument();
    expect(screen.getByText("proc-0.exe")).toBeInTheDocument();
    expect(screen.queryByText("proc-59.exe")).not.toBeInTheDocument();
  });

  it("filters by process and ownership", async () => {
    render(<ListenerTable listeners={listeners} />);
    fireEvent.change(screen.getByPlaceholderText("Search address, port, PID or process"), { target: { value: "proc-42" } });
    await waitFor(() => expect(screen.getByText("proc-42.exe")).toBeInTheDocument());
    expect(screen.queryByText("proc-0.exe")).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Filter listener ownership"), { target: { value: "confirmed" } });
    await waitFor(() => expect(screen.queryByText("proc-42.exe")).not.toBeInTheDocument());
    expect(screen.getByText(/No listeners match/)).toBeInTheDocument();
  });
});
