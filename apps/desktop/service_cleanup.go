package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup"
	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
	"github.com/Caseymccallum/StackBroom/internal/history"
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
	Execution *execution.Context
}

// CleanupService discovers candidates and keeps execution gated in Go.
type CleanupService struct {
	mu                sync.Mutex
	discoveries       map[string]*cleanupDiscoveryState
	previews          map[string]*cleanupPreviewState
	active            string
	now               func() time.Time
	discoverFn        func(context.Context, cleanup.Rule, int, func(cleanup.Progress), func() time.Time) (*cleanup.Discovery, error)
	executor          *execution.Engine
	historyStore      *history.Store
	running           map[string]context.CancelFunc
	executionProgress map[string]execution.Progress
	closed            bool
}

func NewCleanupService() *CleanupService {
	historyPath := ""
	if config, err := os.UserConfigDir(); err == nil {
		historyPath = filepath.Join(config, "StackBroom", "cleanup-history.jsonl")
	}
	store := history.NewStore(historyPath, 30, 500)
	return &CleanupService{
		discoveries:       make(map[string]*cleanupDiscoveryState),
		previews:          make(map[string]*cleanupPreviewState),
		now:               time.Now,
		discoverFn:        cleanup.Discover,
		executor:          execution.NewEngine(execution.NewNativeExecutor(), execution.NewRecoveryVerifier(), store),
		historyStore:      store,
		running:           make(map[string]context.CancelFunc),
		executionProgress: make(map[string]execution.Progress),
	}
}

func cleanupExecutionEnabled() bool {
	return execution.BuildEnabled() && os.Getenv("STACKBROOM_ENABLE_CLEANUP_EXECUTION") == "1"
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
	if s.closed {
		s.mu.Unlock()
		return "", fmt.Errorf("cleanup service is shutting down")
	}
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
		ExecutionAvailable: cleanupExecutionEnabled() && execution.NewRecoveryVerifier().Available() == nil,
		RecoveryNote:       preview.RecoveryNote, Risks: append([]string(nil), preview.Risks...),
		SideEffects: append([]string(nil), preview.SideEffects...), ExpiresAt: preview.ExpiresAt,
	}
	for _, candidate := range preview.Selected {
		view.Selected = append(view.Selected, mapCleanupCandidate(candidate))
	}
	for _, exclusion := range preview.Excluded {
		view.Excluded = append(view.Excluded, CleanupExclusion{RelativePath: exclusion.RelativePath, Reason: exclusion.Reason})
	}
	targets := make([]execution.Target, 0, len(preview.Selected))
	for _, candidate := range preview.Selected {
		targets = append(targets, execution.Target{ID: candidate.ID, File: candidate.File})
	}
	executionContext, err := execution.NewContext(id, preview.Rule.ID, preview.Rule.Version, result.Root, targets, preview.ExpiresAt, execution.RecycleOnly)
	if err != nil {
		return CleanupPreviewView{}, fmt.Errorf("cleanup execution context was refused: %w", err)
	}
	s.mu.Lock()
	s.prunePreviewsLocked()
	s.previews[id] = &cleanupPreviewState{View: cloneCleanupPreview(view), ExpiresAt: preview.ExpiresAt, Execution: executionContext}
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
	return cloneCleanupPreview(preview.View), nil
}

func cloneCleanupPreview(view CleanupPreviewView) CleanupPreviewView {
	view.Selected = append([]CleanupCandidate(nil), view.Selected...)
	view.Excluded = append([]CleanupExclusion(nil), view.Excluded...)
	view.Risks = append([]string(nil), view.Risks...)
	view.SideEffects = append([]string(nil), view.SideEffects...)
	return view
}

// Cancellation is cooperative: an in-flight Shell call is never terminated.
func (s *CleanupService) CancelCleanupExecution(previewID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cancel := s.running[previewID]
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (s *CleanupService) GetCleanupExecutionProgress(previewID string) (execution.Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.executionProgress[previewID]
	if !ok {
		return p, fmt.Errorf("cleanup execution not found")
	}
	return p, nil
}

// ServiceShutdown is called by Wails without creating any UI.
func (s *CleanupService) ServiceShutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for _, cancel := range s.running {
		cancel()
	}
	for _, state := range s.discoveries {
		if state.Cancel != nil {
			state.Cancel()
		}
	}
	return nil
}

