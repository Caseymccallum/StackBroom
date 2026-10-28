import type { StorageNode } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { formatBytes, formatPercent } from "../lib/format";

const MAX_TILES = 60;

function tone(node: StorageNode): string {
  if (node.isReparse) return "var(--muted)";
  if (node.isPlaceholder) return "var(--amber)";
  if (node.kind === "dir") return "var(--blue)";
  if (node.kind === "link") return "var(--muted)";
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
      return "var(--teal, var(--accent))";
  }
}

export function collapseTreemapNodes(nodes: StorageNode[]): StorageNode[] {
  if (nodes.length <= MAX_TILES) return nodes;
  const visible = nodes.slice(0, MAX_TILES - 1);
  const rest = nodes.slice(MAX_TILES - 1);
  const other: StorageNode = {
    id: "other",
    name: `${rest.length} smaller items`,
    kind: "other",
    logicalBytes: rest.reduce((sum, node) => sum + node.logicalBytes, 0),
    allocatedBytes: rest.reduce((sum, node) => sum + node.allocatedBytes, 0),
    fileCount: rest.reduce((sum, node) => sum + node.fileCount, 0),
    dirCount: rest.reduce((sum, node) => sum + node.dirCount, 0),
    childCount: 0,
    truncated: true,
  };
  return [...visible, other];
}

export function StorageTreemap({ nodes, totalBytes, onOpen }: { nodes: StorageNode[]; totalBytes?: number; onOpen: (node: StorageNode) => void }) {
  const visible = collapseTreemapNodes(nodes.filter((node) => node.logicalBytes > 0));
  const total = totalBytes || visible.reduce((sum, node) => sum + node.logicalBytes, 0);
  if (total === 0) {
    return <p className="muted">No measurable file content in this folder.</p>;
  }
  return (
    <div className="storage-treemap" role="group" aria-label="Storage treemap. Use the table below for keyboard access.">
      {visible.map((node) => {
        const percent = (node.logicalBytes / total) * 100;
        const clickable = node.kind === "dir";
        return (
          <button
            key={node.id}
            type="button"
            className="storage-tile"
            disabled={!clickable}
            onClick={() => clickable && onOpen(node)}
            style={{
              flexGrow: Math.max(node.logicalBytes, 1),
              flexBasis: `${Math.max(percent, 6)}%`,
              borderColor: tone(node),
            }}
            title={`${node.name}: ${formatBytes(node.logicalBytes)} (${formatPercent(percent)})`}
            aria-label={`${node.name}, ${node.kind}, ${formatBytes(node.logicalBytes)}, ${formatPercent(percent)}${clickable ? ". Open folder" : ""}`}
          >
            <span className="storage-tile-name">{node.name}</span>
            <span className="storage-tile-size">{formatBytes(node.logicalBytes)}</span>
            <span className="storage-tile-percent">{formatPercent(percent)}</span>
          </button>
        );
      })}
    </div>
  );
}
