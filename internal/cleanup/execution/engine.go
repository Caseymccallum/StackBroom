package execution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup"
	"github.com/Caseymccallum/StackBroom/internal/storage"
)

type Engine struct {
	executor     Executor
	verifier     RecoveryVerifier
	preflight    NoFallbackPreflight
	finalGuard   FinalFileGuard
	receipts     ReceiptProvider
	recorder     Recorder
	now          func() time.Time
	ruleResolver func(string) (cleanup.Rule, bool)
	rootResolver func(cleanup.Rule) (string, error)
}

func (e *Engine) Available() error {
	if e == nil || e.executor == nil {
		return fmt.Errorf("cleanup executor is unavailable")
	}
	if e.verifier == nil || e.verifier.Available() != nil {
		return fmt.Errorf("recovery verifier is unavailable")
	}
	if e.preflight == nil || e.preflight.Available() != nil {
		return fmt.Errorf("no-fallback preflight is unavailable")
	}
	if e.finalGuard == nil || e.finalGuard.Available() != nil {
		return fmt.Errorf("final-file guard is unavailable")
	}
	if e.receipts == nil || e.receipts.Available() != nil {
		return fmt.Errorf("recovery receipt provider is unavailable")
	}
	return nil
}

func NewEngine(executor Executor, verifier RecoveryVerifier, recorder Recorder) *Engine {
	return &Engine{
		executor: executor, verifier: verifier, recorder: recorder, now: time.Now,
		preflight: NewUnavailablePreflight(), finalGuard: NewUnavailableFinalGuard(),
		receipts:     NewUnavailableReceiptProvider(),
		ruleResolver: cleanup.RuleByID,
		rootResolver: func(rule cleanup.Rule) (string, error) { return rule.ResolveRoot() },
	}
}

