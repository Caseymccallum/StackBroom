package processcontrol

import (
	"fmt"
	"time"
)

// ExecuteTerminate forcefully terminates a process
func ExecuteTerminate(operationID string, store *OperationStore) OperationResult {
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

	// Open process with terminate access
	handle, err := openProcess(preview.PID, PROCESS_TERMINATE)
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

	// Terminate process
	if err := terminateProcess(handle, 1); err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "failure",
			Details:     fmt.Sprintf("termination failed: %v", err),
			VerifiedAt:  time.Now(),
		}
	}

	// Wait up to 5 seconds for process to actually exit
	exited, err := waitForProcessExit(handle, 5000)
	if err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "partial",
			Details:     fmt.Sprintf("termination initiated but exit verification failed: %v", err),
			VerifiedAt:  time.Now(),
		}
	}

	if !exited {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "partial",
			Details:     "termination initiated but process did not exit within timeout",
			VerifiedAt:  time.Now(),
		}
	}

	return OperationResult{
		OperationID: operationID,
		Success:     true,
		Outcome:     "success",
		Details:     "process terminated and verified exited",
		VerifiedAt:  time.Now(),
	}
}
