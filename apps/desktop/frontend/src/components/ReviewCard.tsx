import { Eye, ShieldCheck } from "lucide-react";
import type { Opportunity } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { Badge } from "./ui";

function riskTone(risk: string): "amber" | "red" | "neutral" {
  if (risk === "high") return "red";
  if (risk === "medium") return "amber";
  return "neutral";
}

export function ReviewCard({ opportunity, compact = false }: { opportunity: Opportunity; compact?: boolean }) {
  const evidence = opportunity.evidence ?? [];
  const review = opportunity.review ?? [];
  const details = evidence.length > 0 || review.length > 0;
  return (
    <article className="review-card" aria-label={opportunity.title}>
      <div className="review-head">
        <h3 className="review-title">{opportunity.title}</h3>
        <Badge tone={riskTone(opportunity.risk)}>{opportunity.risk} risk</Badge>
      </div>
      <p className="review-why">{opportunity.why}</p>
      <div className="review-meta">
        <Badge tone="blue">{opportunity.confidence} confidence</Badge>
        <Badge>{opportunity.resource}</Badge>
      </div>
      {!compact && details ? (
        <details className="review-details">
          <summary>Evidence and review steps</summary>
          <div className="muted" style={{ marginTop: 8, fontSize: 11.5 }}>
            {opportunity.detected}
          </div>
          {evidence.length > 0 ? (
            <ul className="review-list">
              {evidence.map((line, index) => (
                <li key={`${index}-${line.slice(0, 24)}`}>{line}</li>
              ))}
            </ul>
          ) : null}
          {review.length > 0 ? (
            <>
              <div className="muted" style={{ marginTop: 10, fontSize: 11, fontWeight: 600 }}>
                Before acting
              </div>
              <ul className="review-list">
                {review.map((line, index) => (
                  <li key={`${index}-${line.slice(0, 24)}`}>{line}</li>
                ))}
              </ul>
            </>
          ) : null}
          <div className="muted" style={{ marginTop: 10, fontSize: 11 }}>
            {opportunity.riskDetail}
          </div>
        </details>
      ) : null}
      <div className="review-footer">
        {compact ? <Eye size={12} aria-hidden="true" /> : <ShieldCheck size={12} aria-hidden="true" />}
        Review only — StackBroom will not act.
      </div>
    </article>
  );
}
