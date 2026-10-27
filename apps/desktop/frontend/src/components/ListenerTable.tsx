import { useDeferredValue, useEffect, useMemo, useState } from "react";
import { ChevronLeft, ChevronRight, Search } from "lucide-react";
import type { ListenerRow } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { listenerAddress, ownershipLabel } from "../lib/format";
import { Badge, Button } from "./ui";

const PAGE_SIZE = 50;

function ownerTone(ownership: string): "teal" | "amber" | "neutral" {
  if (ownership === "confirmed") return "teal";
  if (ownership === "inferred") return "amber";
  return "neutral";
}

export function ListenerTable({ listeners }: { listeners: ListenerRow[] }) {
  const [query, setQuery] = useState("");
  const [ownership, setOwnership] = useState("all");
  const [page, setPage] = useState(0);
  const deferredQuery = useDeferredValue(query);

  const rows = useMemo(() => {
    const needle = deferredQuery.trim().toLowerCase();
    return listeners.filter((listener) => {
      if (ownership !== "all" && listener.ownership !== ownership) return false;
      if (!needle) return true;
      return [
        listener.address,
        String(listener.port),
        String(listener.pid),
        listener.processName ?? "",
        listener.projectName ?? "",
      ].join(" ").toLowerCase().includes(needle);
    });
  }, [listeners, deferredQuery, ownership]);
  useEffect(() => setPage(0), [deferredQuery, ownership]);

  const pageCount = Math.max(1, Math.ceil(rows.length / PAGE_SIZE));
  const currentPage = Math.min(page, pageCount - 1);
  const visible = rows.slice(currentPage * PAGE_SIZE, currentPage * PAGE_SIZE + PAGE_SIZE);

  return (
    <div className="grid" style={{ gap: 12 }}>
      <div className="toolbar">
        <label className="search-wrap">
          <span className="sr-only">Search listeners</span>
          <Search className="search-icon" aria-hidden="true" />
          <input className="input" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search address, port, PID or process" />
        </label>
        <label>
          <span className="sr-only">Filter listener ownership</span>
          <select className="select" value={ownership} onChange={(event) => setOwnership(event.target.value)}>
            <option value="all">All ownership</option>
            <option value="confirmed">Confirmed</option>
            <option value="inferred">Inferred</option>
            <option value="unknown">Uncertain</option>
          </select>
        </label>
        <span className="muted" style={{ fontSize: 11 }}>{rows.length} of {listeners.length}</span>
      </div>

      {rows.length === 0 ? (
        <div className="empty-state" style={{ minHeight: 120 }}>
          <div className="empty-title">No listeners match</div>
          <div className="empty-copy">Try a different address, port, process or ownership filter.</div>
        </div>
      ) : (
        <>
          <div className="table-wrap">
            <table className="table">
              <caption className="sr-only">Observed TCP listeners</caption>
              <thead>
                <tr><th>Port</th><th>Address</th><th>Family</th><th>PID</th><th>Process</th><th>Project</th><th>Ownership</th></tr>
              </thead>
              <tbody>
                {visible.map((listener) => (
                  <tr key={`${listener.family}-${listener.address}-${listener.port}-${listener.pid}`}>
                    <td className="num strong">{listener.port}</td>
                    <td className="listener-address">
                      {listenerAddress(listener.address)}
                      {listener.wildcard ? <span className="faint"> · wildcard</span> : null}
                    </td>
                    <td>{listener.family}</td>
                    <td className="num">{listener.pid}</td>
                    <td className="name" title={listener.processName}>{listener.processName || "Unknown process"}</td>
                    <td>{listener.projectName || "—"}</td>
                    <td><Badge tone={ownerTone(listener.ownership)}>{ownershipLabel(listener.ownership)}</Badge></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="toolbar" style={{ justifyContent: "space-between" }}>
            <span className="muted" style={{ fontSize: 11 }}>
              Showing {currentPage * PAGE_SIZE + 1}–{Math.min(rows.length, (currentPage + 1) * PAGE_SIZE)}
            </span>
            <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
              <Button variant="ghost" onClick={() => setPage(Math.max(0, currentPage - 1))} disabled={currentPage === 0}>
                <ChevronLeft size={14} /> Previous
              </Button>
              <span className="muted" style={{ fontSize: 11 }}>{currentPage + 1} / {pageCount}</span>
              <Button variant="ghost" onClick={() => setPage(Math.min(pageCount - 1, currentPage + 1))} disabled={currentPage >= pageCount - 1}>
                Next <ChevronRight size={14} />
              </Button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
