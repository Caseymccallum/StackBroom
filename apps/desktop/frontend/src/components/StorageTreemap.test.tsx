import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { StorageNode } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { StorageTreemap } from "./StorageTreemap";

function node(id: string, bytes: number, kind = "file"): StorageNode {
  return {
    id,
    name: `${id}.bin`,
    kind,
    logicalBytes: bytes,
    allocatedBytes: bytes,
    fileCount: 1,
    dirCount: 0,
    childCount: 0,
  };
}

describe("StorageTreemap", () => {
  it("renders area-proportional tiles with an accessible table fallback heading", () => {
    const nodes = [node("alpha", 75), node("beta", 25)];
    render(<StorageTreemap nodes={nodes} totalBytes={100} onOpen={vi.fn()} />);
    expect(screen.getByRole("button", { name: /alpha\.bin, file, 75 B, 75%/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /beta\.bin, file, 25 B, 25%/i })).toBeInTheDocument();
  });

  it("keeps the visible budget and accounts for dropped nodes in Other", () => {
    const nodes = Array.from({ length: 70 }, (_, index) => node(`node-${index}`, 10));
    render(<StorageTreemap nodes={nodes} totalBytes={700} onOpen={vi.fn()} />);
    const tiles = screen.getAllByRole("button");
    expect(tiles.length).toBeLessThanOrEqual(60);
    expect(tiles.some((tile) => tile.getAttribute("aria-label")?.startsWith("Other"))).toBe(true);
  });
});
