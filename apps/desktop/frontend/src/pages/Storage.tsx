import { useEffect, useMemo, useState } from "react";
import { AlertTriangle, ChevronRight, FolderOpen, HardDrive, Pause, Play, Search, ShieldCheck } from "lucide-react";
import type { QuickResult, StorageDuplicateGroup, StorageFile, StorageNode, StorageScanOptions, StorageScanView } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { formatBytes, formatPercent } from "../lib/format";
import { Badge, Button, Card, CardBody, CardHeader, EmptyState } from "../components/ui";
import { StorageTreemap } from "../components/StorageTreemap";
import { useStorage } from "../hooks/useStorage";

const initialOptions: StorageScanOptions = {
  includeHidden: false,
  includeSystem: false,
  maxDepth: 64,
  maxEntries: 250000,
};

export function StoragePage({ quick, full }: { quick: QuickResult | null; full: { overview: QuickResult["overview"] } | null }) {
  const overview = quick && (!full || new Date(quick.overview.capturedAt) >= new Date(full.overview.capturedAt))
    ? quick.overview : full?.overview ?? null;
  const volumes = overview?.volumes ?? [];
  const total = volumes.reduce((sum, volume) => sum + volume.totalBytes, 0);
  const free = volumes.reduce((sum, volume) => sum + volume.freeBytes, 0);
  const used = Math.max(0, total - free);
  const usedPercent = total > 0 ? (used / total) * 100 : 0;
  const storage = useStorage();
  const [options, setOptions] = useState<StorageScanOptions>(initialOptions);
  const [searchTerm, setSearchTerm] = useState("");
  const running = storage.view?.status === "running";
  const result = storage.view && ["completed", "cancelled", "timeout"].includes(storage.view.status) ? storage.view : null;

  useEffect(() => {
    setSearchTerm(storage.query);
  }, [storage.query]);

  useEffect(() => {
    if (!storage.view?.scanId) return;
    const timer = window.setTimeout(() => {
      if (searchTerm !== storage.query) void storage.search(searchTerm);
    }, 250);
    return () => window.clearTimeout(timer);
  }, [searchTerm, storage.query, storage.view?.scanId, storage.search]);

  const visibleChildren = useMemo(() => {
    if (!result || storage.remainingLogical <= 0) return storage.children;
    const other: StorageNode = {
      id: "__other",
      name: storage.query ? `Other matches (${storage.totalChildren - storage.children.length})` : `Other (${storage.totalChildren - storage.children.length})`,
      kind: "other",
      logicalBytes: storage.remainingLogical,
      allocatedBytes: 0,
      fileCount: 0,
      dirCount: 0,
      childCount: 0,
      truncated: true,
    };
    return [...storage.children, other];
  }, [result, storage.children, storage.totalChildren, storage.remainingLogical, storage.query]);

  return (
    <div className="grid" style={{ gap: 16 }}>
      <div className="grid metric-grid">
        <Card className="metric-card" style={{ ["--metric-accent" as string]: "var(--blue)" }}>
          <div className="metric-top"><span>Total capacity</span><span className="metric-icon"><HardDrive size={16} /></span></div>
          <div className="metric-value">{formatBytes(total)}</div>
          <div className="metric-note">{volumes.length} fixed volume{volumes.length === 1 ? "" : "s"}</div>
        </Card>
        <Card className="metric-card" style={{ ["--metric-accent" as string]: "var(--accent)" }}>
          <div className="metric-top"><span>Free space</span><span className="metric-icon"><HardDrive size={16} /></span></div>
          <div className="metric-value">{formatBytes(free)}</div>
          <div className="metric-note">{formatPercent(100 - usedPercent)} of total capacity</div>
        </Card>
        <Card className="metric-card" style={{ ["--metric-accent" as string]: "var(--amber)" }}>
          <div className="metric-top"><span>Used space</span><span className="metric-icon"><HardDrive size={16} /></span></div>
          <div className="metric-value">{formatBytes(used)}</div>
          <div className="metric-note">Not a reclaimable-space estimate</div>
        </Card>
        <Card className="metric-card" style={{ ["--metric-accent" as string]: "var(--accent-strong)" }}>
          <div className="metric-top"><span>Usage</span><span className="metric-icon"><HardDrive size={16} /></span></div>
          <div className="metric-value">{formatPercent(usedPercent)}</div>
          <div className="metric-note">Windows volume capacity</div>
        </Card>
      </div>

      <Card>
        <CardHeader
          title="Storage explorer"
          subtitle="Read-only, user-selected folder analysis. No cleanup action exists in this milestone."
          action={
            <Button onClick={() => void storage.chooseRoot()} disabled={storage.busy || running}>
              <FolderOpen size={14} /> {storage.root ? "Change folder" : "Choose folder"}
            </Button>
          }
        />
        <CardBody>
          {!storage.root ? (
            <EmptyState
              icon={<FolderOpen size={20} />}
              title="Choose a folder to understand its storage"
              action={<Button variant="primary" onClick={() => void storage.chooseRoot()} disabled={storage.busy}>Choose folder</Button>}
            >
              StackBroom scans only the folder you select. It never follows junctions, symlinks or mounted boundaries,
              and it does not delete or modify files.
            </EmptyState>
          ) : (
            <div style={{ display: "grid", gap: 14 }}>
              <div className="toolbar" style={{ marginBottom: 0 }}>
                <div>
                  <span className="muted" style={{ fontSize: 11 }}>Selected root</span>
                  <div className="mono" style={{ color: "var(--text)", overflowWrap: "anywhere" }}>{storage.root.displayPath}</div>
                </div>
                <div className="toolbar" style={{ marginLeft: "auto", marginBottom: 0 }}>
                  <label className="checkbox-label">
                    <input type="checkbox" checked={options.includeHidden} onChange={(event) => setOptions({ ...options, includeHidden: event.target.checked })} />
                    Hidden
                  </label>
                  <label className="checkbox-label">
                    <input type="checkbox" checked={options.includeSystem} onChange={(event) => setOptions({ ...options, includeSystem: event.target.checked })} />
                    System
                  </label>
                  <label>
                    <span className="sr-only">Maximum depth</span>
                    <select className="select" value={options.maxDepth} onChange={(event) => setOptions({ ...options, maxDepth: Number(event.target.value) })}>
                      <option value={16}>16 levels</option>
                      <option value={32}>32 levels</option>
                      <option value={64}>64 levels</option>
                      <option value={128}>128 levels</option>
                    </select>
                  </label>
                  {running ? (
                    <Button onClick={() => void storage.cancel()}><Pause size={14} /> Cancel scan</Button>
                  ) : (
                    <Button variant="primary" onClick={() => void storage.startScan(options)} disabled={storage.busy}>
                      <Play size={14} /> Start scan
                    </Button>
                  )}
                </div>
              </div>


              {storage.root.wholeDrive && (
                <div className="banner" style={{ borderColor: "var(--amber)" }} role="status">
                  <AlertTriangle size={16} style={{ color: "var(--amber)" }} />
                  <div><strong>Whole-drive scan.</strong> This can take much longer and may encounter protected system folders. A specific folder is usually faster and easier to understand.</div>
                </div>
              )}

              {storage.error && (
                <div className="banner" style={{ borderColor: "var(--red)" }} role="alert">
                  <AlertTriangle size={16} style={{ color: "var(--red)" }} />
                  <div><strong>Storage analysis error.</strong> {storage.error}</div>
                </div>
              )}

              {running && storage.view && (
                <div className="storage-progress" role="status" aria-live="polite">
                  <div>
                    <strong>{storage.view.progress.phase || "starting"}</strong>
                    <span className="muted"> · {storage.view.progress.items.toLocaleString()} items · {formatBytes(storage.view.progress.bytes)} observed</span>
                  </div>
                  <span className="muted">
                    {storage.view.progress.errors > 0 ? `${storage.view.progress.errors} skipped/errors` : "No skipped paths reported"}
                  </span>
                </div>
              )}

              {result && (
                <div style={{ display: "grid", gap: 14 }}>
                  <div className="grid storage-summary">
                    <div><span>Measured size</span><strong>{formatBytes(result.totalLogical)}</strong></div>
                    <div><span>Allocated</span><strong>{formatBytes(result.totalAllocated)}</strong></div>
                    <div><span>Files</span><strong>{result.totalFiles.toLocaleString()}</strong></div>
                    <div><span>Folders</span><strong>{result.totalDirs.toLocaleString()}</strong></div>
                    <div><span>Skipped</span><strong>{result.skippedCount.toLocaleString()}</strong></div>
                    <div><span>Scan time</span><strong>{(result.durationMs / 1000).toFixed(2)} s</strong></div>
                  </div>

                  {result.partial && (
                    <div className="banner" style={{ borderColor: "var(--amber)" }} role="alert">
                      <AlertTriangle size={16} style={{ color: "var(--amber)" }} />
                      <div>
                        <strong>Partial result.</strong> {result.partialReason || "The scan did not complete."} Totals describe only the paths that were read.
                      </div>
                    </div>
                  )}
                  {result.warnings?.length ? (
                    <div className="banner" role="status">
                      <ShieldCheck size={16} />
                      <div>{result.warnings.join(" ")}</div>
                    </div>
                  ) : null}

                  <StorageCoveragePanel view={result} />

                  <div className="storage-breadcrumb" aria-label="Storage path">
                    <button type="button" className="crumb" onClick={() => void storage.goToTrail(-1)}>{result.rootLabel}</button>
                    {storage.trail.map((node, index) => (
                      <span key={node.id} className="crumb-wrap">
                        <ChevronRight size={12} aria-hidden="true" />
                        <button type="button" className="crumb" onClick={() => void storage.goToTrail(index)}>{node.name}</button>
                      </span>
                    ))}
                  </div>

                  <div className="toolbar" style={{ marginBottom: 0 }}>
                    <label className="search-wrap">
                      <span className="sr-only">Filter current folder</span>
                      <Search className="search-icon" aria-hidden="true" />
                      <input className="input" value={searchTerm} onChange={(event) => setSearchTerm(event.target.value)} placeholder="Search the complete indexed folder" />
                    </label>
                    <Badge>{storage.children.length}/{storage.totalChildren} children loaded</Badge>
                    {storage.nextCursor ? <Badge tone="amber">More indexed children available</Badge> : null}
                    {storage.nextCursor ? (
                      <Button onClick={() => void storage.nextPage()}>Load more</Button>
                    ) : null}
                  </div>

                  <StorageTreemap
                    nodes={visibleChildren}
                    totalBytes={storage.query ? storage.totalLogical : (storage.current?.logicalBytes ?? result.rootLogical)}
                    onOpen={(node) => void storage.openNode(node)}
                  />
                  <StorageChildrenTable nodes={storage.children} onOpen={(node) => void storage.openNode(node)} />
                  <StorageEvidence view={result} onOpen={(node) => void storage.openNode(node)} />
                </div>
              )}
            </div>
          )}
        </CardBody>
      </Card>
    </div>
  );
}


