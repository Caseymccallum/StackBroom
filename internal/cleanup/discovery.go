package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/storage"
)

type Progress struct {
	Phase   string
	Items   int
	Bytes   uint64
	Errors  int
	Partial bool
	Message string
}

type Candidate struct {
	ID              string
	File            storage.FileRecord
	RelativePath    string
	Uncertain       bool
	UncertainReason string
}

type Exclusion struct {
	RelativePath string
	Reason       string
}

type Discovery struct {
	Rule             Rule
	Root             string
	ScannedFiles     int
	Eligible         int
	Candidates       []Candidate
	Excluded         []Exclusion
	ExcludedCounts   map[string]int
	ExcludedTotal    int
	TotalLogical     uint64
	TotalAllocated   uint64
	AllocatedUnknown int
	Truncated        bool
	Partial          bool
	PartialReason    string
	Coverage         storage.Coverage
	SkippedCount     int
}

// Discover performs a read-only scan of the rule's exact scope. It reuses the
// hardened storage scanner and never writes or deletes anything.
func Discover(ctx context.Context, rule Rule, maxCandidates int, progress func(Progress), now func() time.Time) (*Discovery, error) {
	if maxCandidates <= 0 || maxCandidates > rule.MaxCandidates {
		maxCandidates = rule.MaxCandidates
	}
	root, err := rule.ResolveRoot()
	if err != nil {
		return nil, err
	}
	root, err = storage.ValidateRoot(root)
	if err != nil {
		return nil, fmt.Errorf("cleanup scope is not usable: %w", err)
	}
	request := storage.DefaultRequest(root)
	request.IncludeHidden = false
	request.IncludeSystem = false
	request.MaxDepth = rule.MaxDepth
	request.MaxEntries = rule.MaxEntries
	request.MinDuplicateSize = ^uint64(0)
	result, err := storage.NewScanner().Scan(ctx, request, func(scan storage.Progress) {
		if progress == nil {
			return
		}
		progress(Progress{
			Phase:   scan.Phase,
			Items:   scan.Items,
			Bytes:   scan.Bytes,
			Errors:  scan.Errors,
			Partial: scan.Partial,
			Message: scan.Message,
		})
	})
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	discovery := &Discovery{
		Rule:           rule,
		Root:           root,
		ScannedFiles:   len(result.Files),
		Coverage:       result.Coverage,
		Partial:        result.Partial,
		PartialReason:  result.PartialReason,
		SkippedCount:   result.SkippedCount,
		ExcludedCounts: make(map[string]int),
	}
	for _, file := range result.Files {
		reason := ""
		switch {
		case file.Placeholder:
			reason = "cloud placeholder"
		case file.Reparse:
			reason = "reparse point"
		case file.LinkCount > 1:
			reason = "hard-linked file"
		case !rule.AllowsExtension(file.Name):
			reason = "extension excluded by rule"
		case now().Sub(file.ModTime) < time.Duration(rule.MinAgeDays)*24*time.Hour:
			reason = fmt.Sprintf("newer than %d days", rule.MinAgeDays)
		}
		if reason != "" {
			discovery.addExclusion(file.RelativePath, reason)
			continue
		}
		discovery.Eligible++
		if len(discovery.Candidates) >= maxCandidates {
			discovery.Truncated = true
			continue
		}
		uncertain := file.FileID == ""
		uncertainReason := ""
		if uncertain {
			uncertainReason = "file identity was unavailable during discovery"
		}
		discovery.Candidates = append(discovery.Candidates, Candidate{
			ID:              fmt.Sprintf("c%d", len(discovery.Candidates)+1),
			File:            *file,
			RelativePath:    file.RelativePath,
			Uncertain:       uncertain,
			UncertainReason: uncertainReason,
		})
		discovery.TotalLogical += file.LogicalBytes
		if file.AllocatedKnown {
			discovery.TotalAllocated += file.AllocatedBytes
		} else {
			discovery.AllocatedUnknown++
		}
	}
	if discovery.Truncated {
		discovery.Partial = true
		if discovery.PartialReason == "" {
			discovery.PartialReason = fmt.Sprintf("candidate limit reached (%d)", maxCandidates)
		}
	}
	return discovery, nil
}

func (d *Discovery) addExclusion(relativePath, reason string) {
	d.ExcludedTotal++
	d.ExcludedCounts[reason]++
	if len(d.Excluded) < 200 {
		d.Excluded = append(d.Excluded, Exclusion{RelativePath: relativePath, Reason: reason})
	}
}
