import { ListTree, RefreshCw, ShieldCheck, Wrench } from "lucide-react";
import type { FullResult } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { formatPorts } from "../lib/format";
import { Badge, Button, Card, CardBody, CardHeader, EmptyState, LoadingBlock } from "../components/ui";
import { DockerPanel } from "../components/DockerPanel";
import { WSLPanel } from "../components/WSLPanel";
import { ListenerTable } from "../components/ListenerTable";
import { ReviewCard } from "../components/ReviewCard";
import { ObservationNotes } from "../components/ObservationNotes";

interface Props {
  full: FullResult | null;
  busy: "quick" | "full" | null;
  onScan: () => void;
}

export function DeveloperPage({ full, busy, onScan }: Props) {
  if (!full) {
    return (
      <Card>
        {busy ? (
          <LoadingBlock label="Reading Docker, WSL and TCP ownership" />
        ) : (
          <EmptyState
            icon={<Wrench size={20} />}
            title="Developer evidence has not been read"
            action={
              <Button variant="primary" onClick={onScan}>
                <RefreshCw size={14} /> Run full scan
              </Button>
            }
          >
            The full scan reuses StackBroom's existing Docker, WSL and TCP listener engine. It remains read-only and
            refuses non-local Docker endpoints.
          </EmptyState>
        )}
      </Card>
    );
  }

  const { developer } = full;
  const processes = full.processes ?? [];
  const listeners = developer.listeners ?? [];
  const duplicates = developer.duplicates ?? [];
  const opportunities = full.opportunities ?? [];
  const warnings = full.warnings ?? [];
  const portProcesses = processes.filter(
    (process) => process.category === "dev-server" || (process.requestedPorts?.length ?? 0) > 0 || (process.listeningPorts?.length ?? 0) > 0,
  );
  const confirmedListeners = listeners.filter((listener) => listener.ownership === "confirmed").length;

  return (
    <div className="grid" style={{ gap: 16 }}>
      <ObservationNotes warnings={warnings} />
      <nav className="section-links" aria-label="Developer sections">
        <a href="#developer-environments">Docker &amp; WSL</a>
        <a href="#developer-listeners">TCP listeners</a>
        <a href="#developer-ports">Process evidence</a>
        <a href="#developer-reviews">Review opportunities</a>
      </nav>

      <div className="grid two-col" id="developer-environments">
        <DockerPanel docker={developer.docker} />
        <WSLPanel wsl={developer.wsl} />
      </div>

      <Card id="developer-listeners">
        <CardHeader
          title="TCP listeners"
          subtitle="Observed sockets from the Windows TCP table. Ownership is confirmed only when PID and creation time match before and after sampling."
          action={<Badge tone="teal">{confirmedListeners} confirmed</Badge>}
        />
        <CardBody>
          {listeners.length === 0 ? (
            <EmptyState icon={<ListTree size={20} />} title="No TCP listeners observed">
              Listener inspection may be unavailable, or Windows returned no LISTEN rows. Absence is not evidence that a
              process failed.
            </EmptyState>
          ) : (
            <ListenerTable listeners={listeners} />
          )}
        </CardBody>
      </Card>

      <div className="grid two-col" id="developer-ports">
        <Card>
          <CardHeader
            title="Requested vs observed ports"
            subtitle="A command-line request is not a binding; a listener is not proof of redundancy."
            action={<Badge>{portProcesses.length} processes</Badge>}
          />
          <CardBody>
            {portProcesses.length === 0 ? (
              <EmptyState icon={<ListTree size={20} />} title="No port evidence on development processes">
                No development process requested or was observed listening on a port in this scan.
              </EmptyState>
            ) : (
              <div className="table-wrap" tabIndex={0} role="region" aria-label="Scrollable table">
                <table className="table">
                  <caption className="sr-only">Requested and observed ports by process</caption>
                  <thead><tr><th>Process</th><th>PID</th><th>Requested</th><th>Observed</th><th>Ownership</th></tr></thead>
                  <tbody>
                    {portProcesses.map((process) => (
                      <tr key={process.pid}>
                        <td className="name" title={process.name}>{process.name}</td>
                        <td className="num">{process.pid}</td>
                        <td className="num">{formatPorts(process.requestedPorts)}</td>
                        <td className="num">{formatPorts(process.listeningPorts)}</td>
                        <td><Badge tone={process.portOwnership === "confirmed" ? "teal" : process.portOwnership === "inferred" ? "amber" : "neutral"}>{process.portOwnership || "not associated"}</Badge></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardBody>
        </Card>

        <Card>
          <CardHeader
            title="Duplicate-server evidence"
            subtitle="Prompts to review, never a verdict that a process is redundant."
            action={<Badge>{duplicates.length}</Badge>}
          />
          <CardBody>
            {duplicates.length === 0 ? (
              <EmptyState icon={<ShieldCheck size={20} />} title="No duplicate evidence met the thresholds">
                That does not prove every development server is necessary; it means no two observations supported a
                duplicate prompt.
              </EmptyState>
            ) : (
              <div style={{ display: "grid", gap: 10 }}>
                {duplicates.map((duplicate) => (
                  <div key={`${duplicate.kind}-${duplicate.key}`} className="review-card" style={{ padding: 14 }}>
                    <div className="review-head">
                      <div className="review-title">{duplicate.key}</div>
                      <Badge tone={duplicate.confidence === "confirmed" ? "teal" : "amber"}>{duplicate.confidence}</Badge>
                    </div>
                    <div className="review-meta">
                      <Badge>PIDs {(duplicate.pids ?? []).join(", ")}</Badge>
                      {duplicate.port > 0 ? <Badge>port {duplicate.port}</Badge> : null}
                    </div>
                    <details className="review-details">
                      <summary>Evidence</summary>
                      <ul className="review-list">
                        {(duplicate.evidence ?? []).map((line, index) => <li key={`${index}-${line.slice(0, 20)}`}>{line}</li>)}
                      </ul>
                    </details>
                  </div>
                ))}
              </div>
            )}
          </CardBody>
        </Card>
      </div>

      <Card id="developer-reviews">
        <CardHeader
          title="Review opportunities"
          subtitle="Full evidence from the existing analysis engine. No control on this screen performs an action."
          action={<Badge>{opportunities.length}</Badge>}
        />
        <CardBody>
          {opportunities.length === 0 ? (
            <EmptyState icon={<ShieldCheck size={20} />} title="No suggestions met the evidence thresholds">
              Nothing is presented as idle or safe to remove without evidence.
            </EmptyState>
          ) : (
            <div className="opportunity-grid">
              {opportunities.map((opportunity) => (
                <ReviewCard key={opportunity.id} opportunity={opportunity} />
              ))}
            </div>
          )}
        </CardBody>
      </Card>
    </div>
  );
}
