package main

import (
	"fmt"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/processcontrol"
)

// ProcessControlService handles process management operations
type ProcessControlService struct {
	store    *processcontrol.OperationStore
	tokens   *processcontrol.TokenIssuer
	actions  processcontrol.NativeActions
	verifyFn func(int32, time.Time) (processcontrol.ProcessIdentity, error)
}

// NewProcessControlServiceWithTokens verifies scan-issued target tokens with
// the application-wide key.
func NewProcessControlServiceWithTokens(tokens *processcontrol.TokenIssuer) *ProcessControlService {
	return &ProcessControlService{
		store:    processcontrol.NewOperationStore(),
		tokens:   tokens,
		actions:  processcontrol.DefaultNativeActions(),
		verifyFn: processcontrol.VerifyProcessIdentity,
	}
}

// verifyTargetToken checks the opaque backend token and re-verifies the live
// process identity. The frontend never supplies an authoritative timestamp.
func (s *ProcessControlService) verifyTargetToken(pid int32, targetToken string) (processcontrol.ProcessIdentity, error) {
	if s.tokens == nil {
		return processcontrol.ProcessIdentity{}, fmt.Errorf("target verification unavailable")
	}
	created, err := s.tokens.Verify(targetToken, pid)
	if err != nil {
		return processcontrol.ProcessIdentity{}, err
	}
	return s.verifyFn(pid, created)
}

// GetProcessDetails returns safe, non-sensitive details for a verified target.
func (s *ProcessControlService) GetProcessDetails(pid int32, targetToken string) (map[string]interface{}, error) {
	identity, err := s.verifyTargetToken(pid, targetToken)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"pid":         identity.PID,
		"name":        identity.Name,
		"isProtected": identity.IsProtected,
		"isCritical":  identity.IsCritical,
	}, nil
}

// PreviewProcessAction creates a preview for a process action after verifying identity
func (s *ProcessControlService) PreviewProcessAction(pid int32, action string, priority int, targetToken string) (map[string]interface{}, error) {
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

	identity, err := s.verifyTargetToken(pid, targetToken)
	if err != nil {
		return nil, fmt.Errorf("identity verification failed: %v", err)
	}

	preview, err := s.store.CreatePreview(pid, processAction, priorityClass, identity)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"operationId": preview.OperationID,
		"pid":         preview.PID,
		"action":      string(preview.Action),
		"priority":    preview.Priority.String(),
		"targetName":  preview.Target.Name,
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
		result = processcontrol.ExecuteGracefulClose(*preview, s.actions)
	case processcontrol.ActionTerminate:
		result = processcontrol.ExecuteTerminate(*preview, s.actions)
	case processcontrol.ActionSetPriority:
		result = processcontrol.ExecuteSetPriority(*preview, s.actions)
	default:
		return nil, fmt.Errorf("invalid action type")
	}

	return map[string]interface{}{
		"operationId":   result.OperationID,
		"success":       result.Success,
		"outcome":       result.Outcome,
		"details":       result.Details,
		"previousValue": result.PreviousValue,
		"newValue":      result.NewValue,
		"verifiedAt":    result.VerifiedAt.Format(time.RFC3339),
	}, nil
}
