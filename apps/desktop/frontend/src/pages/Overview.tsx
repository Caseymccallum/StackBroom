import { Activity, ArrowRight, Cpu, Gauge, HardDrive, List, RefreshCw, ShieldCheck } from "lucide-react";
import type { FullResult, QuickResult } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import type { Sample } from "../types";
import { formatBytes, formatDuration, formatPercent, formatWhen } from "../lib/format";
import { Badge, Button, Card, CardBody, CardHeader, EmptyState, LoadingBlock, Progress } from "../components/ui";
import { ReviewCard } from "../components/ReviewCard";
import { Sparkline } from "../components/Sparkline";
import { ObservationNotes } from "../components/ObservationNotes";

interface Props {
  quick: QuickResult | null;
  full: FullResult | null;
  history: Sample[];
  busy: "quick" | "full" | null;
  onRefresh: () => void;
  onNavigate: (key: "processes" | "storage" | "developer") => void;
}

function memoryTone(percent: number): "teal" | "amber" | "red" {
  if (percent >= 90) return "red";
  if (percent >= 75) return "amber";
  return "teal";
}

export function OverviewPage({ quick, full, history, busy, onRefresh, onNavigate }: Props) {
  const overview = (() => {
    if (!quick) return full?.overview ?? null;
    if (!full) return quick.overview;
    const quickAt = new Date(quick.overview.capturedAt).getTime();
    const fullAt = new Date(full.overview.capturedAt).getTime();
    return quickAt >= fullAt ? quick.overview : full.overview;
  })();
  const warnings = overview?.scanMode === "full" ? (full?.warnings ?? []) : (quick?.warnings ?? []);
  if (!overview) {
    return (
      <Card>
        {busy ? (
          <LoadingBlock label={busy === "full" ? "Reading processes, Docker and WSL" : "Reading local system metrics"} />
        ) : (
          <EmptyState
            icon={<Gauge size={20} />}
            title="No system observation yet"
            action={
              <Button variant="primary" onClick={onRefresh}>
                <RefreshCw size={14} /> Read this PC
              </Button>
            }
          >
            StackBroom reads Windows process, memory, volume and listener data locally. No cleanup is performed.
          </EmptyState>
        )}
      </Card>
    );
  }

  const volumes = overview.volumes ?? [];
  const topProcesses = overview.topProcesses ?? [];
  const usedMemory = overview.memoryKnown ? Math.max(0, overview.totalMemoryBytes - overview.availableMemoryBytes) : 0;
  const totalStorage = volumes.reduce((sum, volume) => sum + volume.totalBytes, 0);
  const freeStorage = volumes.reduce((sum, volume) => sum + volume.freeBytes, 0);
  const storagePercent = totalStorage > 0 ? ((totalStorage - freeStorage) / totalStorage) * 100 : 0;
  const opportunities = full?.opportunities ?? [];

  return (
    <div className="grid" style={{ gap: 16 }}>
      <Card className="hero">
        <div>
          <div className="page-eyebrow">
            <span className="dot" aria-hidden="true" />
            This PC
          </div>
          <h2 className="hero-title">{overview.hostname || "Windows PC"}</h2>
          <p className="hero-subtitle">
            {overview.windowsVersion || "Windows version unavailable"} · {overview.logicalProcessors || "Unknown"} logical
            processors. An evidence-based snapshot of what is running and what it uses.
          </p>
          <div className="hero-meta">
            <Badge tone="teal">
              <ShieldCheck size={11} aria-hidden="true" /> Observe + verified control
            </Badge>
            <Badge>{overview.scanMode === "full" ? "Full scan" : "Quick local scan"}</Badge>
            <Badge>{overview.capturedAt ? formatWhen(overview.capturedAt) : "Not sampled"}</Badge>
            {full ? <Badge>{formatDuration(full.durationMs)} full scan</Badge> : null}
          </div>
        </div>
        <div className="hero-mark" aria-hidden="true">
          <Gauge size={46} strokeWidth={1.2} />
        </div>
      </Card>

      <ObservationNotes warnings={warnings} />
      <div className="grid metric-grid">
        <Card className="metric-card" style={{ ["--metric-accent" as string]: "var(--cyan)" }}>
          <div className="metric-top">
            <span>Memory in use</span>
            <span className="metric-icon"><Gauge size={16} /></span>
          </div>
          <div className="metric-value">
            {formatBytes(usedMemory, overview.memoryKnown)}
            <span className="metric-unit">of {formatBytes(overview.totalMemoryBytes, overview.memoryKnown)}</span>
          </div>
          <div className="metric-note">{formatPercent(overview.memoryUsedPercent, overview.memoryKnown)} used · {formatBytes(overview.availableMemoryBytes, overview.memoryKnown)} available</div>
          <div style={{ marginTop: 12 }}>
            <Progress label="Memory used" value={overview.memoryKnown ? overview.memoryUsedPercent : null} tone={memoryTone(overview.memoryUsedPercent)} />
          </div>
        </Card>

        <Card className="metric-card" style={{ ["--metric-accent" as string]: "var(--accent)" }}>
          <div className="metric-top">
            <span>CPU</span>
            <span className="metric-icon"><Activity size={16} /></span>
          </div>
          <div className="metric-value">
            {formatPercent(overview.cpuPercent, overview.cpuKnown)}
            {!overview.cpuKnown ? <span className="metric-unit">unavailable</span> : null}
          </div>
          <div className="metric-note">{overview.cpuKnown ? "Short interval sample from Windows system counters" : "The CPU counters could not be sampled"}</div>
          <div style={{ marginTop: 12 }}>
            <Progress label="CPU usage" value={overview.cpuKnown ? overview.cpuPercent : null} />
          </div>
        </Card>

        <Card className="metric-card" style={{ ["--metric-accent" as string]: "var(--blue)" }}>
          <div className="metric-top">
            <span>Storage</span>
            <span className="metric-icon"><HardDrive size={16} /></span>
          </div>
          <div className="metric-value">
            {formatBytes(freeStorage, totalStorage > 0)}
            <span className="metric-unit">free</span>
          </div>
          <div className="metric-note">{volumes.length} fixed volume{volumes.length === 1 ? "" : "s"} · {formatBytes(totalStorage)} capacity</div>
          <div style={{ marginTop: 12 }}>
            <Progress label="Storage used" value={totalStorage > 0 ? storagePercent : null} tone={storagePercent >= 90 ? "red" : storagePercent >= 75 ? "amber" : "teal"} />
          </div>
        </Card>

        <Card className="metric-card" style={{ ["--metric-accent" as string]: "var(--amber)" }}>
          <div className="metric-top">
            <span>Processes</span>
            <span className="metric-icon"><List size={16} /></span>
          </div>
          <div className="metric-value">
            {overview.processCount}
            <span className="metric-unit">observed</span>
          </div>
          <div className="metric-note">{overview.devProcessCount} development processes · {overview.devServerCount} servers</div>
          <button type="button" className="btn btn-ghost" style={{ marginTop: 8, paddingLeft: 0 }} onClick={() => onNavigate("processes")}>
            Browse processes <ArrowRight size={13} />
          </button>
        </Card>
      </div>
      <div className="grid two-col">
        <Card>
          <CardHeader
            title="Session trends"
            subtitle="Only samples collected while this window has been open. Nothing is saved or sent."
            action={<Badge>{history.length} sample{history.length === 1 ? "" : "s"}</Badge>}
          />
          <CardBody>
            {history.length < 2 ? (
              <EmptyState icon={<Activity size={20} />} title="Waiting for a second sample">
                The first chart line appears after another quick refresh. StackBroom does not fabricate a history.
              </EmptyState>
            ) : (
              <div className="chart-card">
                <div className="chart-block">
                  <Sparkline values={history.map((sample) => sample.memoryPercent)} label="Memory used" tone="blue" />
                </div>
                <div className="chart-block">
                  <Sparkline values={history.map((sample) => sample.cpuPercent)} label="CPU" />
                </div>
              </div>
            )}
          </CardBody>
        </Card>

        <Card>
          <CardHeader
            title="Volumes"
            subtitle="Capacity and free space as Windows reports them."
            action={
              <Button variant="ghost" onClick={() => onNavigate("storage")}>
                Details <ArrowRight size={13} />
              </Button>
            }
          />
          <CardBody>
            {volumes.length === 0 ? (
              <EmptyState icon={<HardDrive size={20} />} title="No fixed volumes measured">
                Windows did not return a readable fixed volume. Nothing is estimated.
              </EmptyState>
            ) : (
              <div>
                {volumes.map((volume) => (
                  <div key={volume.root}>
                    <div className="volume-row" style={{ paddingBottom: 8 }}>
                      <div className="volume-label">
                        <span className="volume-root">{volume.root.replace("\\", "")}</span>
                        <div className="volume-meta">
                          <div className="volume-name">{volume.label || "Local disk"}</div>
                          <div className="volume-detail">{volume.fileSystem || "Unknown filesystem"}</div>
                        </div>
                      </div>
                      <div className="volume-figures">
                        {formatBytes(volume.freeBytes)} free of {formatBytes(volume.totalBytes)}
                      </div>
                    </div>
                    <Progress value={volume.usedPercent} tone={volume.usedPercent >= 90 ? "red" : volume.usedPercent >= 75 ? "amber" : "teal"} />
                  </div>
                ))}
              </div>
            )}
          </CardBody>
        </Card>
      </div>
      <div className="grid two-col">
        <Card>
          <CardHeader
            title="Top memory consumers"
            subtitle="Working set, not private bytes. Shared pages may be counted more than once."
            action={<Badge>{topProcesses.length} shown</Badge>}
          />
          <CardBody>
            {topProcesses.length === 0 ? (
              <EmptyState icon={<Cpu size={20} />} title="No measured processes">
                Process memory was unavailable or no processes were returned.
              </EmptyState>
            ) : (
              <div className="table-wrap" tabIndex={0} role="region" aria-label="Scrollable table">
                <table className="table">
                  <caption className="sr-only">Top processes by working set memory</caption>
                  <thead>
                    <tr>
                      <th>Process</th>
                      <th>PID</th>
                      <th>Memory</th>
                      <th>Category</th>
                      <th>Project</th>
                    </tr>
                  </thead>
                  <tbody>
                    {topProcesses.map((process) => (
                      <tr key={process.pid}>
                        <td className="name" title={process.name}>{process.name}</td>
                        <td className="num">{process.pid}</td>
                        <td className="num">{formatBytes(process.memoryBytes, process.memoryKnown)}</td>
                        <td>{process.category === "dev-server" ? "Dev server" : process.category === "dev-tool" ? "Dev tool" : "Process"}</td>
                        <td>{process.projectName || "—"}</td>
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
            title="Review opportunities"
            subtitle="Evidence-led prompts, never automatic actions."
            action={<Badge tone={opportunities.length > 0 ? "amber" : "neutral"}>{opportunities.length}</Badge>}
          />
          <CardBody>
            {!full ? (
              <EmptyState icon={<ShieldCheck size={20} />} title="Full evidence is still loading">
                Docker, WSL and command-line classification are read in the background. You can continue using the dashboard.
              </EmptyState>
            ) : opportunities.length === 0 ? (
              <EmptyState icon={<ShieldCheck size={20} />} title="No suggestions met the evidence thresholds">
                That does not mean nothing is idle; it means StackBroom could not show that anything is worth reviewing.
              </EmptyState>
            ) : (
              <div style={{ display: "grid", gap: 12 }}>
                {opportunities.slice(0, 2).map((opportunity) => (
                  <ReviewCard key={opportunity.id} opportunity={opportunity} compact />
                ))}
                <Button variant="ghost" onClick={() => onNavigate("developer")}>
                  See all evidence in Developer Tools <ArrowRight size={13} />
                </Button>
              </div>
            )}
          </CardBody>
        </Card>
      </div>
    </div>
  );
}
