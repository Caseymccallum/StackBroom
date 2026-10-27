import { formatPercent } from "../lib/format";

export function Sparkline({
  values,
  label,
  tone = "teal",
}: {
  values: number[];
  label: string;
  tone?: "teal" | "blue";
}) {
  const width = 100;
  const height = 90;
  const padding = 4;
  const clamped = values.map((value) => Math.min(100, Math.max(0, value)));
  const point = (value: number, index: number) => {
    const x = clamped.length === 1 ? width / 2 : padding + (index / (clamped.length - 1)) * (width - padding * 2);
    const y = height - padding - (value / 100) * (height - padding * 2);
    return [x, y] as const;
  };
  const points = clamped.map(point);
  const line = points.map(([x, y], index) => `${index === 0 ? "M" : "L"}${x.toFixed(2)},${y.toFixed(2)}`).join(" ");
  const area = `${line} L${points[points.length - 1][0].toFixed(2)},${height - padding} L${points[0][0].toFixed(2)},${height - padding} Z`;
  const latest = clamped[clamped.length - 1] ?? 0;
  return (
    <div>
      <div className="chart-head">
        <span className="muted" style={{ fontSize: 11, fontWeight: 600 }}>
          {label}
        </span>
        <span className="chart-value">{formatPercent(latest, true)}</span>
      </div>
      <svg
        className="sparkline"
        viewBox={`0 0 ${width} ${height}`}
        preserveAspectRatio="none"
        role="img"
        aria-label={`${label}, session samples, latest ${formatPercent(latest, true)}`}
      >
        <line className="sparkline-grid" x1={padding} y1={height / 2} x2={width - padding} y2={height / 2} />
        <line className="sparkline-grid" x1={padding} y1={padding} x2={width - padding} y2={padding} />
        <line className="sparkline-grid" x1={padding} y1={height - padding} x2={width - padding} y2={height - padding} />
        {clamped.length > 1 ? <path className="sparkline-area" d={area} style={{ fill: tone === "blue" ? "var(--blue-soft)" : undefined }} /> : null}
        {clamped.length > 1 ? (
          <path className="sparkline-line" d={line} style={{ stroke: tone === "blue" ? "var(--blue)" : undefined }} />
        ) : (
          <circle cx={width / 2} cy={point(latest, 0)[1]} r={1.6} style={{ fill: tone === "blue" ? "var(--blue)" : "var(--accent)" }} />
        )}
      </svg>
      <div className="sparkline-axis" aria-hidden="true">
        <span>0%</span>
        <span>now</span>
      </div>
    </div>
  );
}