func (e *Engine) Execute(ctx context.Context, execution *Context, progress func(Progress)) (result Result) {
	started := e.now()
	result = Result{State: StateValidating, States: []State{StatePreviewed, StateConfirmed, StateValidating}, StartedAt: started}
	defer func() {
		result.CompletedAt = e.now()
		if len(result.States) == 0 || result.States[len(result.States)-1] != result.State {
			result.States = append(result.States, result.State)
		}
		if e.recorder != nil {
			e.recorder.Record(Record{
				At: result.CompletedAt, Type: "execution", RuleID: executionRuleID(execution),
				RuleVersion: executionRuleVersion(execution), State: result.State,
				TargetCount: len(result.Items), LogicalBytes: sumLogical(result.Items),
				AllocatedBytes: sumAllocated(result.Items), DeletionMode: "recycle",
				ReasonCode: result.ReasonCode, RecoveryAvailable: result.State == StateCompleted,
			})
		}
	}()
	if execution == nil {
		return failed(result, "missing_execution", "execution context is missing")
	}
	if ctx.Err() != nil {
		result.State, result.ReasonCode = StateCancelled, "cancelled"
		return result
	}
	if err := execution.Consume(started); err != nil {
		return failed(result, "context", err.Error())
	}
	rule, ok := e.ruleResolver(execution.RuleID())
	if !ok || rule.ID != execution.RuleID() || rule.Version != execution.RuleVersion() {
		return failed(result, "rule_changed", "cleanup rule is no longer available at the previewed version")
	}
	if rule.DeletionMode != "recycle" {
		return failed(result, "deletion_mode", "only recoverable cleanup is supported")
	}
	if len(rule.RequiresClosed) > 0 {
		return failed(result, "prerequisite_unverified", "application-close prerequisites cannot be verified")
	}
	root, err := e.rootResolver(rule)
	if err != nil || !samePath(root, execution.Root()) {
		return failed(result, "scope_changed", "cleanup scope is no longer the trusted rule root")
	}
	if e.executor == nil {
		return failed(result, "execution_unavailable", "cleanup executor is unavailable")
	}
	if e.verifier == nil {
		return failed(result, "recovery_unavailable", "recovery verification is unavailable")
	}
	if err := e.verifier.Available(); err != nil {
		return failed(result, "recovery_unavailable", "recovery verification is unavailable")
	}
	if e.preflight == nil || e.preflight.Available() != nil {
		return failed(result, "recycle_preflight_unavailable", "no-permanent-fallback preflight is unavailable")
	}
	if e.finalGuard == nil || e.finalGuard.Available() != nil {
		return failed(result, "final_file_guard_unavailable", "final-file identity guard is unavailable")
	}
	if e.receipts == nil || e.receipts.Available() != nil {
		return failed(result, "recovery_receipt_unavailable", "durable recovery receipt provider is unavailable")
	}
	targets := execution.Targets()
	result.Items = make([]ItemResult, 0, len(targets))
	for _, target := range targets {
		evidence, preflightErr := e.preflight.Check(ctx, target)
		if preflightErr != nil || !evidence.Supported || !evidence.PolicyKnown || !evidence.CapacityKnown ||
			evidence.Provider == "" || evidence.ObservedAt.IsZero() || !e.now().Before(evidence.ExpiresAt) {
			return failed(result, "permanent_fallback_unproven", "Recycle Bin capacity/policy cannot be proven without a permanent-delete fallback")
		}
		if ctx.Err() != nil {
			result.State, result.ReasonCode = StateCancelled, "cancelled"
			return result
		}
		if !target.File.RootEqual(root) || rule.ValidateEligibility(target.File, len(targets), e.now()) != nil {
			return failed(result, "target_changed", "target is no longer eligible under the rule")
		}
		item := ItemResult{
			TargetID: target.ID, Status: ItemValidated,
			LogicalBytes: target.File.LogicalBytes, AllocatedBytes: target.File.AllocatedBytes,
			AllocatedKnown: target.File.AllocatedKnown,
		}
		if err := storage.ValidateCleanupTarget(&target.File); err != nil {
			item.Status = ItemChanged
			item.ReasonCode = storage.ValidateTargetErrorCode(err)
			item.Message = err.Error()
			result.Items = append(result.Items, item)
			return failed(result, "target_changed", "one or more targets changed before execution; nothing was executed")
		}
		if !rule.AllowsExtension(target.File.Name) {
			item.Status = ItemChanged
			item.ReasonCode = "extension"
			item.Message = "target extension no longer matches the rule"
			result.Items = append(result.Items, item)
			return failed(result, "target_changed", "one or more targets changed before execution; nothing was executed")
		}
		if e.now().Sub(target.File.ModTime) < time.Duration(rule.MinAgeDays)*24*time.Hour {
			item.Status = ItemChanged
			item.ReasonCode = "age"
			item.Message = "target is no longer old enough"
			result.Items = append(result.Items, item)
			return failed(result, "target_changed", "one or more targets changed before execution; nothing was executed")
		}
		result.Items = append(result.Items, item)
	}
	guards := make([]Guard, 0, len(targets))
	defer func() {
		for _, guard := range guards {
			_ = guard.Close()
		}
	}()
	for _, target := range targets {
		guard, guardErr := e.finalGuard.Acquire(ctx, target)
		if guardErr != nil || guard == nil {
			return failed(result, "final_file_guard_unavailable", "final-file identity could not be held through Shell processing")
		}
		guards = append(guards, guard)
	}
	result.State = StateExecuting
	result.States = append(result.States, StateExecuting)
	if progress != nil {
		progress(Progress{State: StateExecuting, Completed: 0, Total: len(targets)})
	}
	if ctx.Err() != nil {
		result.State, result.ReasonCode = StateCancelled, "cancelled"
		return result
	}
	executionResult, executeErr := e.executor.Execute(ctx, execution, targets, progress)
	result.Aborted = executionResult.Aborted
	if executeErr != nil {
		result.ReasonCode = "executor_failed"
		result.Message = executeErr.Error()
	}
	result.State = StateVerifying
	result.States = append(result.States, StateVerifying)
	byTarget := make(map[string]ItemResult, len(executionResult.Items))
	for _, item := range executionResult.Items {
		byTarget[item.TargetID] = item
	}
	verified := 0
	for index := range result.Items {
		target := targets[index]
		item := result.Items[index]
		executorItem, present := byTarget[target.ID]
		item.HRESULT = executorItem.HRESULT
		if present && executorItem.Status == ItemDenied {
			item.Status = ItemDenied
			item.ReasonCode = executorItem.ReasonCode
			item.Message = executorItem.Message
			result.Items[index] = item
			continue
		}
		if _, statErr := os.Lstat(target.File.Path); statErr == nil {
			item.Status = ItemStillPresent
			item.ReasonCode = "still_present"
			item.Message = "target is still present"
			if present && (executorItem.Status == ItemFailed || executorItem.Status == ItemCancelled || executorItem.Status == ItemUnknown) {
				item.Status, item.ReasonCode, item.Message = executorItem.Status, executorItem.ReasonCode, executorItem.Message
			} else if ctx.Err() != nil || result.Aborted {
				item.Status = ItemCancelled
				item.ReasonCode = "cancelled"
			}
			result.Items[index] = item
			continue
		} else if !os.IsNotExist(statErr) {
			item.Status = ItemUnknown
			item.ReasonCode = "verification_failed"
			item.Message = "post-operation presence could not be determined"
			result.Items[index] = item
			continue
		}
		evidence, evidenceErr := e.verifier.Verify(ctx, target)
		switch {
		case evidenceErr != nil:
			item.Status = ItemUnknown
			item.ReasonCode = "recovery_verification_failed"
			item.Message = "target left its path but recovery could not be verified"
		case !evidence.Available || evidence.Kind == "" || evidence.Token == "" || !present || executorItem.Status != ItemRecycled:
			item.Status = ItemRemovedUnverified
			item.ReasonCode = "recovery_unverified"
			item.Message = "target left its path but Recycle Bin evidence is unavailable"
		default:
			receipt, receiptErr := e.receipts.Capture(ctx, target, evidence, e.now())
			if receiptErr != nil || receipt.TargetID != target.ID || receipt.OriginalFileID != target.File.FileID ||
				receipt.Provider == "" || receipt.OpaqueToken == "" || receipt.ObservedAt.IsZero() ||
				!e.now().Before(receipt.ExpiresAt) {
				item.Status = ItemRemovedUnverified
				item.ReasonCode = "recovery_receipt_unavailable"
				item.Message = "target left its path but durable recovery evidence could not be stored"
			} else {
				item.Status = ItemRecycled
				item.ReasonCode = "recycled"
				item.Recovery = evidence
				verified++
			}
		}
		result.Items[index] = item
	}
	switch {
	case ctx.Err() != nil:
		result.State = StateCancelled
		result.ReasonCode = "cancelled"
	case verified == len(result.Items) && executeErr == nil:
		result.State = StateCompleted
		if result.Aborted {
			result.State = StatePartial
			result.ReasonCode = "aborted"
		}
	case verified > 0:
		result.State = StatePartial
		if result.ReasonCode == "" {
			result.ReasonCode = "partial"
		}
	default:
		result.State = StateFailed
		if result.ReasonCode == "" {
			result.ReasonCode = "no_verified_recovery"
		}
	}
	return result
}

func failed(result Result, code, message string) Result {
	result.State = StateFailed
	result.ReasonCode = code
	result.Message = message
	return result
}

func samePath(first, second string) bool {
	if first == "" || second == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(first), filepath.Clean(second))
}

func sumLogical(items []ItemResult) uint64 {
	var total uint64
	for _, item := range items {
		total += item.LogicalBytes
	}
	return total
}

func sumAllocated(items []ItemResult) uint64 {
	var total uint64
	for _, item := range items {
		if item.AllocatedKnown {
			total += item.AllocatedBytes
		}
	}
	return total
}

func executionRuleID(execution *Context) string {
	if execution == nil {
		return ""
	}
	return execution.RuleID()
}

func executionRuleVersion(execution *Context) string {
	if execution == nil {
		return ""
	}
	return execution.RuleVersion()
}
