package processcontrol

import (
	"testing"
	"time"
)

func TestOperationStore(t *testing.T) {
	store := NewOperationStore()

	// Test creating a preview
	preview, err := store.CreatePreview(1234, ActionGracefulClose, PriorityNormal)
	if err != nil {
		t.Fatalf("CreatePreview failed: %v", err)
	}

	if preview.OperationID == "" {
		t.Error("expected non-empty operation ID")
	}

	if preview.PID != 1234 {
		t.Errorf("expected PID 1234, got %d", preview.PID)
	}

	if preview.Action != ActionGracefulClose {
		t.Errorf("expected action %s, got %s", ActionGracefulClose, preview.Action)
	}

	// Test retrieving preview
	retrieved, err := store.GetPreview(preview.OperationID)
	if err != nil {
		t.Fatalf("GetPreview failed: %v", err)
	}

	if retrieved.OperationID != preview.OperationID {
		t.Error("operation ID mismatch")
	}

	// Test consuming preview (single-use)
	consumed, err := store.ConsumePreview(preview.OperationID)
	if err != nil {
		t.Fatalf("ConsumePreview failed: %v", err)
	}

	if consumed.OperationID != preview.OperationID {
		t.Error("operation ID mismatch")
	}

	// Test that consumed preview is gone
	_, err = store.GetPreview(preview.OperationID)
	if err == nil {
		t.Error("expected error for consumed preview")
	}
}

func TestOperationExpiry(t *testing.T) {
	store := NewOperationStore()

	// Create preview with very short expiry for testing
	preview, err := store.CreatePreview(1234, ActionTerminate, PriorityNormal)
	if err != nil {
		t.Fatalf("CreatePreview failed: %v", err)
	}

	// Manually expire it
	store.mu.Lock()
	preview.ExpiresAt = time.Now().Add(-1 * time.Second)
	store.mu.Unlock()

	// Should fail to retrieve
	_, err = store.GetPreview(preview.OperationID)
	if err == nil {
		t.Error("expected error for expired preview")
	}
}

func TestPriorityClassString(t *testing.T) {
	tests := []struct {
		priority PriorityClass
		expected string
	}{
		{PriorityIdle, "Idle"},
		{PriorityBelowNormal, "Below Normal"},
		{PriorityNormal, "Normal"},
		{PriorityAboveNormal, "Above Normal"},
		{PriorityHigh, "High"},
		{PriorityClass(999), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.priority.String(); got != tt.expected {
			t.Errorf("PriorityClass(%d).String() = %s, want %s", tt.priority, got, tt.expected)
		}
	}
}
