import { useEffect, useMemo, useState } from "react";
import { AlertTriangle, ChevronDown, FolderSearch, ShieldCheck, Sparkles } from "lucide-react";
import type { CleanupCandidate, CleanupPreviewView, CleanupRule } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { formatBytes, formatWhen } from "../lib/format";
import { Badge, Button, Card, CardBody, CardHeader, EmptyState } from "../components/ui";
import { useCleanup } from "../hooks/useCleanup";

export function CleaningPage() {
  const cleanup = useCleanup();
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const candidates = cleanup.discovery?.candidates ?? [];
  const selected = useMemo(() => new Set(selectedIDs), [selectedIDs]);

  useEffect(() => {
    void cleanup.loadRules();
  }, [cleanup.loadRules]);

  useEffect(() => {
    if (!cleanup.discovery || cleanup.discovery.status === "running") return;
    setSelectedIDs((cleanup.discovery.candidates ?? []).map((candidate) => candidate.id));
  }, [cleanup.discovery]);

  const toggle = (candidate: CleanupCandidate) => {
    setSelectedIDs((current) => current.includes(candidate.id)
      ? current.filter((id) => id !== candidate.id)
      : [...current, candidate.id]);
  };

  return (
    <div className="grid" style={{ gap: 16 }}>
      <Card>
        <CardHeader
          title="Cleaning preview"
          subtitle="Curated, read-only discovery. No file is deleted, moved, renamed or modified in this milestone."
          action={<Badge tone="amber">Preview only — cleaning not yet available</Badge>}
        />
        <CardBody>
          <div className="banner" style={{ borderColor: "var(--amber)" }}>
            <ShieldCheck size={16} style={{ color: "var(--amber)" }} />
            <div>
              <strong>Nothing here cleans your disk yet.</strong> Analyse a curated category, review exactly what is eligible and build an expiring preview. Execution is intentionally not implemented.
            </div>
          </div>
          {cleanup.error && (
            <div className="banner" style={{ marginTop: 10, borderColor: "var(--red)" }} role="alert">
              <AlertTriangle size={16} style={{ color: "var(--red)" }} />
              <div>{cleanup.error}</div>
            </div>
          )}
        </CardBody>
      </Card>

      <div className="grid cleanup-rules">
        {cleanup.rules.map((rule) => (
          <RuleCard
            key={rule.id}
            rule={rule}
            active={cleanup.rule?.id === rule.id}
            busy={cleanup.busy}
            onAnalyse={() => void cleanup.start(rule)}
          />
        ))}
        {cleanup.rules.length === 0 && (
          <Card><CardBody><EmptyState icon={<Sparkles size={20} />} title="Curated rules are loading">The cleanup catalogue is read from the local application.</EmptyState></CardBody></Card>
        )}
      </div>

      {cleanup.discovery && (
        <Card>
          <CardHeader
            title={`Discovery: ${cleanup.discovery.rule.title}`}
            subtitle={`Scope: ${cleanup.discovery.rule.scopeDisplay} · rule ${cleanup.discovery.rule.id} v${cleanup.discovery.rule.version}`}
            action={cleanup.discovery.status === "running"
              ? <Button onClick={() => void cleanup.cancel()}>Cancel discovery</Button>
              : <Badge tone={cleanup.discovery.partial ? "amber" : "teal"}>{cleanup.discovery.status}</Badge>}
          />
          <CardBody>
            {cleanup.discovery.status === "running" ? (
              <div className="storage-progress" role="status" aria-live="polite">
                <div><strong>{cleanup.discovery.progress.phase || "starting"}</strong><span className="muted"> · {cleanup.discovery.progress.items.toLocaleString()} items · {formatBytes(cleanup.discovery.progress.bytes)} observed</span></div>
                <span className="muted">{cleanup.discovery.progress.errors} skipped/errors</span>
              </div>
            ) : (
              <div style={{ display: "grid", gap: 12 }}>
                <div className="grid cleanup-summary">
                  <Summary label="Files found" value={cleanup.discovery.scannedFiles.toLocaleString()} />
                  <Summary label="Eligible" value={cleanup.discovery.eligible.toLocaleString()} />
                  <Summary label="Known size" value={formatBytes(cleanup.discovery.totalLogical)} />
                  <Summary label="Allocated" value={formatBytes(cleanup.discovery.totalAllocated)} />
                  <Summary label="Excluded" value={cleanup.discovery.excludedTotal.toLocaleString()} />
                </div>
                {cleanup.discovery.partial && (
                  <div className="banner" style={{ borderColor: "var(--amber)" }} role="alert">
                    <AlertTriangle size={16} style={{ color: "var(--amber)" }} />
                    <div><strong>Partial discovery.</strong> {cleanup.discovery.partialReason || "Not every path could be inspected."}</div>
                  </div>
                )}
                <div className="cleanup-actions">
                  <Button
                    variant="primary"
                    disabled={cleanup.busy || selectedIDs.length === 0}
                    onClick={() => void cleanup.createPreview(selectedIDs)}
                  >
                    Preview selected cleanup
                  </Button>
                  <span className="muted">{selectedIDs.length} selected · preview only</span>
                </div>
                {candidates.length === 0 ? (
                  <EmptyState icon={<FolderSearch size={20} />} title="No eligible candidates">The rule found no files that met its age, extension and identity checks.</EmptyState>
                ) : (
                  <div className="table-wrap" tabIndex={0} role="region" aria-label="Cleanup candidate table">
                    <table className="table">
                      <caption className="sr-only">Eligible cleanup candidates. Selection is for a preview only.</caption>
                      <thead><tr><th>Select</th><th>File</th><th>Logical</th><th>Allocated</th><th>Modified</th><th>Evidence</th></tr></thead>
                      <tbody>
                        {candidates.map((candidate) => (
                          <tr key={candidate.id}>
                            <td><input type="checkbox" checked={selected.has(candidate.id)} onChange={() => toggle(candidate)} aria-label={`Select ${candidate.relativePath}`} /></td>
                            <td className="name" title={candidate.relativePath}>{candidate.relativePath}</td>
                            <td className="num">{formatBytes(candidate.logicalBytes)}</td>
                            <td className="num">{candidate.allocatedKnown ? formatBytes(candidate.allocatedBytes) : "Unknown"}</td>
                            <td>{formatWhen(candidate.modifiedAt)}</td>
                            <td>{candidate.uncertain ? <Badge tone="amber">Uncertain</Badge> : <span className="muted">Verified at discovery</span>}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
                {(cleanup.discovery.excluded ?? []).length > 0 && (
                  <details className="cleanup-exclusions">
                    <summary><ChevronDown size={14} /> Excluded or uncertain targets ({cleanup.discovery.excludedTotal})</summary>
                    <ul>
                      {(cleanup.discovery.excluded ?? []).map((exclusion) => (
                        <li key={`${exclusion.relativePath}-${exclusion.reason}`}><span>{exclusion.relativePath}</span><span className="muted">{exclusion.reason}</span></li>
                      ))}
                    </ul>
                  </details>
                )}
              </div>
            )}
          </CardBody>
        </Card>
      )}

      {cleanup.preview && <PreviewCard preview={cleanup.preview} onClear={cleanup.clearPreview} />}
    </div>
  );
}


function RuleCard({ rule, active, busy, onAnalyse }: { rule: CleanupRule; active: boolean; busy: boolean; onAnalyse: () => void }) {
  return (
    <Card className={active ? "cleanup-rule active" : "cleanup-rule"}>
      <CardBody>
        <div className="cleanup-rule-head">
          <div><Badge tone="teal">{rule.category}</Badge><h3 className="card-title" style={{ marginTop: 6 }}>{rule.title}</h3></div>
          <Badge>{rule.recoverable === "uncertain" ? "Recovery uncertain" : rule.recoverable}</Badge>
        </div>
        <p className="muted">{rule.description}</p>
        <dl className="cleanup-facts">
          <div><dt>Scope</dt><dd className="mono">{rule.scopeDisplay}</dd></div>
          <div><dt>Owner</dt><dd>{rule.owner}</dd></div>
          <div><dt>Minimum age</dt><dd>{rule.minAgeDays} days</dd></div>
          <div><dt>Future deletion mode</dt><dd>{rule.deletionMode === "recycle" ? "Recycle Bin where supported" : rule.deletionMode}</dd></div>
        </dl>
        <details><summary>Why this rule is considered safe</summary><p className="muted">{rule.whySafe}</p><ul>{(rule.evidence ?? []).map((item) => <li key={item}>{item}</li>)}</ul></details>
        <details><summary>Risks and prerequisites</summary><ul>{(rule.risks ?? []).map((item) => <li key={item}>{item}</li>)}</ul>{rule.requiresClosed?.length ? <p className="muted">Close first: {rule.requiresClosed.join("; ")}</p> : null}</details>
        <div className="cleanup-rule-action"><Button variant="primary" onClick={onAnalyse} disabled={busy}>Analyse category</Button></div>
      </CardBody>
    </Card>
  );
}

function PreviewCard({ preview, onClear }: { preview: CleanupPreviewView; onClear: () => void }) {
  return (
    <Card>
      <CardHeader title="Exact cleanup preview" subtitle={`Rule ${preview.ruleId} v${preview.ruleVersion} · expires ${formatWhen(preview.expiresAt)}`} action={<Badge tone="amber">No execution</Badge>} />
      <CardBody>
        <div className="grid cleanup-summary">
          <Summary label="Selected files" value={preview.fileCount.toLocaleString()} />
          <Summary label="Logical size" value={formatBytes(preview.totalLogical)} />
          <Summary label="Allocated size" value={formatBytes(preview.totalAllocated)} />
          <Summary label="Recoverable estimate" value={preview.recoverableKnown ? formatBytes(preview.recoverableBytes) : "Unknown"} />
          <Summary label="Future mode" value={preview.deletionMode === "recycle" ? "Recycle where supported" : preview.deletionMode} />
        </div>
        <div className="banner" style={{ marginTop: 10, borderColor: "var(--amber)" }} role="status">
          <AlertTriangle size={16} style={{ color: "var(--amber)" }} />
          <div><strong>Preview only.</strong> {preview.recoveryNote} No cleanup method exists in this milestone.</div>
        </div>
        {(preview.excluded ?? []).length > 0 && <p className="muted">{(preview.excluded ?? []).length} selected target(s) changed and were excluded from this preview.</p>}
        <div className="cleanup-actions"><Button variant="ghost" onClick={onClear}>Close preview</Button></div>
      </CardBody>
    </Card>
  );
}

function Summary({ label, value }: { label: string; value: string }) {
  return <div><span>{label}</span><strong>{value}</strong></div>;
}

