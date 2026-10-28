package execution

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrDisabled = errors.New("cleanup execution is disabled in this build")

func BuildEnabled() bool { return executionBuildEnabled }

type State string

const (
	StatePreviewed  State = "previewed"
	StateConfirmed  State = "confirmed"
	StateValidating State = "validating"
	StateExecuting  State = "executing"
	StateVerifying  State = "verifying"
	StateCompleted  State = "completed"
	StatePartial    State = "partial"
	StateFailed     State = "failed"
	StateCancelled  State = "cancelled"
)

type ItemStatus string

const (
	ItemValidated         ItemStatus = "validated"
	ItemQueued            ItemStatus = "queued"
	ItemRecycled          ItemStatus = "recycled"
	ItemRemovedUnverified ItemStatus = "removed_unverified"
	ItemStillPresent      ItemStatus = "still_present"
	ItemChanged           ItemStatus = "changed"
	ItemDenied            ItemStatus = "access_denied"
	ItemFailed            ItemStatus = "failed"
	ItemCancelled         ItemStatus = "cancelled"
	ItemUnknown           ItemStatus = "unknown"
)

type RecoveryEvidence struct {
	Available bool
	Kind      string
	Token     string
	Note      string
}

type ItemResult struct {
	TargetID       string
	Status         ItemStatus
	ReasonCode     string
	Message        string
	HRESULT        uint32
	Recovery       RecoveryEvidence
	LogicalBytes   uint64
	AllocatedBytes uint64
	AllocatedKnown bool
}

type Result struct {
	State       State
	States      []State
	Items       []ItemResult
	Aborted     bool
	StartedAt   time.Time
	CompletedAt time.Time
	ReasonCode  string
	Message     string
}

type Progress struct {
	State     State
	TargetID  string
	Completed int
	Total     int
	Message   string
}

type Executor interface {
	Execute(ctx context.Context, execution *Context, targets []Target, progress func(Progress)) (Result, error)
}

type RecoveryVerifier interface {
	Available() error
	Verify(ctx context.Context, target Target) (RecoveryEvidence, error)
}

// PreflightEvidence is read-only proof needed before a recoverable operation.
// CapacityKnown and PolicyKnown are deliberately separate from Supported:
// a Recycle flag or a successful API call is not a no-fallback guarantee.
type PreflightEvidence struct {
	Supported     bool
	PolicyKnown   bool
	CapacityKnown bool
	Provider      string
	ObservedAt    time.Time
	ExpiresAt     time.Time
	ReasonCode    string
}

type NoFallbackPreflight interface {
	Available() error
	Check(ctx context.Context, target Target) (PreflightEvidence, error)
}

type Guard interface {
	Close() error
}

// FinalFileGuard would hold an identity-bound final-file guard through Shell
// processing. Ordinary builds and the current native implementation refuse.
type FinalFileGuard interface {
	Available() error
	Acquire(ctx context.Context, target Target) (Guard, error)
}

type ReceiptProvider interface {
	Available() error
	Capture(ctx context.Context, target Target, evidence RecoveryEvidence, now time.Time) (RecycleReceipt, error)
	Rebind(ctx context.Context, receipt RecycleReceipt, target Target, now time.Time) (RecoveryEvidence, error)
}

type Recorder interface {
	Record(record Record)
}

type Record struct {
	At                time.Time
	Type              string
	RuleID            string
	RuleVersion       string
	State             State
	TargetCount       int
	LogicalBytes      uint64
	AllocatedBytes    uint64
	DeletionMode      string
	ReasonCode        string
	RecoveryAvailable bool
}

type disabledExecutor struct{}

func (disabledExecutor) Execute(context.Context, *Context, []Target, func(Progress)) (Result, error) {
	return Result{State: StateFailed, ReasonCode: "execution_disabled", Message: ErrDisabled.Error()}, ErrDisabled
}

// NewDisabledExecutor is the ordinary-build executor. It has no filesystem or
// COM mutation path.
func NewDisabledExecutor() Executor { return disabledExecutor{} }

func (r Result) Finished() bool {
	switch r.State {
	case StateCompleted, StatePartial, StateFailed, StateCancelled:
		return true
	default:
		return false
	}
}

func (r Result) String() string {
	return fmt.Sprintf("%s: %d item(s)", r.State, len(r.Items))
}
