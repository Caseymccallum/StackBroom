package cleanup

import (
	"fmt"
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
	byID := make(map[string]Candidate, len(discovery.Candidates))
	for _, candidate := range discovery.Candidates {
		byID[candidate.ID] = candidate
	}
	seen := make(map[string]bool)
	preview := &Preview{
		Rule:             discovery.Rule,
		DeletionMode:     discovery.Rule.DeletionMode,
		RecoverableKnown: true,
		ExpiresAt:        now.Add(ttl),
		Risks:            append([]string(nil), discovery.Rule.Risks...),
		SideEffects:      []string{"No files are changed by this preview.", "A future cleanup would move selected files to the Recycle Bin where supported."},
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
		if err := storage.ValidateFileRecord(&candidate.File); err != nil {
			preview.Excluded = append(preview.Excluded, Exclusion{RelativePath: candidate.RelativePath, Reason: err.Error()})
			continue
		}
		if candidate.File.LinkCount > 1 {
			preview.Excluded = append(preview.Excluded, Exclusion{RelativePath: candidate.RelativePath, Reason: "hard-linked file"})
			continue
		}
		preview.Selected = append(preview.Selected, candidate)
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
		preview.RecoveryNote = "A future cleanup would prefer the Recycle Bin. Availability depends on volume, file size, policy and file type; permanent deletion is not enabled by this rule."
	default:
		preview.RecoveryNote = "Recovery is not guaranteed for this rule."
	}
	return preview, nil
}
