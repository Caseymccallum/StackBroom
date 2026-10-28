package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup"
	"github.com/Caseymccallum/StackBroom/internal/storage"
)

type cleanupDiscoveryState struct {
	ID          string
	Rule        cleanup.Rule
	Status      string
	Progress    cleanup.Progress
	Result      *cleanup.Discovery
	ErrorCode   string
	Error       string
	StartedAt   time.Time
	CompletedAt time.Time
	Cancel      context.CancelFunc
}

type cleanupPreviewState struct {
	View      CleanupPreviewView
	ExpiresAt time.Time
}

// CleanupService is read-only: it discovers candidates and creates expiring
// previews, but intentionally has no execution or deletion method.
type CleanupService struct {
	mu          sync.Mutex
	discoveries map[string]*cleanupDiscoveryState
	previews    map[string]*cleanupPreviewState
	active      string
	now         func() time.Time
	discoverFn  func(context.Context, cleanup.Rule, int, func(cleanup.Progress), func() time.Time) (*cleanup.Discovery, error)
}

func NewCleanupService() *CleanupService {
	return &CleanupService{
		discoveries: make(map[string]*cleanupDiscoveryState),
		previews:    make(map[string]*cleanupPreviewState),
		now:         time.Now,
		discoverFn:  cleanup.Discover,
	}
}

// ListCleanupRules returns the curated catalogue. No path is accepted.
func (s *CleanupService) ListCleanupRules() []CleanupRule {
	rules := cleanup.Catalog()
	out := make([]CleanupRule, 0, len(rules))
	for _, rule := range rules {
		out = append(out, mapCleanupRule(rule))
	}
	return out
}

func mapCleanupRule(rule cleanup.Rule) CleanupRule {
	return CleanupRule{
		ID:               rule.ID,
		Version:          rule.Version,
		Title:            rule.Title,
		Description:      rule.Description,
		Owner:            rule.Owner,
		Category:         rule.Category,
		ScopeDisplay:     rule.ScopeDisplay,
		SupportedWindows: rule.SupportedWindows,
		MinAgeDays:       rule.MinAgeDays,
		DeletionMode:     rule.DeletionMode,
		Recoverable:      rule.Recoverable,
		RequiresClosed:   append([]string(nil), rule.RequiresClosed...),
		Risks:            append([]string(nil), rule.Risks...),
		Evidence:         append([]string(nil), rule.Evidence...),
		WhySafe:          rule.WhySafe,
	}
}

// StartCleanupDiscovery starts one read-only discovery for a curated rule.
func (s *CleanupService) StartCleanupDiscovery(ruleID string, maxCandidates int) (string, error) {
	rule, ok := cleanup.RuleByID(ruleID)
	if !ok {
		return "", fmt.Errorf("cleanup rule is not recognised")
	}
	if maxCandidates <= 0 || maxCandidates > rule.MaxCandidates {
		maxCandidates = rule.MaxCandidates
	}
	s.mu.Lock()
	if s.active != "" {
		if active := s.discoveries[s.active]; active != nil && active.Status == "running" {
			s.mu.Unlock()
			return "", fmt.Errorf("a cleanup discovery is already running")
		}
	}
	s.pruneDiscoveriesLocked()
	id, err := randomStorageID()
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	state := &cleanupDiscoveryState{
		ID:        id,
		Rule:      rule,
		Status:    "running",
		StartedAt: s.now(),
		Cancel:    cancel,
	}
	s.discoveries[id] = state
	s.active = id
	s.mu.Unlock()

	go func() {
		result, discoverErr := s.discoverFn(ctx, rule, maxCandidates, func(progress cleanup.Progress) {
			s.mu.Lock()
			if current := s.discoveries[id]; current != nil {
				current.Progress = progress
			}
			s.mu.Unlock()
		}, s.now)
		contextErr := ctx.Err()
		cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		state.CompletedAt = s.now()
		if discoverErr != nil {
			state.Status = "error"
			state.ErrorCode = "discovery_failed"
			root, _ := rule.ResolveRoot()
			state.Error = sanitizeStorageError(discoverErr, root)
		} else {
			state.Result = result
			switch {
			case result.PartialReason == "cancelled" || contextErr == context.Canceled:
				state.Status = "cancelled"
			case result.PartialReason == "timeout" || contextErr == context.DeadlineExceeded:
				state.Status = "timeout"
			default:
				state.Status = "completed"
			}
		}
		if s.active == id {
			s.active = ""
		}
	}()
	return id, nil
}

