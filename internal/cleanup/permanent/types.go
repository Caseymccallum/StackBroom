// Package permanent implements the narrow, irreversible crash-dump prototype.
// It is intentionally separate from the Recycle Bin execution path so one mode
// can never silently become the other.
package permanent

import (
	"context"
	"errors"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
)

var (
	ErrDisabled = errors.New("permanent cleanup is disabled in this build")
	ErrElevated = errors.New("cleanup is refused because StackBroom is running elevated")
)

const RuleID = "crash-dumps-old"

type State string

const (
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
	ItemValidated    ItemStatus = "validated"
	ItemDeleted      ItemStatus = "deleted"
	ItemRefused      ItemStatus = "refused"
	ItemChanged      ItemStatus = "changed"
	ItemStillPresent ItemStatus = "still_present"
	ItemUnknown      ItemStatus = "unknown"
	ItemCancelled    ItemStatus = "cancelled"
)

type ItemResult struct {
	TargetID            string
	Status              ItemStatus
	ReasonCode          string
	Message             string
	LogicalBytes        uint64
	AllocatedBytes      uint64
	AllocatedKnown      bool
	SpaceReclaimed      uint64
	SpaceReclaimedKnown bool
	RecoveryAvailable   bool
	Verified            bool
}

type Result struct {
	State       State
	Items       []ItemResult
	Aborted     bool
	StartedAt   time.Time
	CompletedAt time.Time
	ReasonCode  string
	Message     string
}

type Summary struct {
	Requested           int
	Processed           int
	Deleted             int
	Refused             int
	Changed             int
	StillPresent        int
	Unknown             int
	LogicalBytes        uint64
	AllocatedBytes      uint64
	SpaceReclaimedBytes uint64
	SpaceReclaimedKnown bool
	RecoveryAvailable   bool
}

type Progress struct {
	State     State
	TargetID  string
	Completed int
	Total     int
	Message   string
}

type Executor interface {
	Available() error
	Execute(ctx context.Context, targets []execution.Target, progress func(Progress)) (Result, error)
}

func (r Result) Finished() bool {
	switch r.State {
	case StateCompleted, StatePartial, StateFailed, StateCancelled:
		return true
	default:
		return false
	}
}

func (r Result) Summary() Summary {
	// SpaceReclaimed is an estimate from selected file allocation. The
	// prototype does not claim an exact volume-free-space delta.
	summary := Summary{Requested: len(r.Items), Processed: len(r.Items), SpaceReclaimedKnown: false}
	for _, item := range r.Items {
		summary.LogicalBytes += item.LogicalBytes
		summary.AllocatedBytes += item.AllocatedBytes
		switch item.Status {
		case ItemDeleted:
			summary.Deleted++
			if item.AllocatedKnown {
				summary.SpaceReclaimedBytes += item.SpaceReclaimed
			}
		case ItemRefused:
			summary.Refused++
		case ItemChanged:
			summary.Changed++
		case ItemStillPresent:
			summary.StillPresent++
		default:
			summary.Unknown++
		}
	}
	return summary
}
