package processcontrol

import (
	"fmt"
	"time"
)

// ExecuteSetPriority changes the priority class of a process
func ExecuteSetPriority(operationID string, store *OperationStore) OperationResult {
	preview, err := store.ConsumePreview(operationID)
	if err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "refused",
			Details:     err.Error(),
			VerifiedAt:  time.Now(),
		}
	}

	// Validate priority class (reject Real-time)
	if preview.Priority != PriorityIdle &&
		preview.Priority != PriorityBelowNormal &&
		preview.Priority != PriorityNormal &&
		preview.Priority != PriorityAboveNormal &&
		preview.Priority != PriorityHigh {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "refused",
			Details:     "unsupported priority class (Real-time not allowed)",
			VerifiedAt:  time.Now(),
		}
	}

	// Re-verify process identity
	identity, err := VerifyProcessIdentity(preview.PID, preview.Target.CreateTime)
	if err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "refused",
			Details:     fmt.Sprintf("identity verification failed: %v", err),
			VerifiedAt:  time.Now(),
		}
	}

	// Check eligibility
	if identity.IsProtected || identity.IsCritical {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "refused",
			Details:     "protected or critical system process",
			VerifiedAt:  time.Now(),
		}
	}

	if IsStackBroomProcess(identity.Name) {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "refused",
			Details:     "StackBroom process",
			VerifiedAt:  time.Now(),
		}
	}

	// Open process with set information access
	handle, err := openProcess(preview.PID, PROCESS_SET_INFORMATION|PROCESS_QUERY_INFORMATION)
	if err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "refused",
			Details:     fmt.Sprintf("insufficient privileges: %v", err),
			VerifiedAt:  time.Now(),
		}
	}
	defer closeHandle(handle)

	// Set priority
	if err := setPriorityClass(handle, preview.Priority); err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "failure",
			Details:     fmt.Sprintf("failed to set priority: %v", err),
			VerifiedAt:  time.Now(),
		}
	}

	// Readback verification
	actualPriority, err := getPriorityClass(handle)
	if err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "partial",
			Details:     fmt.Sprintf("priority set but readback failed: %v", err),
			VerifiedAt:  time.Now(),
		}
	}

	if actualPriority != preview.Priority {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "partial",
			Details:     fmt.Sprintf("priority mismatch: requested %v, got %v", preview.Priority, actualPriority),
			VerifiedAt:  time.Now(),
		}
	}

	return OperationResult{
		OperationID: operationID,
		Success:     true,
		Outcome:     "success",
		Details:     fmt.Sprintf("priority set to %s", preview.Priority.String()),
		VerifiedAt:  time.Now(),
	}
}
