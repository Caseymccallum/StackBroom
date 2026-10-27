import { formatPercent } from "../lib/format";

export function Sparkline({ values, label, tone = "teal" }: {
  values: Array<number | null>; label: string; tone?: "teal" | "blue";
}) {
  const width = 100, height = 90, padding = 4;
  const samples = values.map(value => value !== null && Number.isFinite(value) ? Math.min(100, Math.max(0, value)) : null);
  const point = (value: number, index: number) => [
    samples.length === 1 ? width / 2 : padding + index / (samples.length - 1) * (width - padding * 2),
    height - padding - value / 100 * (height - padding * 2),
  ];
  // A missing measurement breaks the line instead of inventing a zero or joining across a gap.
  let previousKnown = false;
  const line = samples.map((value, index) => {
    if (value === null) { previousKnown = false; return ""; }
    const [x, y] = point(value, index);
    const command = previousKnown ? "L" : "M";
    previousKnown = true;
    return `${command}${x.toFixed(2)},${y.toFixed(2)}`;
  }).join(" ");
  const latest = samples.at(-1) ?? null;
  const known = samples.filter(value => value !== null).length;
  const latestLabel = latest === null ? "Unavailable" : formatPercent(latest, true);
  const color = tone === "blue" ? "var(--blue)" : "var(--accent)";
  return (
    <div>
      <div className="chart-head"><span className="muted">{label}</span><span className="chart-value">{latestLabel}</span></div>
      <svg className="sparkline" viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" role="img"
        aria-label={`${label}, ${known} measured of ${samples.length} session samples, latest ${latestLabel}. Gaps are unavailable measurements.`}>
        {[padding, height / 2, height - padding].map(y => <line key={y} className="sparkline-grid" x1={padding} y1={y} x2={width - padding} y2={y} />)}
        <path className="sparkline-line" d={line} style={{ stroke: color }} />
        {samples.map((value, index) => {
          if (value === null) return null;
          const [x, y] = point(value, index);
          return <circle key={index} cx={x} cy={y} r={1.1} style={{ fill: color }} />;
        })}
      </svg>
      <div className="sparkline-axis"><span>0–100% · session order</span><span>{known}/{samples.length} measured</span></div>
    </div>
  );
}
