import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Sidebar } from "./Sidebar";

describe("Sidebar", () => {
  it("marks the current destination and exposes developer tools by default", () => {
    render(<Sidebar active="overview" showDeveloperTools version="0.2.0" onNavigate={() => undefined} />);
    expect(screen.getByRole("button", { name: /Overview/ })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: /Developer Tools/ })).toBeInTheDocument();
    expect(screen.getByText("Read-only")).toBeInTheDocument();
  });

  it("can hide developer tools without breaking navigation", () => {
    const onNavigate = vi.fn();
    render(<Sidebar active="overview" showDeveloperTools={false} version="0.2.0" onNavigate={onNavigate} />);
    expect(screen.queryByRole("button", { name: /Developer Tools/ })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Processes/ }));
    expect(onNavigate).toHaveBeenCalledWith("processes");
  });
});