func (s *CleanupService) pruneDiscoveriesLocked() {
	if len(s.discoveries) < 8 {
		return
	}
	oldestID := ""
	var oldest time.Time
	for id, state := range s.discoveries {
		if state.Status == "running" {
			continue
		}
		if oldestID == "" || state.StartedAt.Before(oldest) {
			oldestID = id
			oldest = state.StartedAt
		}
	}
	if oldestID != "" {
		delete(s.discoveries, oldestID)
	}
}

func (s *CleanupService) CancelCleanupDiscovery(discoveryID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.discoveries[discoveryID]
	if state == nil || state.Status != "running" {
		return false
	}
	state.Cancel()
	return true
}

func (s *CleanupService) GetCleanupDiscovery(discoveryID string) (CleanupDiscoveryView, error) {
	s.mu.Lock()
	state := s.discoveries[discoveryID]
	if state == nil {
		s.mu.Unlock()
		return CleanupDiscoveryView{}, fmt.Errorf("cleanup discovery not found")
	}
	view := CleanupDiscoveryView{
		DiscoveryID:  state.ID,
		Rule:         mapCleanupRule(state.Rule),
		Status:       state.Status,
		ErrorCode:    state.ErrorCode,
		ErrorMessage: state.Error,
		Progress: CleanupProgress{
			Phase: state.Progress.Phase, Items: state.Progress.Items, Bytes: state.Progress.Bytes,
			Errors: state.Progress.Errors, Partial: state.Progress.Partial, Message: state.Progress.Message,
		},
		StartedAt: state.StartedAt, CompletedAt: state.CompletedAt,
	}
	result := state.Result
	s.mu.Unlock()
	if result == nil {
		return view, nil
	}
	view.ExcludedCounts = make(map[string]int, len(result.ExcludedCounts))
	for reason, count := range result.ExcludedCounts {
		view.ExcludedCounts[reason] = count
	}
	view.ExcludedTotal = result.ExcludedTotal
	view.ScannedFiles = result.ScannedFiles
	view.Eligible = result.Eligible
	view.TotalLogical = result.TotalLogical
	view.TotalAllocated = result.TotalAllocated
	view.AllocatedUnknown = result.AllocatedUnknown
	view.Truncated = result.Truncated
	view.Partial = result.Partial
	view.PartialReason = result.PartialReason
	view.SkippedCount = result.SkippedCount
	view.Coverage = mapStorageCoverage(result.Coverage)
	for _, candidate := range result.Candidates {
		view.Candidates = append(view.Candidates, mapCleanupCandidate(candidate))
	}
	for _, exclusion := range result.Excluded {
		view.Excluded = append(view.Excluded, CleanupExclusion{RelativePath: exclusion.RelativePath, Reason: exclusion.Reason})
	}
	return view, nil
}

func mapCleanupCandidate(candidate cleanup.Candidate) CleanupCandidate {
	return CleanupCandidate{
		ID: candidate.ID, Name: candidate.File.Name, RelativePath: candidate.RelativePath,
		LogicalBytes: candidate.File.LogicalBytes, AllocatedBytes: candidate.File.AllocatedBytes,
		AllocatedKnown: candidate.File.AllocatedKnown, ModifiedAt: candidate.File.ModTime,
		LinkCount: candidate.File.LinkCount, Uncertain: candidate.Uncertain, UncertainReason: candidate.UncertainReason,
	}
}

