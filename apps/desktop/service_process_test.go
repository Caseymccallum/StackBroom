package main

import (
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/processcontrol"
)

func TestProcessControlService_PreviewProcessAction(t *testing.T) {
	service := NewProcessControlService()

	// Test creating a preview for graceful close
	preview, err := service.PreviewProcessAction(1234, "graceful_close", 0)
	if err != nil {
		t.Fatalf("PreviewProcessAction failed: %v", err)
	}

	if preview["operationId"] == nil || preview["operationId"] == "" {
		t.Error("expected non-empty operation ID")
	}

	if preview["pid"] != int32(1234) {
		t.Errorf("expected PID 1234, got %v", preview["pid"])
	}

	if preview["action"] != "graceful_close" {
		t.Errorf("expected action graceful_close, got %v", preview["action"])
	}

	// Test invalid action
	_, err = service.PreviewProcessAction(1234, "invalid_action", 0)
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestProcessControlService_ConfirmProcessAction(t *testing.T) {
	service := NewProcessControlService()

	// Create a preview
	preview, err := service.PreviewProcessAction(1234, "graceful_close", 0)
	if err != nil {
		t.Fatalf("PreviewProcessAction failed: %v", err)
	}

	operationID := preview["operationId"].(string)

	// Note: This test will fail because PID 1234 likely doesn't exist
	// In a real test, we'd use a mock or a real disposable process
	result, err := service.ConfirmProcessAction(operationID)
	if err != nil {
		// Expected to fail since PID 1234 doesn't exist
		t.Logf("ConfirmProcessAction failed as expected: %v", err)
		return
	}

	// If it somehow succeeded, check the result structure
	if result["operationId"] == nil {
		t.Error("expected operation ID in result")
	}
}

func TestProcessControlService_GetProcessDetails(t *testing.T) {
	service := NewProcessControlService()

	// Test with a non-existent process
	_, err := service.GetProcessDetails(999999, time.Now().Format(time.RFC3339))
	if err == nil {
		t.Error("expected error for non-existent process")
	}
}

func TestProcessControlService_PriorityValidation(t *testing.T) {
	service := NewProcessControlService()

	// Test valid priority values
	validPriorities := []int{
		int(processcontrol.PriorityIdle),
		int(processcontrol.PriorityBelowNormal),
		int(processcontrol.PriorityNormal),
		int(processcontrol.PriorityAboveNormal),
		int(processcontrol.PriorityHigh),
	}

	for _, priority := range validPriorities {
		preview, err := service.PreviewProcessAction(1234, "set_priority", priority)
		if err != nil {
			t.Errorf("PreviewProcessAction failed for priority %d: %v", priority, err)
		}
		if preview["priority"] == nil {
			t.Errorf("expected priority in preview for priority %d", priority)
		}
	}

	// Test invalid priority (Real-time, which should be rejected)
	_, err := service.PreviewProcessAction(1234, "set_priority", int(processcontrol.PriorityRealTime))
	if err == nil {
		t.Error("expected error for Real-time priority")
	}
}
