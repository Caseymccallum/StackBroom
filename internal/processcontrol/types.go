package processcontrol

import "time"

// ProcessIdentity represents verified process identity for safe operations.
type ProcessIdentity struct {
	PID         int32
	Name        string
	ExePath     string
	CreateTime  time.Time
	SessionID   uint32
	IsProtected bool
	IsCritical  bool
}

// ProcessAction represents a requested operation on a process.
type ProcessAction string

const (
	ActionGracefulClose ProcessAction = "graceful_close"
	ActionTerminate     ProcessAction = "terminate"
	ActionSetPriority   ProcessAction = "set_priority"
)

// PriorityClass represents Windows process priority levels.
type PriorityClass int32

const (
	PriorityIdle        PriorityClass = 0x00000040
	PriorityBelowNormal PriorityClass = 0x00004000
	PriorityNormal      PriorityClass = 0x00000020
	PriorityAboveNormal PriorityClass = 0x00008000
	PriorityHigh        PriorityClass = 0x00000080
	PriorityRealTime    PriorityClass = 0x00000100 // NOT exposed to users
)

// PriorityClassString returns human-readable priority name.
func (p PriorityClass) String() string {
	switch p {
	case PriorityIdle:
		return "Idle"
	case PriorityBelowNormal:
		return "Below Normal"
	case PriorityNormal:
		return "Normal"
	case PriorityAboveNormal:
		return "Above Normal"
	case PriorityHigh:
		return "High"
	default:
		return "Unknown"
	}
}

// OperationPreview represents a pending operation awaiting confirmation.
type OperationPreview struct {
	OperationID string
	PID         int32
	Action      ProcessAction
	Target      ProcessIdentity
	Priority    PriorityClass // only for set_priority
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// OperationResult represents the outcome of an executed operation.
type OperationResult struct {
	OperationID string
	Success     bool
	Outcome     string // "success", "timeout", "refused", "partial", "failure"
	Details     string
	VerifiedAt  time.Time
}

// RefusalReason explains why an operation was refused.
type RefusalReason string

const (
	RefusalNotFound      RefusalReason = "process_not_found"
	RefusalPIDReuse      RefusalReason = "pid_reuse_detected"
	RefusalProtected     RefusalReason = "protected_process"
	RefusalCritical      RefusalReason = "critical_system_process"
	RefusalSelf          RefusalReason = "stackbroom_process"
	RefusalInsufficient  RefusalReason = "insufficient_privileges"
	RefusalExpired       RefusalReason = "operation_expired"
	RefusalInvalid       RefusalReason = "invalid_operation_id"
)
