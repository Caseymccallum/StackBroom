package cleanup

import (
	"fmt"
	"strings"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/storage"
)

type Preview struct {
	ID               string
	Rule             Rule
	Selected         []Candidate
	Excluded         []Exclusion
	TotalLogical     uint64
	TotalAllocated   uint64
	RecoverableBytes uint64
	RecoverableKnown bool
	AllocatedUnknown int
	DeletionMode     string
	RecoveryNote     string
	Risks            []string
	SideEffects      []string
	ExpiresAt        time.Time
}

// BuildPreview revalidates exact candidate identities selected from a
// discovery. It has no execution path and expires.
func BuildPreview(discovery *Discovery, candidateIDs []string, now time.Time, ttl time.Duration) (*Preview, error) {
	if discovery == nil {
		return nil, fmt.Errorf("cleanup discovery is required")
	}
	if len(candidateIDs) == 0 {
		return nil, fmt.Errorf("select at least one candidate")
	}
	rule, ok := RuleByID(discovery.Rule.ID)
	if !ok || rule.Version != discovery.Rule.Version {
		return nil, fmt.Errorf("cleanup rule changed")
	}
	if len(candidateIDs) > rule.MaxCandidates {
		return nil, fmt.Errorf("candidate limit")
	}
	if discovery.Rule.DeletionMode != "recycle" {
		return nil, fmt.Errorf("only recoverable cleanup previews are supported")
	}
	trustedRoot, err := rule.ResolveRoot()
	if err != nil {
		return nil, fmt.Errorf("cleanup scope is not trusted: %w", err)
	}
	if discovery.Root == "" || !strings.EqualFold(trustedRoot, discovery.Root) {
		return nil, fmt.Errorf("discovery root is not the trusted rule scope")
	}
	byID := make(map[string]Candidate, len(discovery.Candidates))
	for _, candidate := range discovery.Candidates {
		byID[candidate.ID] = candidate
	}
	seen := make(map[string]bool)
	preview := &Preview{
		Rule:             rule,
		DeletionMode:     discovery.Rule.DeletionMode,
		RecoverableKnown: true,
		ExpiresAt:        now.Add(ttl),
		Risks:            append([]string(nil), discovery.Rule.Risks...),
		SideEffects:      []string{"No files are changed by this review.", "Cleaning would move selected files to the Recycle Bin when Windows allows it."},
	}
	for _, id := range candidateIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		candidate, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("candidate is not part of this discovery")
		}
		if err := storage.ValidateCleanupTarget(&candidate.File); err != nil {
			preview.Excluded = append(preview.Excluded, Exclusion{RelativePath: candidate.RelativePath, Reason: err.Error()})
			continue
		}
		if !candidate.File.RootEqual(discovery.Root) {
			preview.Excluded = append(preview.Excluded, Exclusion{RelativePath: candidate.RelativePath, Reason: "target belongs to a different approved root"})
			continue
		}
		if !discovery.Rule.AllowsExtension(candidate.File.Name) {
			preview.Excluded = append(preview.Excluded, Exclusion{RelativePath: candidate.RelativePath, Reason: "extension no longer matches rule"})
			continue
		}
		if now.Sub(candidate.File.ModTime) < time.Duration(discovery.Rule.MinAgeDays)*24*time.Hour {
			preview.Excluded = append(preview.Excluded, Exclusion{RelativePath: candidate.RelativePath, Reason: "target is no longer old enough"})
			continue
		}
		preview.Selected = append(preview.Selected, candidate)
		if rule.ValidateEligibility(candidate.File, len(candidateIDs), now) != nil {
			return nil, fmt.Errorf("target no longer matches rule constraints")
		}
		preview.TotalLogical += candidate.File.LogicalBytes
		if candidate.File.AllocatedKnown {
			preview.TotalAllocated += candidate.File.AllocatedBytes
			preview.RecoverableBytes += candidate.File.AllocatedBytes
		} else {
			preview.RecoverableKnown = false
			preview.AllocatedUnknown++
		}
	}
	if len(preview.Selected) == 0 {
		return nil, fmt.Errorf("no selected candidate is still safe to preview")
	}
	switch preview.Rule.DeletionMode {
	case "recycle":
		preview.RecoveryNote = "Windows may not be able to put every file in the Recycle Bin. StackBroom will not permanently delete a file instead."
	default:
		preview.RecoveryNote = "This cleanup method cannot be made recoverable."
	}
	return preview, nil
}
