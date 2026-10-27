import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Sparkline } from "./Sparkline";

describe("observed chart samples", () => {
  it("breaks lines at missing measurements", () => {
    const { container } = render(<Sparkline label="CPU" values={[20, null, 30]} />);
    expect(screen.getByRole("img")).toHaveAccessibleName(/2 measured of 3/);
    expect(container.querySelector("path")?.getAttribute("d")).not.toContain("L");
    expect(container.querySelectorAll("circle")).toHaveLength(2);
  });
  it("accepts empty and entirely unavailable history", () => {
    const { rerender } = render(<Sparkline label="CPU" values={[]} />);
    expect(screen.getByText("Unavailable")).toBeInTheDocument();
    rerender(<Sparkline label="CPU" values={[null, null]} />);
    expect(screen.getByRole("img")).toHaveAccessibleName(/0 measured of 2/);
  });
});
