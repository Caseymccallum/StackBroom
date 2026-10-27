import { Box, CircleCheck, TriangleAlert } from "lucide-react";
import type { DockerView } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { formatBytes } from "../lib/format";
import { Badge, Card, CardBody, CardHeader, EmptyState } from "./ui";

export function DockerPanel({ docker }: { docker: DockerView }) {
  const diskUsage = docker.diskUsage ?? [];
  const containers = docker.containers ?? [];
  const images = docker.images ?? [];
  const warnings = docker.warnings ?? [];
  return (
    <Card>
      <CardHeader
        title="Docker"
        subtitle="Local daemon inspection through the existing read-only engine."
        action={
          <Badge tone={docker.available ? "teal" : "amber"}>
            {docker.available ? <CircleCheck size={11} /> : <TriangleAlert size={11} />}
            {docker.available ? "Available" : "Unavailable"}
          </Badge>
        }
      />
      <CardBody>
        {!docker.available ? (
          <EmptyState icon={<Box size={20} />} title="Docker is not available">
            {docker.reason || "The Docker CLI or daemon did not respond."} Remote endpoints are refused by default; this
            desktop build has no override.
          </EmptyState>
        ) : (
          <div className="grid" style={{ gap: 14 }}>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
              <Badge>{docker.running} running</Badge>
              <Badge>{docker.stopped} stopped</Badge>
              {docker.serverVersion ? <Badge tone="blue">server {docker.serverVersion}</Badge> : null}
              {docker.context ? <Badge>context {docker.context}</Badge> : null}
              <Badge tone={docker.endpointLocal ? "teal" : "amber"}>
                {docker.endpointLocal ? "local endpoint" : "network endpoint"}
              </Badge>
            </div>
            {docker.endpoint ? (
              <div className="muted mono" style={{ fontSize: 10.5, overflowWrap: "anywhere" }}>{docker.endpoint}</div>
            ) : null}

            {diskUsage.length > 0 ? (
              <div className="table-wrap">
                <table className="table">
                  <caption className="sr-only">Docker storage categories</caption>
                  <thead>
                    <tr><th>Category</th><th>Total</th><th>Active</th><th>Size</th><th>Reclaimable (Docker estimate)</th></tr>
                  </thead>
                  <tbody>
                    {diskUsage.map((row) => (
                      <tr key={row.type}>
                        <td className="strong">{row.type}</td>
                        <td className="num">{row.total}</td>
                        <td className="num">{row.active}</td>
                        <td className="num">{row.sizeText || formatBytes(row.sizeBytes ?? null, row.sizeBytes !== null)}</td>
                        <td className="num">{row.reclaimableText || formatBytes(row.reclaimableBytes ?? null, row.reclaimableBytes !== null)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="muted" style={{ fontSize: 11.5 }}>Docker did not report storage categories.</div>
            )}

            <div>
              <div className="muted" style={{ fontSize: 11, fontWeight: 600, marginBottom: 6 }}>Containers</div>
              {containers.length === 0 ? (
                <div className="muted" style={{ fontSize: 11.5 }}>No containers reported.</div>
              ) : (
                <div className="table-wrap">
                  <table className="table">
                    <caption className="sr-only">Docker containers</caption>
                    <thead>
                      <tr><th>Container</th><th>Image</th><th>State</th><th>CPU</th><th>Memory</th><th>Compose</th></tr>
                    </thead>
                    <tbody>
                      {containers.map((container) => (
                        <tr key={`${container.name}-${container.image}`}>
                          <td className="name" title={container.name}>{container.name}</td>
                          <td className="mono" style={{ maxWidth: 180, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={container.image}>{container.image}</td>
                          <td>{container.state}</td>
                          <td className="num">{container.statsKnown ? container.cpuPercent || "—" : "Unknown"}</td>
                          <td className="num">{container.statsKnown ? container.memoryUsage || "—" : "Unknown"}</td>
                          <td>{container.composeProject ? `${container.composeProject}${container.composeService ? ` / ${container.composeService}` : ""}` : "—"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
            {images.length > 0 ? (
              <div className="muted" style={{ fontSize: 10.5 }}>
                {images.length} image{images.length === 1 ? "" : "s"} reported. In-use status comes from
                Docker labels; sizes are Docker's own.
              </div>
            ) : null}
            {warnings.length > 0 ? (
              <div className="muted" style={{ fontSize: 10.5 }}>{warnings.join(" ")}</div>
            ) : null}
          </div>
        )}
      </CardBody>
    </Card>
  );
}