function StorageCoveragePanel({ view }: { view: StorageScanView }) {
  const coverage = view.coverage;
  const duplicates = view.duplicateCoverage;
  return (
    <div className="storage-coverage" role="status" aria-live="polite">
      <div>
        <Badge tone={coverage.exhaustive ? "teal" : "amber"}>
          {coverage.exhaustive ? "Exhaustive traversal" : "Not exhaustive"}
        </Badge>
        <span className="muted"> Coverage describes what this scan could read, not what exists on the volume.</span>
      </div>
      <div className="storage-coverage-facts">
        {coverage.hiddenExcluded > 0 ? <span>{coverage.hiddenExcluded} hidden excluded</span> : null}
        {coverage.systemExcluded > 0 ? <span>{coverage.systemExcluded} system excluded</span> : null}
        {coverage.reparseExcluded > 0 ? <span>{coverage.reparseExcluded} reparse points not followed</span> : null}
        {coverage.accessDenied > 0 ? <span>{coverage.accessDenied} access errors</span> : null}
        {coverage.depthLimit ? <span>Depth limit reached</span> : null}
        {coverage.entryLimit ? <span>Entry limit reached</span> : null}
        {coverage.cancelled ? <span>Scan cancelled</span> : null}
        {coverage.timedOut ? <span>Scan timed out</span> : null}
      </div>
      <div className="storage-coverage-facts">
        <span>Duplicate analysis: {duplicates.enumerated} files enumerated</span>
        <span>{duplicates.eligible} eligible</span>
        <span>{duplicates.policySkipped} policy-skipped</span>
        <span>{duplicates.candidateConsidered}/{duplicates.candidateCap} candidates considered</span>
        <span>{duplicates.hashAttempted} hash attempts</span>
        <span>{duplicates.verified} verified</span>
        {duplicates.hashFailed > 0 ? <span>{duplicates.hashFailed} hash failures</span> : null}
        {duplicates.changed > 0 ? <span>{duplicates.changed} changed during verification</span> : null}
        {duplicates.truncated ? <strong>Duplicate analysis incomplete</strong> : <span>Duplicate analysis complete</span>}
      </div>
    </div>
  );
}

