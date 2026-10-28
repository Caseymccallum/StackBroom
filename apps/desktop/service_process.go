package main

import (
	"fmt"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/processcontrol"
)

// ProcessControlService handles process management operations
type ProcessControlService struct {
	store *processcontrol.OperationStore
}

// NewProcessControlService creates a new process control service
func NewProcessControlService() *ProcessControlService {
	return &ProcessControlService{
		store: processcontrol.NewOperationStore(),
	}
}

// GetProcessDetails returns detailed information about a specific process
func (s *ProcessControlService) GetProcessDetails(pid int32, createTime string) (map[string]interface{}, error) {
	// Parse creation time
	creationTime, err := time.Parse(time.RFC3339, createTime)
	if err != nil {
		return nil, err
	}

	// Verify identity
	identity, err := processcontrol.VerifyProcessIdentity(pid, creationTime)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"pid":         identity.PID,
		"name":        identity.Name,
		"exePath":     identity.ExePath,
		"createTime":  identity.CreateTime.Format(time.RFC3339),
		"isProtected": identity.IsProtected,
		"isCritical":  identity.IsCritical,
	}, nil
}

// PreviewProcessAction creates a preview for a process action
func (s *ProcessControlService) PreviewProcessAction(pid int32, action string, priority int) (map[string]interface{}, error) {
	// Validate action
	var processAction processcontrol.ProcessAction
	switch action {
	case "graceful_close":
		processAction = processcontrol.ActionGracefulClose
	case "terminate":
		processAction = processcontrol.ActionTerminate
	case "set_priority":
		processAction = processcontrol.ActionSetPriority
	default:
		return nil, fmt.Errorf("invalid action type")
	}

	// Validate priority class only for set_priority action
	priorityClass := processcontrol.PriorityClass(priority)
	if processAction == processcontrol.ActionSetPriority {
		if priorityClass != processcontrol.PriorityIdle &&
			priorityClass != processcontrol.PriorityBelowNormal &&
			priorityClass != processcontrol.PriorityNormal &&
			priorityClass != processcontrol.PriorityAboveNormal &&
			priorityClass != processcontrol.PriorityHigh {
			return nil, fmt.Errorf("unsupported priority class (Real-time not allowed)")
		}
	}

	// Create preview
	preview, err := s.store.CreatePreview(pid, processAction, priorityClass)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"operationId": preview.OperationID,
		"pid":         preview.PID,
		"action":      string(preview.Action),
		"priority":    preview.Priority.String(),
		"expiresAt":   preview.ExpiresAt.Format(time.RFC3339),
	}, nil
}

// ConfirmProcessAction executes a previously previewed action
// Uses atomic consume to prevent duplicate execution
func (s *ProcessControlService) ConfirmProcessAction(operationID string) (map[string]interface{}, error) {
	// Atomically consume the preview to prevent duplicate execution
	preview, err := s.store.TryConsumePreview(operationID)
	if err != nil {
		return nil, err
	}

	var result processcontrol.OperationResult

	switch preview.Action {
	case processcontrol.ActionGracefulClose:
		result = processcontrol.ExecuteGracefulClose(operationID, s.store)
	case processcontrol.ActionTerminate:
		result = processcontrol.ExecuteTerminate(operationID, s.store)
	case processcontrol.ActionSetPriority:
		result = processcontrol.ExecuteSetPriority(operationID, s.store)
	default:
		return nil, fmt.Errorf("invalid action type")
	}

	return map[string]interface{}{
		"operationId": result.OperationID,
		"success":     result.Success,
		"outcome":     result.Outcome,
		"details":     result.Details,
		"verifiedAt":  result.VerifiedAt.Format(time.RFC3339),
	}, nil
}

