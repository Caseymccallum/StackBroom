import { CircleCheck, Terminal, TriangleAlert } from "lucide-react";
import type { WSLView } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { formatBytes } from "../lib/format";
import { Badge, Card, CardBody, CardHeader, EmptyState } from "./ui";

export function WSLPanel({ wsl }: { wsl: WSLView }) {
  const distributions = wsl.distributions ?? [];
  const warnings = wsl.warnings ?? [];
  return (
    <Card>
      <CardHeader
        title="WSL2"
        subtitle="Distribution state and the shared host-side utility VM."
        action={
          <Badge tone={wsl.available ? "teal" : "amber"}>
            {wsl.available ? <CircleCheck size={11} /> : <TriangleAlert size={11} />}
            {wsl.available ? "Available" : "Unavailable"}
          </Badge>
        }
      />
      <CardBody>
        {!wsl.available ? (
          <EmptyState icon={<Terminal size={20} />} title="WSL is not available">
            {wsl.reason || "wsl.exe did not return distribution data."}
          </EmptyState>
        ) : (
          <div className="grid" style={{ gap: 12 }}>
            {distributions.length === 0 ? (
              <div className="muted" style={{ fontSize: 11.5 }}>No WSL distributions reported.</div>
            ) : (
              <div className="wsl-list">
                {distributions.map((distro) => (
                  <div className="wsl-row" key={distro.name}>
                    <div className="wsl-name" title={distro.name}>{distro.name}</div>
                    <div className="wsl-meta">
                      <Badge tone={distro.state.toLowerCase() === "running" ? "teal" : "neutral"}>{distro.state}</Badge>
                      <span>WSL {distro.version}</span>
                      {distro.isDefault ? <Badge tone="blue">default</Badge> : null}
                    </div>
                  </div>
                ))}
              </div>
            )}
            <div className="muted" style={{ fontSize: 11.5 }}>
              {wsl.vmMemoryKnown ? (
                <>Host-side WSL utility VM: <strong style={{ color: "var(--text)" }}>{formatBytes(wsl.vmMemoryBytes)}</strong>.
                  This is shared by every running distribution and is approximate.</>
              ) : (
                "The host-side WSL utility VM memory could not be measured."
              )}
            </div>
            {wsl.vmNote ? <div className="muted" style={{ fontSize: 10.5 }}>{wsl.vmNote}</div> : null}
            {warnings.length > 0 ? <div className="muted" style={{ fontSize: 10.5 }}>{warnings.join(" ")}</div> : null}
          </div>
        )}
      </CardBody>
    </Card>
  );
}