function StorageChildrenTable({ nodes, onOpen }: { nodes: StorageNode[]; onOpen: (node: StorageNode) => void }) {
  if (nodes.length === 0) {
    return <p className="muted">No child entries are available for this folder.</p>;
  }
  return (
    <div className="table-wrap" tabIndex={0} role="region" aria-label="Storage children table">
      <table className="table">
        <caption className="sr-only">Bounded child entries for the current folder</caption>
        <thead>
          <tr>
            <th>Name</th><th>Type</th><th>Logical size</th><th>Allocated</th><th>Files</th><th>Folders</th><th>Flags</th>
          </tr>
        </thead>
        <tbody>
          {nodes.map((node) => {
            const clickable = node.kind === "dir";
            const links = node.linkCount ?? 0;
            return (
              <tr key={node.id}>
                <td className="name">
                  {clickable ? (
                    <button type="button" className="row-detail-link" onClick={() => onOpen(node)} aria-label={`Open folder ${node.name}`}>
                      {node.name}
                    </button>
                  ) : node.name}
                </td>
                <td><Badge>{node.kind}</Badge></td>
                <td className="num">{formatBytes(node.logicalBytes)}</td>
                <td className="num">{formatBytes(node.allocatedBytes)}</td>
                <td className="num">{node.fileCount.toLocaleString()}</td>
                <td className="num">{node.dirCount.toLocaleString()}</td>
                <td className="muted">
                  {node.isPlaceholder ? "Placeholder " : ""}
                  {node.isReparse ? "Reparse " : ""}
                  {node.isSparse ? "Sparse " : ""}
                  {links > 1 ? `Hard link x${links} ` : ""}
                  {node.truncated ? "Truncated" : ""}
                  {!node.isPlaceholder && !node.isReparse && !node.isSparse && links <= 1 && !node.truncated ? "—" : ""}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function StorageEvidence({ view, onOpen }: { view: StorageScanView; onOpen: (node: StorageNode) => void }) {
  return (
    <div className="grid storage-evidence">
      <Card>
        <CardHeader title="Largest folders" subtitle="Ranked by measured logical content. Hard links are counted once." />
        <CardBody>
          {(view.largestFolders ?? []).length === 0 ? <p className="muted">No folders measured.</p> : (
            <ul className="storage-list">
              {(view.largestFolders ?? []).map((node) => (
                <li key={node.id}>
                  <button type="button" className="storage-list-link" onClick={() => onOpen(node)}>
                    <span>{node.name}</span><span>{formatBytes(node.logicalBytes)}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </CardBody>
      </Card>

      <Card>
        <CardHeader title="File types" subtitle="Measured extension totals. Unknown extensions remain explicit." />
        <CardBody>
          {(view.types ?? []).length === 0 ? <p className="muted">No file types measured.</p> : (
            <div className="table-wrap" tabIndex={0} role="region" aria-label="File type table">
              <table className="table">
                <thead><tr><th>Extension</th><th>Files</th><th>Logical size</th></tr></thead>
                <tbody>
                  {(view.types ?? []).map((stat) => (
                    <tr key={stat.extension}><td className="mono">{stat.extension}</td><td className="num">{stat.files}</td><td className="num">{formatBytes(stat.logicalBytes)}</td></tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardBody>
      </Card>


      <Card>
        <CardHeader title="Largest files" subtitle="Bounded to the largest measured entries. Placeholders are labelled." />
        <CardBody>
          {(view.largestFiles ?? []).length === 0 ? <p className="muted">No files measured.</p> : (
            <StorageFileTable files={view.largestFiles ?? []} />
          )}
        </CardBody>
      </Card>

      <Card>
        <CardHeader
          title="Verified duplicate candidates"
          subtitle="Complete SHA-256 verification. Evidence only; no deletion action exists."
        />
        <CardBody>
          {(view.duplicates ?? []).length === 0 ? (
            <p className="muted">No duplicate candidates met the size and content checks.</p>
          ) : (
            <div className="storage-duplicates">
              {(view.duplicates ?? []).map((group) => <DuplicateCard key={group.id} group={group} />)}
            </div>
          )}
          <div className="banner" style={{ marginTop: 10 }}>
            <ShieldCheck size={16} />
            <div>Duplicate detection proves identical content only. It does not prove that a file is unnecessary.</div>
          </div>
        </CardBody>
      </Card>
    </div>
  );
}

function StorageFileTable({ files }: { files: StorageFile[] }) {
  return (
    <div className="table-wrap" tabIndex={0} role="region" aria-label="Large file table">
      <table className="table">
        <thead><tr><th>Name</th><th>Logical</th><th>Allocated</th><th>Links</th><th>State</th></tr></thead>
        <tbody>
          {files.map((file) => (
            <tr key={file.nodeId}>
              <td className="name" title={file.relativePath}>{file.relativePath}</td>
              <td className="num">{formatBytes(file.logicalBytes)}</td>
              <td className="num">{file.allocatedKnown ? formatBytes(file.allocatedBytes) : "Unknown"}</td>
              <td className="num">{file.linkCount || 1}</td>
              <td>{file.isPlaceholder ? <Badge tone="amber">Placeholder</Badge> : <Badge>{file.extension}</Badge>}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function DuplicateCard({ group }: { group: StorageDuplicateGroup }) {
  const files = group.files ?? [];
  return (
    <div className="storage-duplicate">
      <div className="storage-duplicate-head">
        <div><strong>{formatBytes(group.size)}</strong> <Badge tone="teal">{files.length} files</Badge></div>
        <span className="mono muted" title={group.hash}>{group.hash.slice(0, 20)}…</span>
      </div>
      <p className="muted" style={{ margin: "3px 0 8px" }}>{group.note}</p>
      <ul className="storage-duplicate-files">
        {files.map((file) => (
          <li key={file.nodeId}>
            <span className="mono">{file.relativePath}</span>
            {(file.linkCount ?? 0) > 1 ? <Badge tone="amber">link x{file.linkCount}</Badge> : null}
          </li>
        ))}
      </ul>
    </div>
  );
}

