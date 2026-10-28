package processcontrol

import (
	"fmt"
	"time"
)

// ExecuteGracefulClose attempts to gracefully close a process via WM_CLOSE
func ExecuteGracefulClose(operationID string, store *OperationStore) OperationResult {
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

	// Send WM_CLOSE to all windows owned by this process
	windowCount, err := sendGracefulClose(preview.PID)
	if err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "failure",
			Details:     fmt.Sprintf("failed to send close message: %v", err),
			VerifiedAt:  time.Now(),
		}
	}

	if windowCount == 0 {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "partial",
			Details:     "no windows found to close",
			VerifiedAt:  time.Now(),
		}
	}

	// Wait briefly to observe outcome
	time.Sleep(500 * time.Millisecond)

	return OperationResult{
		OperationID: operationID,
		Success:     true,
		Outcome:     "success",
		Details:     fmt.Sprintf("sent WM_CLOSE to %d window(s)", windowCount),
		VerifiedAt:  time.Now(),
	}
}
