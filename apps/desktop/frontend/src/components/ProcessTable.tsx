import { ChevronLeft, ChevronRight } from "lucide-react";
import type { ProcessRow } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import type { ProcessSort } from "../lib/processes";
import { formatBytes, formatPorts, formatWhen } from "../lib/format";
import { Badge, Button } from "./ui";

const PAGE_SIZE = 50;

function categoryTone(category: string): "teal" | "blue" | "neutral" {
  if (category === "dev-server") return "teal";
  if (category === "dev-tool") return "blue";
  return "neutral";
}

function SortHeader({ label, active, descending, onClick }: { label: string; active: boolean; descending: boolean; onClick: () => void }) {
  return (
    <button type="button" className="sort-btn" aria-pressed={active} onClick={onClick}>
      {label}
      {active ? descending ? " ↓" : " ↑" : ""}
    </button>
  );
}

export function ProcessTable({
  rows,
  page,
  sort,
  reverse = false,
  onSort,
  onPage,
  onProcessClick,
}: {
  rows: ProcessRow[];
  page: number;
  sort: ProcessSort;
  reverse?: boolean;
  onSort: (sort: ProcessSort) => void;
  onPage: (page: number) => void;
  onProcessClick?: (process: ProcessRow) => void;
}) {
  const pageCount = Math.max(1, Math.ceil(rows.length / PAGE_SIZE));
  const currentPage = Math.min(page, pageCount - 1);
  const visible = rows.slice(currentPage * PAGE_SIZE, currentPage * PAGE_SIZE + PAGE_SIZE);
  const descending = (sort === "memory") !== reverse;
  return (
    <>
      <div className="table-wrap" tabIndex={0} role="region" aria-label="Scrollable table">
        <table className="table">
          <caption className="sr-only">Observed Windows processes</caption>
          <thead>
            <tr>
              <th aria-sort={sort === "name" ? descending ? "descending" : "ascending" : "none"}><SortHeader label="Process" descending={descending} active={sort === "name"} onClick={() => onSort("name")} /></th>
              <th aria-sort={sort === "pid" ? descending ? "descending" : "ascending" : "none"}><SortHeader label="PID" descending={descending} active={sort === "pid"} onClick={() => onSort("pid")} /></th>
              <th aria-sort={sort === "category" ? descending ? "descending" : "ascending" : "none"}><SortHeader label="Category" descending={descending} active={sort === "category"} onClick={() => onSort("category")} /></th>
              <th aria-sort={sort === "memory" ? descending ? "descending" : "ascending" : "none"}><SortHeader label="Memory" descending={descending} active={sort === "memory"} onClick={() => onSort("memory")} /></th>
              <th>Project</th>
              <th aria-sort={sort === "started" ? descending ? "descending" : "ascending" : "none"}><SortHeader label="Started" descending={descending} active={sort === "started"} onClick={() => onSort("started")} /></th>
              <th>Ports</th>
            </tr>
          </thead>
          <tbody>
            {visible.map((process) => (
              <ProcessTableRow key={process.pid} process={process} onClick={() => onProcessClick?.(process)} />
            ))}
          </tbody>
        </table>
      </div>
      <div className="toolbar" style={{ justifyContent: "space-between", marginTop: 12 }}>
        <span className="muted" style={{ fontSize: 11 }}>
          Showing {currentPage * PAGE_SIZE + 1}–{Math.min(rows.length, (currentPage + 1) * PAGE_SIZE)} of {rows.length}
        </span>
        <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
          <Button variant="ghost" onClick={() => onPage(Math.max(0, currentPage - 1))} disabled={currentPage === 0} aria-label="Previous page">
            <ChevronLeft size={14} /> Previous
          </Button>
          <span className="muted" style={{ fontSize: 11 }}>
            {currentPage + 1} / {pageCount}
          </span>
          <Button variant="ghost" onClick={() => onPage(Math.min(pageCount - 1, currentPage + 1))} disabled={currentPage >= pageCount - 1} aria-label="Next page">
            Next <ChevronRight size={14} />
          </Button>
        </div>
      </div>
    </>
  );
}

function ProcessTableRow({ process, onClick }: { process: ProcessRow; onClick?: () => void }) {
  const ports = [
    process.requestedPorts?.length ? `req ${formatPorts(process.requestedPorts)}` : "",
    process.listeningPorts?.length ? `listen ${formatPorts(process.listeningPorts)}` : "",
  ].filter(Boolean);
  return (
    <tr onClick={onClick} style={{ cursor: onClick ? "pointer" : "default" }} className="process-row">
      <td className="name" title={process.name}>
        {onClick ? (
          <button
            type="button"
            className="row-detail-link"
            aria-label={`Open details for ${process.name}, PID ${process.pid}`}
            onClick={(event) => { event.stopPropagation(); onClick(); }}
          >
            {process.name}
          </button>
        ) : process.name}
      </td>
      <td className="num">{process.pid}</td>
      <td><Badge tone={categoryTone(process.category)}>{process.category}</Badge></td>
      <td className="num">{formatBytes(process.memoryBytes, process.memoryKnown)}</td>
      <td>
        {process.projectName ? (
          <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
            {process.projectName}
            <Badge tone={process.projectConfidence === "confirmed" ? "teal" : "neutral"}>{process.projectConfidence}</Badge>
          </span>
        ) : (
          <span className="faint">—</span>
        )}
      </td>
      <td className="num">{process.startTimeKnown ? formatWhen(process.startedAt) : "Unknown"}</td>
      <td className="num">
        {ports.length > 0 ? (
          <span title={process.portOwnership ? `Ownership: ${process.portOwnership}` : undefined}>{ports.join(" · ")}</span>
        ) : (
          <span className="faint">—</span>
        )}
      </td>
    </tr>
  );
}