// ConfirmCleanup is deliberately gated at the Go boundary. In an ordinary
// build there is no compiled native executor; even a hostile webview cannot
// enable deletion with an environment variable.
func (s *CleanupService) ConfirmCleanup(previewID string) (CleanupExecutionView, error) {
	if !execution.BuildEnabled() {
		return CleanupExecutionView{}, fmt.Errorf("cleanup execution is disabled in this build")
	}
	if !cleanupExecutionEnabled() {
		return CleanupExecutionView{}, fmt.Errorf("cleanup execution is disabled by the backend feature gate")
	}
	s.mu.Lock()
	preview := s.previews[previewID]
	if s.closed || len(s.running) > 0 {
		s.mu.Unlock()
		return CleanupExecutionView{}, fmt.Errorf("cleanup service is busy or shutting down")
	}
	if preview == nil || preview.Execution == nil {
		s.mu.Unlock()
		return CleanupExecutionView{}, fmt.Errorf("cleanup execution context not found")
	}
	executionContext := preview.Execution
	if !s.now().Before(preview.ExpiresAt) {
		s.mu.Unlock()
		return CleanupExecutionView{}, fmt.Errorf("cleanup execution context expired")
	}
	if s.executor == nil {
		s.mu.Unlock()
		return CleanupExecutionView{}, fmt.Errorf("cleanup executor is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	if len(s.executionProgress) >= 20 {
		for id := range s.executionProgress {
			if s.running[id] == nil {
				delete(s.executionProgress, id)
				break
			}
		}
	}
	s.running[previewID] = cancel
	s.executionProgress[previewID] = execution.Progress{State: execution.StateValidating}
	delete(s.previews, previewID)
	s.mu.Unlock()
	defer func() { cancel(); s.mu.Lock(); delete(s.running, previewID); s.mu.Unlock() }()
	result := s.executor.Execute(ctx, executionContext, func(p execution.Progress) { s.mu.Lock(); s.executionProgress[previewID] = p; s.mu.Unlock() })
	s.mu.Lock()
	terminal := s.executionProgress[previewID]
	terminal.State = result.State
	terminal.Total = len(result.Items)
	s.executionProgress[previewID] = terminal
	s.mu.Unlock()
	view := CleanupExecutionView{
		PreviewID: previewID, State: string(result.State), Aborted: result.Aborted,
		States: func() []string {
			states := make([]string, 0, len(result.States))
			for _, state := range result.States {
				states = append(states, string(state))
			}
			return states
		}(),
		ReasonCode: result.ReasonCode, Message: result.Message,
		StartedAt: result.StartedAt, CompletedAt: result.CompletedAt,
	}
	for _, item := range result.Items {
		view.Items = append(view.Items, CleanupExecutionItem{
			TargetID: item.TargetID, Status: string(item.Status), ReasonCode: item.ReasonCode,
			Message: item.Message, HRESULT: item.HRESULT,
			RecoveryAvailable: item.Recovery.Available, RecoveryKind: item.Recovery.Kind,
			LogicalBytes: item.LogicalBytes, AllocatedBytes: item.AllocatedBytes, AllocatedKnown: item.AllocatedKnown,
		})
	}
	return view, nil
}

func (s *CleanupService) ListCleanupHistory() (CleanupHistoryView, error) {
	if s.historyStore == nil {
		return CleanupHistoryView{}, nil
	}
	entries, err := s.historyStore.List()
	if err != nil {
		return CleanupHistoryView{}, fmt.Errorf("cleanup history could not be read")
	}
	view := CleanupHistoryView{RetentionDays: 30, MaxRecords: 500}
	for _, entry := range entries {
		view.Records = append(view.Records, CleanupHistoryRecord{
			At: entry.At, Type: entry.Type, RuleID: entry.RuleID, RuleVersion: entry.RuleVersion,
			State: entry.State, TargetCount: entry.TargetCount, LogicalBytes: entry.LogicalBytes,
			AllocatedBytes: entry.AllocatedBytes, DeletionMode: entry.DeletionMode,
			ReasonCode: entry.ReasonCode, RecoveryAvailable: entry.RecoveryAvailable,
		})
	}
	return view, nil
}

func (s *CleanupService) ClearCleanupHistory() error {
	if s.historyStore == nil {
		return nil
	}
	if err := s.historyStore.Clear(); err != nil {
		return fmt.Errorf("cleanup history could not be cleared")
	}
	return nil
}
