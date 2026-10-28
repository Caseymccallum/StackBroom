package processcontrol

import (
	"fmt"
	"time"
)

// ExecuteSetPriority changes the priority class of the verified target and
// verifies the new value by reading it back.
func ExecuteSetPriority(preview *OperationPreview, actions NativeActions) OperationResult {
	operationID := preview.OperationID

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

	handle, identity, err := actions.OpenVerified(preview.PID, PROCESS_SET_INFORMATION|PROCESS_QUERY_INFORMATION, preview.Target.CreateTime)
	if err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "refused",
			Details:     fmt.Sprintf("target could not be verified or opened: %v", err),
			VerifiedAt:  time.Now(),
		}
	}
	defer actions.Close(handle)

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

	previous, previousErr := actions.GetPriority(handle)
	previousText := "Unknown"
	if previousErr == nil {
		previousText = previous.String()
	}

	if err := actions.SetPriority(handle, preview.Priority); err != nil {
		return OperationResult{
			OperationID:   operationID,
			Success:       false,
			Outcome:       "failure",
			Details:       fmt.Sprintf("failed to set priority: %v", err),
			PreviousValue: previousText,
			VerifiedAt:    time.Now(),
		}
	}

	actual, err := actions.GetPriority(handle)
	if err != nil {
		return OperationResult{
			OperationID:   operationID,
			Success:       false,
			Outcome:       "partial",
			Details:       fmt.Sprintf("priority was set but readback failed: %v", err),
			PreviousValue: previousText,
			VerifiedAt:    time.Now(),
		}
	}
	if actual != preview.Priority {
		return OperationResult{
			OperationID:   operationID,
			Success:       false,
			Outcome:       "partial",
			Details:       fmt.Sprintf("priority readback mismatch: requested %s, observed %s", preview.Priority.String(), actual.String()),
			PreviousValue: previousText,
			NewValue:      actual.String(),
			VerifiedAt:    time.Now(),
		}
	}

	return OperationResult{
		OperationID:   operationID,
		Success:       true,
		Outcome:       "success",
		Details:       fmt.Sprintf("priority changed from %s to %s and verified", previousText, actual.String()),
		PreviousValue: previousText,
		NewValue:      actual.String(),
		VerifiedAt:    time.Now(),
	}
}
