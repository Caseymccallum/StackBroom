import { HardDrive, ShieldCheck } from "lucide-react";
import type { QuickResult } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { formatBytes, formatPercent } from "../lib/format";
import { Badge, Card, CardBody, CardHeader, EmptyState, Progress } from "../components/ui";

export function StoragePage({ quick, full }: { quick: QuickResult | null; full: { overview: QuickResult["overview"] } | null }) {
  const overview = quick?.overview ?? full?.overview ?? null;
  const volumes = overview?.volumes ?? [];
  const total = volumes.reduce((sum, volume) => sum + volume.totalBytes, 0);
  const free = volumes.reduce((sum, volume) => sum + volume.freeBytes, 0);
  const used = Math.max(0, total - free);
  const usedPercent = total > 0 ? (used / total) * 100 : 0;

  if (!overview) {
    return (
      <Card>
        <EmptyState icon={<HardDrive size={20} />} title="No volume observation yet">
          Refresh the dashboard to read fixed-volume capacity from Windows.
        </EmptyState>
      </Card>
    );
  }

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
          title="Fixed volumes"
          subtitle="Free space is what Windows reports as available to the current user."
          action={<Badge>{formatBytes(free)} free</Badge>}
        />
        <CardBody>
          {volumes.length === 0 ? (
            <EmptyState icon={<HardDrive size={20} />} title="No fixed volumes measured">
              Removable and network volumes are intentionally not inspected.
            </EmptyState>
          ) : (
            <div className="table-wrap">
              <table className="table">
                <caption className="sr-only">Fixed volume capacity</caption>
                <thead>
                  <tr>
                    <th>Volume</th>
                    <th>Label</th>
                    <th>File system</th>
                    <th>Capacity</th>
                    <th>Free</th>
                    <th>Used</th>
                    <th style={{ minWidth: 150 }}>Usage</th>
                  </tr>
                </thead>
                <tbody>
                  {volumes.map((volume) => (
                    <tr key={volume.root}>
                      <td className="strong mono">{volume.root}</td>
                      <td>{volume.label || "Local disk"}</td>
                      <td>{volume.fileSystem || "Unknown"}</td>
                      <td className="num">{formatBytes(volume.totalBytes)}</td>
                      <td className="num">{formatBytes(volume.freeBytes)}</td>
                      <td className="num">{formatPercent(volume.usedPercent)}</td>
                      <td>
                        <Progress value={volume.usedPercent} tone={volume.usedPercent >= 90 ? "red" : volume.usedPercent >= 75 ? "amber" : "teal"} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardBody>
      </Card>

      <Card>
        <CardHeader title="What this view does not do" subtitle="Capacity is an observation, not a clean-up plan." />
        <CardBody>
          <div className="banner" style={{ alignItems: "flex-start" }}>
            <ShieldCheck size={16} aria-hidden="true" />
            <div>
              <strong style={{ color: "var(--text)" }}>Used space is not presented as safely reclaimable.</strong>
              <p style={{ margin: "5px 0 0" }}>
                StackBroom does not scan folders recursively, detect duplicate files, claim that caches are safe to
                delete, or remove anything. A future storage analysis must separately design reparse-point safety,
                permissions, exclusions, hard links and large-directory traversal before it can be trusted.
              </p>
            </div>
          </div>
        </CardBody>
      </Card>
    </div>
  );
}
