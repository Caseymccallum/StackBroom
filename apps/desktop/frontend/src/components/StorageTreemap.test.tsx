import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { StorageNode } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { computeTreemapLayout, StorageTreemap } from "./StorageTreemap";

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

  it("divides recursive regions by their own totals", () => {
    const tiles = computeTreemapLayout([node("fifty", 50), node("thirty", 30), node("twenty", 20)]);
    const area = (id: string) => {
      const tile = tiles.find((entry) => entry.node.id === id)!;
      return (tile.width * tile.height) / 10_000;
    };
    expect(area("fifty")).toBeCloseTo(0.5, 5);
    expect(area("thirty")).toBeCloseTo(0.3, 5);
    expect(area("twenty")).toBeCloseTo(0.2, 5);
  });

  it("handles single, zero-sized, disproportionate, Other and 60+ node cases", () => {
    expect(computeTreemapLayout([node("single", 1)])).toEqual([
      expect.objectContaining({ node: expect.objectContaining({ id: "single" }), x: 0, y: 0, width: 100, height: 100 }),
    ]);

    const withZero = computeTreemapLayout([node("zero", 0), node("positive", 10)]);
    expect(withZero).toHaveLength(1);
    expect(Number.isFinite(withZero[0].width)).toBe(true);

    const disproportionate = computeTreemapLayout([node("huge", 1_000_000), node("tiny-a", 1), node("tiny-b", 1)]);
    const huge = disproportionate.find((tile) => tile.node.id === "huge")!;
    expect((huge.width * huge.height) / 10_000).toBeCloseTo(0.999998, 5);

    const other = computeTreemapLayout([node("other", 900), node("visible", 100)]);
    const otherTile = other.find((tile) => tile.node.id === "other")!;
    expect((otherTile.width * otherTile.height) / 10_000).toBeCloseTo(0.9, 5);

    const many = computeTreemapLayout(Array.from({ length: 80 }, (_, index) => node(`many-${index}`, 1)));
    expect(many).toHaveLength(60);
  });

  it("never overlaps or leaves the normalized bounds", () => {
    const nodes = [node("a", 50), node("b", 30), node("c", 20), node("d", 7), node("e", 3)];
    const tiles = computeTreemapLayout(nodes);
    for (const tile of tiles) {
      expect(tile.x).toBeGreaterThanOrEqual(-1e-9);
      expect(tile.y).toBeGreaterThanOrEqual(-1e-9);
      expect(tile.x + tile.width).toBeLessThanOrEqual(100 + 1e-9);
      expect(tile.y + tile.height).toBeLessThanOrEqual(100 + 1e-9);
      expect(tile.width).toBeGreaterThan(0);
      expect(tile.height).toBeGreaterThan(0);
    }
    for (let first = 0; first < tiles.length; first++) {
      for (let second = first + 1; second < tiles.length; second++) {
        const a = tiles[first];
        const b = tiles[second];
        const overlapWidth = Math.max(0, Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x));
        const overlapHeight = Math.max(0, Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y));
        expect(overlapWidth * overlapHeight).toBeCloseTo(0, 8);
      }
    }
  });
});