func mapStorageCoverage(coverage storage.Coverage) StorageCoverage {
	return StorageCoverage{
		Exhaustive: coverage.Exhaustive, AccessDenied: coverage.AccessDenied,
		HiddenExcluded: coverage.HiddenExcluded, SystemExcluded: coverage.SystemExcluded,
		ReparseExcluded: coverage.ReparseExcluded, DepthLimit: coverage.DepthLimit,
		EntryLimit: coverage.EntryLimit, Cancelled: coverage.Cancelled, TimedOut: coverage.TimedOut,
		Reasons: append([]string(nil), coverage.Reasons...),
	}
}

// CreateCleanupPreview builds an expiring preview from exact discovery IDs.
// It never mutates a file and there is deliberately no execution method.
func (s *CleanupService) CreateCleanupPreview(discoveryID string, candidateIDs []string) (CleanupPreviewView, error) {
	s.mu.Lock()
	state := s.discoveries[discoveryID]
	if state == nil || state.Result == nil {
		s.mu.Unlock()
		return CleanupPreviewView{}, fmt.Errorf("cleanup discovery result not found")
	}
	if !state.CompletedAt.IsZero() && s.now().Sub(state.CompletedAt) > time.Hour {
		s.mu.Unlock()
		return CleanupPreviewView{}, fmt.Errorf("cleanup discovery result expired")
	}
	result := state.Result
	s.mu.Unlock()

	preview, err := cleanup.BuildPreview(result, candidateIDs, s.now(), 10*time.Minute)
	if err != nil {
		return CleanupPreviewView{}, err
	}
	id, err := randomStorageID()
	if err != nil {
		return CleanupPreviewView{}, err
	}
	view := CleanupPreviewView{
		PreviewID: id, RuleID: preview.Rule.ID, RuleVersion: preview.Rule.Version,
		RuleTitle: preview.Rule.Title, FileCount: len(preview.Selected),
		TotalLogical: preview.TotalLogical, TotalAllocated: preview.TotalAllocated,
		RecoverableBytes: preview.RecoverableBytes, RecoverableKnown: preview.RecoverableKnown,
		AllocatedUnknown: preview.AllocatedUnknown, DeletionMode: preview.DeletionMode,
		RecoveryNote: preview.RecoveryNote, Risks: append([]string(nil), preview.Risks...),
		SideEffects: append([]string(nil), preview.SideEffects...), ExpiresAt: preview.ExpiresAt,
	}
	for _, candidate := range preview.Selected {
		view.Selected = append(view.Selected, mapCleanupCandidate(candidate))
	}
	for _, exclusion := range preview.Excluded {
		view.Excluded = append(view.Excluded, CleanupExclusion{RelativePath: exclusion.RelativePath, Reason: exclusion.Reason})
	}
	s.mu.Lock()
	s.prunePreviewsLocked()
	s.previews[id] = &cleanupPreviewState{View: view, ExpiresAt: preview.ExpiresAt}
	s.mu.Unlock()
	return view, nil
}

func (s *CleanupService) prunePreviewsLocked() {
	if len(s.previews) < 20 {
		return
	}
	oldestID := ""
	var oldest time.Time
	for id, preview := range s.previews {
		if oldestID == "" || preview.ExpiresAt.Before(oldest) {
			oldestID = id
			oldest = preview.ExpiresAt
		}
	}
	if oldestID != "" {
		delete(s.previews, oldestID)
	}
}

func (s *CleanupService) GetCleanupPreview(previewID string) (CleanupPreviewView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview := s.previews[previewID]
	if preview == nil {
		return CleanupPreviewView{}, fmt.Errorf("cleanup preview not found")
	}
	if !s.now().Before(preview.ExpiresAt) {
		delete(s.previews, previewID)
		return CleanupPreviewView{}, fmt.Errorf("cleanup preview expired")
	}
	return preview.View, nil
}
