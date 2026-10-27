import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { Opportunity } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { ReviewCard } from "./ReviewCard";

const opportunity: Opportunity = {
  id: "docker-build-cache",
  kind: "docker-build-cache",
  title: "Docker reports reclaimable build cache",
  detected: "Docker reported 630 MB of build cache.",
  why: "Docker's own estimate says this space may be recoverable.",
  evidence: ["Docker system df reported 630.1MB.", "The figure is Docker's estimate."],
  resource: "630 MB (Docker estimate)",
  resourceUnknown: false,
  review: ["Confirm no active build depends on it.", "Check the Docker estimate yourself."],
  risk: "medium",
  riskDetail: "Removing build cache slows the next build.",
  confidence: "confirmed",
};

describe("ReviewCard", () => {
  it("shows evidence without offering an action", () => {
    render(<ReviewCard opportunity={opportunity} />);
    expect(screen.getByText(opportunity.title)).toBeInTheDocument();
    expect(screen.getByText("medium risk")).toBeInTheDocument();
    expect(screen.getByText("confirmed confidence")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /clean|fix|optimize|delete|remove now/i })).not.toBeInTheDocument();
    expect(screen.getByText(/Review only/)).toBeInTheDocument();
  });

  it("expands evidence and review steps on demand", () => {
    render(<ReviewCard opportunity={opportunity} />);
    fireEvent.click(screen.getByText("Evidence and review steps"));
    expect(screen.getByText("Docker system df reported 630.1MB.")).toBeInTheDocument();
    expect(screen.getByText("Before acting")).toBeInTheDocument();
  });
});
