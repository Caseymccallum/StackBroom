package permanent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup"
	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
	"github.com/Caseymccallum/StackBroom/internal/storage"
)

type Engine struct {
	executor     Executor
	recorder     execution.Recorder
	now          func() time.Time
	ruleResolver func(string) (cleanup.Rule, bool)
	rootResolver func(cleanup.Rule) (string, error)
	stat         func(string) (bool, error)
}

func NewEngine(executor Executor, recorder execution.Recorder) *Engine {
	return &Engine{
		executor:     executor,
		recorder:     recorder,
		now:          time.Now,
		ruleResolver: cleanup.RuleByID,
		rootResolver: func(rule cleanup.Rule) (string, error) { return rule.ResolveRoot() },
		stat: func(path string) (bool, error) {
			_, err := os.Lstat(path)
			if err == nil {
				return true, nil
			}
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		},
	}
}

func (e *Engine) Available() error {
	if e == nil || e.executor == nil {
		return fmt.Errorf("permanent cleanup executor is unavailable")
	}
	return e.executor.Available()
}

// Execute is the narrow permanent-delete gate. It accepts only the
// crash-dumps-old rule and exact targets carried by a permanent-only context.
// The executor performs the handle-bound operation; this engine verifies the
// resulting path state before reporting success.
func (e *Engine) Execute(ctx context.Context, executionContext *execution.Context, progress func(Progress)) (result Result) {
	started := e.now()
	result = Result{State: StateValidating, StartedAt: started}
	defer func() {
		result.CompletedAt = e.now()
		if e.recorder != nil {
			summary := result.Summary()
			e.recorder.Record(execution.Record{
				At: result.CompletedAt, Type: "execution", RuleID: executionRuleID(executionContext),
				RuleVersion: executionRuleVersion(executionContext), State: execution.State(result.State),
				TargetCount: summary.Requested, LogicalBytes: summary.LogicalBytes,
				AllocatedBytes: summary.AllocatedBytes, DeletionMode: "permanent",
				ReasonCode: result.ReasonCode, RecoveryAvailable: false,
			})
		}
	}()
	if executionContext == nil {
		return failed(result, "missing_execution", "cleanup context is missing")
	}
	targets := executionContext.Targets()
	result.Requested = len(targets)
	if ctx.Err() != nil {
		result.State, result.ReasonCode = StateCancelled, "cancelled"
		return result
	}
	if err := executionContext.Consume(started); err != nil {
		return failed(result, "context", err.Error())
	}
	if executionContext.Permission() != execution.PermanentOnly {
		return failed(result, "permission", "this cleanup mode is not permanent deletion")
	}
	rule, ok := e.ruleResolver(executionContext.RuleID())
	if !ok || rule.ID != RuleID || rule.ID != executionContext.RuleID() || rule.Version != executionContext.RuleVersion() {
		return failed(result, "rule_changed", "the crash-dump cleanup rule is no longer available")
	}
	root, err := e.rootResolver(rule)
	if err != nil || !samePath(root, executionContext.Root()) {
		return failed(result, "scope_changed", "the crash-dump folder is no longer the trusted cleanup scope")
	}
	if err := e.Available(); err != nil {
		return failed(result, "execution_unavailable", "permanent cleanup is not available in this build")
	}
	if len(targets) == 0 {
		return failed(result, "no_targets", "no crash dumps were selected")
	}
	items := make([]ItemResult, 0, len(targets))
	for _, target := range targets {
		if ctx.Err() != nil {
			result.State, result.ReasonCode = StateCancelled, "cancelled"
			return result
		}
		if target.ID == "" || target.File.FileID == "" {
			return failed(result, "target_changed", "a selected file has no stable identity")
		}
		if !target.File.RootEqual(root) || target.File.Name != filepath.Base(target.File.Path) ||
			!strings.EqualFold(filepath.Ext(target.File.Name), ".dmp") ||
			rule.ValidateEligibility(target.File, len(targets), e.now()) != nil {
			return failed(result, "target_changed", "a selected file is no longer eligible for crash-dump cleanup")
		}
		if err := storage.ValidateCleanupTarget(&target.File); err != nil {
			return failed(result, "target_changed", "a selected file changed before cleanup")
		}
		items = append(items, ItemResult{
			TargetID: target.ID, Status: ItemValidated,
			LogicalBytes: target.File.LogicalBytes, AllocatedBytes: target.File.AllocatedBytes,
			AllocatedKnown: target.File.AllocatedKnown, RecoveryAvailable: false,
		})
	}
	result.Items = items
	result.State = StateExecuting
	if progress != nil {
		progress(Progress{State: StateExecuting, Completed: 0, Total: len(targets)})
	}
	executionResult, executeErr := e.executor.Execute(ctx, targets, progress)
	if executeErr != nil {
		result.ReasonCode = "executor_failed"
		result.Message = executeErr.Error()
	}
	result.State = StateVerifying
	byTarget := make(map[string]ItemResult, len(executionResult.Items))
	for _, item := range executionResult.Items {
		byTarget[item.TargetID] = item
	}
	verified := 0
	for index, target := range targets {
		item := result.Items[index]
		executorItem, present := byTarget[target.ID]
		item.RecoveryAvailable = false
		if present {
			item.ReasonCode = executorItem.ReasonCode
			item.Message = executorItem.Message
		}
		exists, statErr := e.stat(target.File.Path)
		if statErr != nil {
			item.Status = ItemUnknown
			item.ReasonCode = "verification_failed"
			item.Message = "the final filesystem state could not be checked"
			result.Items[index] = item
			continue
		}
		if exists {
			if present && (executorItem.Status == ItemRefused || executorItem.Status == ItemChanged) {
				item.Status = executorItem.Status
			} else {
				item.Status = ItemStillPresent
				item.ReasonCode = "still_present"
				item.Message = "the file is still present"
			}
			result.Items[index] = item
			continue
		}
		if !present || executorItem.Status != ItemDeleted || !executorItem.Verified {
			if present && (executorItem.Status == ItemRefused || executorItem.Status == ItemChanged) {
				item.Status = executorItem.Status
			} else {
				item.Status = ItemUnknown
				item.ReasonCode = "outcome_unverified"
				item.Message = "the file is absent but deletion was not verified by the executor"
			}
			result.Items[index] = item
			continue
		}
		item.Status = ItemDeleted
		item.ReasonCode = "deleted"
		item.Message = "the file was permanently deleted"
		item.SpaceReclaimed = target.File.AllocatedBytes
		item.SpaceReclaimedKnown = false
		item.Verified = true
		verified++
		result.Items[index] = item
	}
	switch {
	case ctx.Err() != nil:
		result.State = StateCancelled
		result.ReasonCode = "cancelled"
	case verified == len(targets) && executeErr == nil:
		result.State = StateCompleted
	case verified > 0:
		result.State = StatePartial
		if result.ReasonCode == "" {
			result.ReasonCode = "partial"
		}
	default:
		result.State = StateFailed
		if result.ReasonCode == "" {
			result.ReasonCode = "no_verified_deletion"
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

func executionRuleID(executionContext *execution.Context) string {
	if executionContext == nil {
		return ""
	}
	return executionContext.RuleID()
}

func executionRuleVersion(executionContext *execution.Context) string {
	if executionContext == nil {
		return ""
	}
	return executionContext.RuleVersion()
}
