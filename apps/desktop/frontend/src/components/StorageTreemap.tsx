import type { StorageNode } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { formatBytes, formatPercent } from "../lib/format";

const MAX_TILES = 60;

export interface TreemapTile {
  node: StorageNode;
  x: number;
  y: number;
  width: number;
  height: number;
}

interface LayoutItem {
  node: StorageNode;
  value: number;
}

function tone(node: StorageNode): string {
  if (node.isPlaceholder) return "var(--amber)";
  if (node.isReparse) return "var(--muted)";
  if (node.kind === "dir") return "var(--blue)";
  if (node.kind === "other") return "var(--muted)";
  switch (node.name.split(".").pop()?.toLowerCase()) {
    case "jpg":
    case "jpeg":
    case "png":
    case "gif":
    case "webp":
      return "var(--accent)";
    case "mp4":
    case "mkv":
    case "mov":
      return "var(--violet, #b39ddb)";
    case "zip":
    case "7z":
    case "rar":
      return "var(--amber)";
    case "exe":
    case "dll":
      return "var(--red)";
    default:
      return "var(--accent)";
  }
}

export function computeTreemapLayout(nodes: StorageNode[]): TreemapTile[] {
  const output: TreemapTile[] = [];
  const items: LayoutItem[] = nodes
    .filter((node) => node.logicalBytes > 0)
    .slice(0, MAX_TILES)
    .map((node) => ({ node, value: node.logicalBytes }));
  if (items.length === 0) return output;
  const total = items.reduce((value, item) => value + item.value, 0);
  partition(items, total, 0, 0, 100, 100, output);
  return output;
}

function partition(items: LayoutItem[], regionTotal: number, x: number, y: number, width: number, height: number, output: TreemapTile[]) {
  if (items.length === 0 || regionTotal <= 0) return;
  if (items.length === 1) {
    output.push({ node: items[0].node, x, y, width, height });
    return;
  }
  let cumulative = 0;
  let split = 1;
  for (; split < items.length; split++) {
    cumulative += items[split - 1].value;
    if (cumulative >= regionTotal / 2) break;
  }
  if (split >= items.length) split = items.length - 1;
  const left = items.slice(0, split);
  const right = items.slice(split);
  const leftTotal = left.reduce((value, item) => value + item.value, 0);
  const rightTotal = regionTotal - leftTotal;
  if (rightTotal <= 0) {
    output.push(...left.map((item) => ({ node: item.node, x, y, width, height })));
    return;
  }
  const leftShare = leftTotal / regionTotal;
  if (width >= height) {
    partition(left, leftTotal, x, y, width * leftShare, height, output);
    partition(right, rightTotal, x + width * leftShare, y, width * (1 - leftShare), height, output);
  } else {
    partition(left, leftTotal, x, y, width, height * leftShare, output);
    partition(right, rightTotal, x, y + height * leftShare, width, height * (1 - leftShare), output);
  }
}

export function StorageTreemap({ nodes, totalBytes, onOpen }: { nodes: StorageNode[]; totalBytes?: number; onOpen: (node: StorageNode) => void }) {
  const candidates = nodes.filter((node) => node.logicalBytes > 0);
  const existingOther = candidates.find((node) => node.kind === "other");
  const allRegular = candidates.filter((node) => node.kind !== "other");
  const regular = allRegular.slice(0, MAX_TILES - 1);
  const total = totalBytes && totalBytes > 0 ? totalBytes : candidates.reduce((sum, node) => sum + node.logicalBytes, 0);
  const selected = regular.reduce((sum, node) => sum + node.logicalBytes, 0);
  const remainder = Math.max(total - selected, 0);
  const other = remainder > 0
    ? {
        ...(existingOther ?? {
          id: "__aggregate-other",
          name: "Other",
          kind: "other",
          logicalBytes: 0,
          allocatedBytes: 0,
          fileCount: 0,
          dirCount: 0,
          childCount: 0,
          truncated: true,
        }),
        name: "Other",
        logicalBytes: remainder,
      }
    : null;
  const visible = other ? [...regular, other] : regular;
  const measured = visible.reduce((sum, node) => sum + node.logicalBytes, 0);
  if (measured === 0) {
    return <p className="muted">No measurable file content in this folder.</p>;
  }
  return (
    <div className="storage-treemap" role="group" aria-label="Area-proportional storage treemap. The table below provides equivalent keyboard access.">
      {computeTreemapLayout(visible).map((tile) => {
        const clickable = tile.node.kind === "dir";
        const percent = (tile.node.logicalBytes / total) * 100;
        return (
          <button
            key={tile.node.id}
            type="button"
            className="storage-tile"
            disabled={!clickable}
            onClick={() => clickable && onOpen(tile.node)}
            style={{
              left: `calc(${tile.x}% + 1px)`,
              top: `calc(${tile.y}% + 1px)`,
              width: `calc(${tile.width}% - 2px)`,
              height: `calc(${tile.height}% - 2px)`,
              borderColor: tone(tile.node),
            }}
            title={`${tile.node.name}: ${formatBytes(tile.node.logicalBytes)} (${formatPercent(percent)})`}
            aria-label={`${tile.node.name}, ${tile.node.kind}, ${formatBytes(tile.node.logicalBytes)}, ${formatPercent(percent)}${clickable ? ". Open folder" : ""}`}
          >
            <span className="storage-tile-name">{tile.node.name}</span>
            <span className="storage-tile-size">{formatBytes(tile.node.logicalBytes)}</span>
            <span className="storage-tile-percent">{formatPercent(percent)}</span>
          </button>
        );
      })}
    </div>
  );
}
