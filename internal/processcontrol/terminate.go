package processcontrol

import (
	"fmt"
	"time"
)

// ExecuteTerminate forcefully terminates the verified target.
func ExecuteTerminate(preview OperationPreview, actions NativeActions) OperationResult {
	operationID := preview.OperationID

	handle, identity, err := actions.OpenVerified(preview.PID, PROCESS_TERMINATE|SYNCHRONIZE, preview.Target.CreateTime)
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

	if err := actions.Terminate(handle, 1); err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "failure",
			Details:     fmt.Sprintf("termination failed: %v", err),
			VerifiedAt:  time.Now(),
		}
	}

	exited, err := actions.Wait(handle, 5*time.Second)
	if err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "partial",
			Details:     fmt.Sprintf("termination initiated but exit could not be verified: %v", err),
			VerifiedAt:  time.Now(),
		}
	}
	if !exited {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "partial",
			Details:     "termination initiated but process did not exit within 5s",
			VerifiedAt:  time.Now(),
		}
	}

	return OperationResult{
		OperationID: operationID,
		Success:     true,
		Outcome:     "success",
		Details:     "process exit verified after termination",
		VerifiedAt:  time.Now(),
	}
}
