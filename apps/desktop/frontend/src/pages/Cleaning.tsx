import { useEffect, useMemo, useState } from "react";
import type { CleanupExecutionView } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { AlertTriangle, ChevronDown, FolderSearch, ShieldCheck, Sparkles } from "lucide-react";
import type { CleanupCandidate, CleanupPreviewView, CleanupRule } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { formatBytes, formatExpiry, formatWhen } from "../lib/format";
import { cleanupError, cleanupReason, cleanupStatus } from "../lib/cleanup";
import { Badge, Button, Card, CardBody, CardHeader, EmptyState } from "../components/ui";
import { useCleanup } from "../hooks/useCleanup";

const steps = ["Find files", "Choose files", "Review", "Clean", "Results"];

export function CleaningPage() {
  const cleanup = useCleanup();
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const candidates = cleanup.discovery?.candidates ?? [];
  const selected = useMemo(() => new Set(selectedIDs), [selectedIDs]);
  const activeStep = cleanup.preview ? 2 : cleanup.discovery ? 1 : 0;

  useEffect(() => {
    void cleanup.loadRules();
  }, [cleanup.loadRules]);

  useEffect(() => {
    if (!cleanup.discovery) return;
    setSelectedIDs([]);
  }, [cleanup.discovery?.discoveryId]);

  const toggle = (candidate: CleanupCandidate) => {
    setSelectedIDs((current) => current.includes(candidate.id)
      ? current.filter((id) => id !== candidate.id)
      : [...current, candidate.id]);
  };

  return (
    <div className="grid" style={{ gap: 16 }}>
      <Card>
        <CardHeader
          title="Cleaning"
          subtitle="Find old files, choose what to review, and decide what to clean."
          action={<Badge tone="amber">Cleaning not available yet</Badge>}
        />
        <CardBody>
          <ol className="cleanup-steps" aria-label="Cleanup steps">
            {steps.map((step, index) => (
              <li
                key={step}
                className={index === activeStep ? "active" : index < activeStep ? "done" : ""}
                aria-current={index === activeStep ? "step" : undefined}
              >
                <span className="cleanup-step-number">{index + 1}</span>
                <span>{step}</span>
              </li>
            ))}
          </ol>
          <div className="banner" style={{ borderColor: "var(--amber)" }}>
            <ShieldCheck size={16} style={{ color: "var(--amber)" }} aria-hidden="true" />
            <div>
              <strong>Cleaning isn't available in this build.</strong> You can find and review files, but nothing will be cleaned or removed yet.
            </div>
          </div>
          {cleanup.error && (
            <div className="banner" style={{ marginTop: 10, borderColor: "var(--red)" }} role="alert">
              <AlertTriangle size={16} style={{ color: "var(--red)" }} aria-hidden="true" />
              <div>{cleanupError(cleanup.error)}</div>
            </div>
          )}
          <details className="cleanup-disclosure" style={{ marginTop: 10 }}>
            <summary><ChevronDown size={14} aria-hidden="true" /> Recycle Bin or permanent deletion?</summary>
            <p className="cleanup-note">
              <strong>Recycle Bin</strong> keeps a file recoverable until you empty it.
              <strong> Permanent deletion</strong> removes it immediately and cannot be undone.
              This build does not offer permanent deletion.
            </p>
          </details>
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
          <Card><CardBody><EmptyState icon={<Sparkles size={20} />} title="Loading cleanup options">Your cleanup options are read from this app.</EmptyState></CardBody></Card>
        )}
      </div>

      {cleanup.discovery && (
        <Card>
          <CardHeader
            title={`Files found: ${cleanup.discovery.rule.title}`}
            subtitle={`Where: ${cleanup.discovery.rule.scopeDisplay}`}
            action={cleanup.discovery.status === "running"
              ? <Button onClick={() => void cleanup.cancel()}>Stop searching</Button>
              : <Badge tone={cleanup.discovery.partial ? "amber" : "teal"}>{cleanupStatus(cleanup.discovery.status)}</Badge>}
          />
          <CardBody>
            {cleanup.discovery.status === "running" ? (
              <div className="storage-progress" role="status" aria-live="polite">
                <div><strong>Searching…</strong><span className="muted"> · {cleanup.discovery.progress.items.toLocaleString()} files checked · {formatBytes(cleanup.discovery.progress.bytes)} found</span></div>
                <span className="muted">{cleanup.discovery.progress.errors} skipped</span>
              </div>
            ) : (
              <div style={{ display: "grid", gap: 12 }}>
                <div className="grid cleanup-summary">
                  <Summary label="Files checked" value={cleanup.discovery.scannedFiles.toLocaleString()} />
                  <Summary label="Ready to choose" value={cleanup.discovery.eligible.toLocaleString()} />
                  <Summary label="Size" value={formatBytes(cleanup.discovery.totalLogical)} />
                  <Summary label="Space used" value={knownTotal(cleanup.discovery.totalAllocated, cleanup.discovery.allocatedUnknown)} />
                  <Summary label="Skipped" value={cleanup.discovery.excludedTotal.toLocaleString()} />
                </div>
                {cleanup.discovery.partial && (
                  <div className="banner" style={{ borderColor: "var(--amber)" }} role="alert">
                    <AlertTriangle size={16} style={{ color: "var(--amber)" }} aria-hidden="true" />
                    <div><strong>Some folders couldn't be checked.</strong> The list may be incomplete.</div>
                  </div>
                )}
                <div className="cleanup-actions">
                  <Button
                    variant="primary"
                    disabled={cleanup.busy || selectedIDs.length === 0}
                    onClick={() => void cleanup.createPreview(selectedIDs)}
                  >
                    Review selected files
                  </Button>
                  <Button variant="ghost" onClick={() => setSelectedIDs(candidates.map((candidate) => candidate.id))} disabled={candidates.length === 0}>
                    Select all
                  </Button>
                  <Button variant="ghost" onClick={() => setSelectedIDs([])} disabled={selectedIDs.length === 0}>
                    Clear selection
                  </Button>
                  <span className="muted">{selectedIDs.length} selected</span>
                </div>
                {candidates.length === 0 ? (
                  <EmptyState icon={<FolderSearch size={20} />} title="No old files found">Nothing matched the age and file checks for this option.</EmptyState>
                ) : (
                  <div className="table-wrap" tabIndex={0} role="region" aria-label="Files you can choose">
                    <table className="table">
                      <caption className="sr-only">Files you can choose to review.</caption>
                      <thead><tr><th>Choose</th><th>File</th><th>Size</th><th>Space used</th><th>Last changed</th><th>Status</th></tr></thead>
                      <tbody>
                        {candidates.map((candidate) => (
                          <tr key={candidate.id}>
                            <td><input type="checkbox" checked={selected.has(candidate.id)} onChange={() => toggle(candidate)} aria-label={`Select ${candidate.relativePath}`} /></td>
                            <td className="name" title={candidate.relativePath}>{candidate.relativePath}</td>
                            <td className="num">{formatBytes(candidate.logicalBytes)}</td>
                            <td className="num">{candidate.allocatedKnown ? formatBytes(candidate.allocatedBytes) : "Unknown"}</td>
                            <td>{formatWhen(candidate.modifiedAt)}</td>
                            <td>{candidate.uncertain ? <Badge tone="amber">Needs checking</Badge> : <span className="muted">Checked</span>}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
                {candidates.some((candidate) => candidate.uncertain) && (
                  <p className="muted">Files marked “Needs checking” couldn't be fully checked. Find files again before cleaning.</p>
                )}
                {(cleanup.discovery.excluded ?? []).length > 0 && (
                  <details className="cleanup-exclusions">
                    <summary><ChevronDown size={14} aria-hidden="true" /> Files skipped ({cleanup.discovery.excludedTotal})</summary>
                    <ul>
                      {(cleanup.discovery.excluded ?? []).map((exclusion) => (
                        <li key={`${exclusion.relativePath}-${exclusion.reason}`}><span>{exclusion.relativePath}</span><span className="muted">{cleanupReason(exclusion.reason)}</span></li>
                      ))}
                    </ul>
                  </details>
                )}
              </div>
            )}
          </CardBody>
        </Card>
      )}

      {cleanup.preview && (
        <PreviewCard preview={cleanup.preview} onClear={cleanup.clearPreview} onConfirm={cleanup.confirmPermanent} />
      )}
    </div>
  );
}


function RuleCard({ rule, active, busy, onAnalyse }: { rule: CleanupRule; active: boolean; busy: boolean; onAnalyse: () => void }) {
  return (
    <Card className={active ? "cleanup-rule active" : "cleanup-rule"}>
      <CardBody>
        <div className="cleanup-rule-head">
          <div><Badge tone="teal">{rule.category}</Badge><h3 className="card-title" style={{ marginTop: 6 }}>{rule.title}</h3></div>
          <Badge tone={rule.permanentAvailable ? "red" : rule.recoverable === "uncertain" ? "amber" : "neutral"}>
            {rule.permanentAvailable ? "Permanent deletion" : rule.recoverable === "uncertain" ? "Recycle Bin may vary" : "Recycle Bin"}
          </Badge>
        </div>
        <p className="muted">{rule.description}</p>
        <dl className="cleanup-facts">
          <div><dt>Where</dt><dd className="mono">{rule.scopeDisplay}</dd></div>
          <div><dt>Applies to</dt><dd>{rule.owner}</dd></div>
          <div><dt>Older than</dt><dd>{rule.minAgeDays} days</dd></div>
          <div><dt>How it would be cleaned</dt><dd>{rule.permanentAvailable ? "Permanent deletion" : rule.deletionMode === "recycle" ? "Move to Recycle Bin" : "Not available"}</dd></div>
        </dl>
        <details className="cleanup-disclosure">
          <summary><ChevronDown size={14} aria-hidden="true" /> What's included</summary>
          <p className="muted">{rule.whySafe}</p>
          <ul>{(rule.evidence ?? []).map((item) => <li key={item}>{item}</li>)}</ul>
        </details>
        <details className="cleanup-disclosure">
          <summary><ChevronDown size={14} aria-hidden="true" /> What to know</summary>
          <ul>{(rule.risks ?? []).map((item) => <li key={item}>{item}</li>)}</ul>
          {rule.requiresClosed?.length ? <p className="muted">Close these apps first: {rule.requiresClosed.join("; ")}</p> : null}
        </details>
        <div className="cleanup-rule-action"><Button variant="primary" onClick={onAnalyse} disabled={busy}>Find files</Button></div>
      </CardBody>
    </Card>
  );
}

function knownTotal(value: number, unknownCount: number): string {
  if (unknownCount === 0) return formatBytes(value);
  if (value > 0) return `At least ${formatBytes(value)}`;
  return "Unknown";
}

function recoveryEstimate(preview: CleanupPreviewView): { value: string; note: string } {
  const unknown = preview.allocatedUnknown > 0
    ? `${preview.allocatedUnknown.toLocaleString()} file${preview.allocatedUnknown === 1 ? "" : "s"} couldn't be measured.`
    : "";
  if (preview.recoverableKnown) return { value: formatBytes(preview.recoverableBytes), note: "" };
  if (preview.recoverableBytes > 0) return { value: `At least ${formatBytes(preview.recoverableBytes)}`, note: unknown };
  return { value: "Unknown", note: unknown };
}

function PreviewCard({
  preview,
  onClear,
  onConfirm,
}: {
  preview: CleanupPreviewView;
  onClear: () => void;
  onConfirm: (previewID: string) => Promise<CleanupExecutionView | null>;
}) {
  const estimate = recoveryEstimate(preview);
  const expiry = formatExpiry(preview.expiresAt);
  const permanent = preview.permanentAvailable === true;
  const [confirming, setConfirming] = useState(false);
  const [executing, setExecuting] = useState(false);
  const [result, setResult] = useState<CleanupExecutionView | null>(null);
  const cleanupMethod = permanent ? "Permanent deletion" : preview.deletionMode === "recycle" ? "Move to Recycle Bin" : "Not available";

  const confirmPermanent = async () => {
    if (!permanent || executing) return;
    setExecuting(true);
    const next = await onConfirm(preview.previewId);
    if (next) setResult(next);
    setConfirming(false);
    setExecuting(false);
  };

  return (
    <Card>
      <CardHeader
        title="Review selected files"
        subtitle={expiry === "Expired" ? "This review expired. Find files again." : `Expires ${expiry}`}
        action={permanent ? <Badge tone="red">Permanent deletion</Badge> : <Badge tone="amber">Nothing changed yet</Badge>}
      />
      <CardBody>
        <div className="grid cleanup-summary">
          <Summary label="Files selected" value={preview.fileCount.toLocaleString()} />
          <Summary label="Size" value={formatBytes(preview.totalLogical)} />
          <Summary label="Space used" value={knownTotal(preview.totalAllocated, preview.allocatedUnknown)} />
          <Summary
            label="Space in selected files"
            value={estimate.value}
            note={estimate.note || (permanent ? "This is an estimate; disk space may not be reclaimed immediately." : preview.deletionMode === "recycle" ? "The Recycle Bin may keep this space until you empty it." : "")}
          />
          <Summary label="Cleanup method" value={cleanupMethod} />
        </div>
        <div
          className={permanent ? "banner banner-error" : "banner"}
          style={{ marginTop: 10, borderColor: permanent ? "var(--red)" : "var(--amber)" }}
          role={permanent ? "alert" : "status"}
        >
          <AlertTriangle size={16} style={{ color: permanent ? "var(--red)" : "var(--amber)" }} aria-hidden="true" />
          <div>
            {permanent ? (
              <>
                <strong>Permanent deletion cannot be undone.</strong> The selected files will be deleted. StackBroom cannot restore them from the Recycle Bin.
                {preview.recoveryNote ? <p className="cleanup-note">{preview.recoveryNote}</p> : null}
              </>
            ) : (
              <>
                <strong>Nothing has been changed.</strong> Cleaning isn't available in this build, so you can review these files but not clean them yet.
                {preview.recoveryNote ? <p className="cleanup-note">{preview.recoveryNote}</p> : null}
              </>
            )}
          </div>
        </div>
        {(preview.excluded ?? []).length > 0 && (
          <p className="muted">Some files changed since you selected them and aren't in this review. Find files again to see the latest list.</p>
        )}
        <details className="cleanup-disclosure" style={{ marginTop: 10 }}>
          <summary><ChevronDown size={14} aria-hidden="true" /> Selected files ({preview.fileCount})</summary>
          <ul className="cleanup-exclusions">
            {(preview.selected ?? []).map((candidate) => <li key={candidate.id}><span>{candidate.relativePath}</span><span className="muted">{formatBytes(candidate.logicalBytes)}</span></li>)}
          </ul>
        </details>
        {confirming && permanent && (
          <div className="banner banner-error cleanup-confirm" style={{ marginTop: 12 }} role="alertdialog" aria-modal="true" aria-labelledby="permanent-confirm-title">
            <AlertTriangle size={18} style={{ color: "var(--red)" }} aria-hidden="true" />
            <div>
              <strong id="permanent-confirm-title">Delete these files permanently?</strong>
              <p>This is irreversible. The selected files will not be available in the Recycle Bin.</p>
              <div className="cleanup-actions">
                <Button variant="primary" onClick={() => void confirmPermanent()} disabled={executing}>Delete permanently</Button>
                <Button variant="ghost" onClick={() => setConfirming(false)} disabled={executing}>Cancel</Button>
              </div>
            </div>
          </div>
        )}
        {result && <ExecutionResult result={result} />}
        <div className="cleanup-actions" style={{ marginTop: 12 }}>
          <Button variant="ghost" onClick={onClear}>Close review</Button>
          {permanent ? (
            <Button variant="primary" onClick={() => setConfirming(true)} disabled={!preview.executionAvailable || executing} aria-describedby="permanent-warning">
              Delete permanently…
            </Button>
          ) : (
            <Button variant="primary" disabled aria-describedby="cleaning-unavailable">Clean selected files</Button>
          )}
          <span id={permanent ? "permanent-warning" : "cleaning-unavailable"} className="muted">
            {permanent ? (preview.executionAvailable ? "You will be asked to confirm. This cannot be undone." : "Permanent deletion isn't available in this build.") : "Cleaning isn't available in this build yet."}
          </span>
        </div>
      </CardBody>
    </Card>
  );
}

function ExecutionResult({ result }: { result: CleanupExecutionView }) {
  const state = result.state === "completed" ? "Cleaning complete" : result.state === "partial" ? "Partially complete" : "Cleaning did not complete";
  return (
    <div className="banner" style={{ marginTop: 12, borderColor: result.state === "completed" ? "var(--accent-line)" : "var(--amber)" }} role="status">
      <div>
        <strong>{state}.</strong>
        <p className="cleanup-note">
          Requested {result.requested}; processed {result.processed}; deleted {result.deleted}; refused {result.refused}; changed {result.changed}; still present {result.stillPresent}; unknown {result.outcomeUnknown}.
        </p>
        <p className="cleanup-note">
          {result.permanent ? "Permanent deletion was requested. Recovery is unavailable." : "Recovery depends on the Recycle Bin."}
          {" "}
          {result.spaceReclaimedKnown ? `Space reclaimed: ${formatBytes(result.spaceReclaimedBytes)}.` : "Space reclaimed: not measured."}
        </p>
      </div>
    </div>
  );
}

function Summary({ label, value, note }: { label: string; value: string; note?: string }) {
  return <div><span>{label}</span><strong>{value}</strong>{note ? <small>{note}</small> : null}</div>;
}

