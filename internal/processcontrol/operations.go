package processcontrol

import (
	"fmt"
	"time"
)

// ExecuteGracefulClose requests a graceful close through WM_CLOSE and verifies
// whether the process actually exited. A message delivery is not success.
func ExecuteGracefulClose(preview OperationPreview, actions NativeActions) OperationResult {
	operationID := preview.OperationID

	handle, identity, err := actions.OpenVerified(preview.PID, PROCESS_QUERY_LIMITED_INFORMATION|SYNCHRONIZE, preview.Target.CreateTime)
	if err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "refused",
			Details:     fmt.Sprintf("target could not be verified: %v", err),
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

	windowCount, err := actions.SendGracefulClose(preview.PID, preview.Target.CreateTime)
	if err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "failure",
			Details:     fmt.Sprintf("failed to deliver close message: %v", err),
			VerifiedAt:  time.Now(),
		}
	}
	if windowCount == 0 {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "partial",
			Details:     "no windows were found for this process; close request was not delivered",
			VerifiedAt:  time.Now(),
		}
	}

	exited, err := actions.Wait(handle, 10*time.Second)
	if err != nil {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "partial",
			Details:     fmt.Sprintf("close request delivered to %d window(s), but exit could not be verified: %v", windowCount, err),
			VerifiedAt:  time.Now(),
		}
	}
	if !exited {
		return OperationResult{
			OperationID: operationID,
			Success:     false,
			Outcome:     "timeout",
			Details:     fmt.Sprintf("close request delivered to %d window(s), but the process ignored it within 10s", windowCount),
			VerifiedAt:  time.Now(),
		}
	}

	return OperationResult{
		OperationID: operationID,
		Success:     true,
		Outcome:     "success",
		Details:     fmt.Sprintf("close request delivered to %d window(s); process exit verified", windowCount),
		VerifiedAt:  time.Now(),
	}
}
