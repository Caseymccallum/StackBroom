import { useDeferredValue, useEffect, useMemo, useState } from "react";
import { List, RefreshCw, Search } from "lucide-react";
import type { FullResult, ProcessRow } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { filterAndSortProcesses, type ProcessSort } from "../lib/processes";
import { Button, Card, CardBody, EmptyState, LoadingBlock } from "../components/ui";
import { ProcessTable } from "../components/ProcessTable";
import { ProcessDetails } from "../components/ProcessDetails";

interface Props {
  full: FullResult | null;
  busy: "quick" | "full" | null;
  onScan: () => void;
  previewDetail?: boolean;
  previewOnly?: boolean;
}

export function ProcessesPage({ full, busy, onScan, previewDetail, previewOnly }: Props) {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("all");
  const [sort, setSort] = useState<ProcessSort>("memory");
  const [reverse, setReverse] = useState(false);
  const [page, setPage] = useState(0);
  const [selectedProcess, setSelectedProcess] = useState<ProcessRow | null>(null);
  const deferredQuery = useDeferredValue(query);

  const rows = useMemo(
    () => (full ? filterAndSortProcesses(full.processes ?? [], { query: deferredQuery, category, sort, reverse }) : []),
    [full, deferredQuery, category, sort, reverse],
  );

  useEffect(() => {
    if (!previewDetail || selectedProcess || rows.length === 0) return;
    const preferred = rows.find((process) => process.category === "dev-server")
      ?? rows.find((process) => process.category === "dev-tool")
      ?? rows[0];
    setSelectedProcess(preferred);
  }, [previewDetail, selectedProcess, rows]);

  useEffect(() => {
    if (!selectedProcess || !full) return;
    const refreshed = (full.processes ?? []).find((process) => process.pid === selectedProcess.pid);
    if (!refreshed) {
      setSelectedProcess(null);
      return;
    }
    if (refreshed.targetToken !== selectedProcess.targetToken) {
      setSelectedProcess(refreshed);
    }
  }, [full, selectedProcess]);

  useEffect(() => setPage(0), [deferredQuery, category, sort, reverse]);
  function chooseSort(next: ProcessSort) {
    setSort(next);
    setReverse(false);
  }

  if (!full) {
    return (
      <Card>
        {busy ? (
          <LoadingBlock label="Reading the full process inventory" />
        ) : (
          <EmptyState
            icon={<List size={20} />}
            title="Full process detail has not been read"
            action={
              <Button variant="primary" onClick={onScan}>
                <RefreshCw size={14} /> Run full scan
              </Button>
            }
          >
            The quick overview reads local metrics only. A full scan adds command-line classification, project
            association and listener evidence. Process actions are explicit, verified and refused for protected processes.
          </EmptyState>
        )}
      </Card>
    );
  }

  return (
    <Card>
      <div className="card-header" style={{ paddingBottom: 0 }}>
        <div>
          <h2 className="card-title">Observed processes</h2>
          <p className="card-subtitle">
            {(full.processes ?? []).length} processes · measured memory and classifications; Unknown is never estimated.
          </p>
        </div>
        <Button onClick={onScan} disabled={busy !== null}>
          <RefreshCw size={14} className={busy === "full" ? "spin" : undefined} /> Refresh full scan
        </Button>
      </div>
      <CardBody>
        <div className="toolbar" style={{ marginBottom: 14 }}>
          <label className="search-wrap">
            <span className="sr-only">Search processes</span>
            <Search className="search-icon" aria-hidden="true" />
            <input
              className="input"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search name, project, PID or port"
            />
          </label>
          <label>
            <span className="sr-only">Filter by category</span>
            <select className="select" value={category} onChange={(event) => setCategory(event.target.value)}>
              <option value="all">All categories</option>
              <option value="dev-server">Dev servers</option>
              <option value="dev-tool">Dev tools</option>
              <option value="runtime">Runtimes</option>
              <option value="other">Other</option>
            </select>
          </label>
          <label>
            <span className="sr-only">Sort processes</span>
            <select className="select" value={sort} onChange={(event) => chooseSort(event.target.value as ProcessSort)}>
              <option value="memory">Memory, high to low</option>
              <option value="name">Name</option>
              <option value="pid">PID</option>
              <option value="started">Start time</option>
              <option value="category">Category</option>
            </select>
          </label>
          <Button onClick={() => setReverse(current => !current)} aria-label="Reverse sort order">
            {(sort === "memory") !== reverse ? "High to low ↓" : "Low to high ↑"}
          </Button>
          <span className="muted" role="status" style={{ fontSize: 11 }}>
            {rows.length} match{rows.length === 1 ? "" : "es"}
          </span>
        </div>

        {rows.length === 0 ? (
          <EmptyState icon={<Search size={20} />} title="No processes match">
            Try a different name, project, PID or category.
          </EmptyState>
        ) : (
          <ProcessTable
            rows={rows}
            page={page}
            sort={sort}
            reverse={reverse}
            onSort={(next) => {
              if (next === sort) setReverse(current => !current);
              else chooseSort(next);
            }}
            onPage={setPage}
            onProcessClick={setSelectedProcess}
          />
        )}
      </CardBody>

      {selectedProcess && (
        <ProcessDetails
          key={selectedProcess.pid}
          process={selectedProcess}
          onClose={() => setSelectedProcess(null)}
          onCompleted={onScan}
          previewOnly={previewOnly}
        />
      )}
    </Card>
  );
}
