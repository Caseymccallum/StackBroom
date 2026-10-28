package processcontrol

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const (
	OperationExpiry = 60 * time.Second
	MaxOperations   = 100
)

// OperationStore manages pending operation previews
type OperationStore struct {
	mu          sync.RWMutex
	operations  map[string]*OperationPreview
}

// NewOperationStore creates a new operation store
func NewOperationStore() *OperationStore {
	return &OperationStore{
		operations: make(map[string]*OperationPreview),
	}
}

// generateOperationID creates a unique operation identifier
func generateOperationID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate operation ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// CreatePreview creates a new operation preview and returns it
func (s *OperationStore) CreatePreview(pid int32, action ProcessAction, priority PriorityClass) (*OperationPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.operations) >= MaxOperations {
		s.cleanupExpired()
	}

	if len(s.operations) >= MaxOperations {
		return nil, fmt.Errorf("too many pending operations")
	}

	id, err := generateOperationID()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	preview := &OperationPreview{
		OperationID: id,
		PID:         pid,
		Action:      action,
		Priority:    priority,
		CreatedAt:   now,
		ExpiresAt:   now.Add(OperationExpiry),
	}

	s.operations[id] = preview
	return preview, nil
}

// GetPreview retrieves a preview by ID, verifying it hasn't expired
func (s *OperationStore) GetPreview(operationID string) (*OperationPreview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	preview, exists := s.operations[operationID]
	if !exists {
		return nil, fmt.Errorf("invalid operation ID")
	}

	if time.Now().After(preview.ExpiresAt) {
		return nil, fmt.Errorf("operation expired")
	}

	return preview, nil
}

// ConsumePreview atomically retrieves and removes a preview (single-use)
// This prevents duplicate execution through race conditions
func (s *OperationStore) ConsumePreview(operationID string) (*OperationPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	preview, exists := s.operations[operationID]
	if !exists {
		return nil, fmt.Errorf("invalid operation ID")
	}

	if time.Now().After(preview.ExpiresAt) {
		delete(s.operations, operationID)
		return nil, fmt.Errorf("operation expired")
	}

	// Atomically remove to prevent duplicate execution
	delete(s.operations, operationID)
	return preview, nil
}

// TryConsumePreview attempts to consume a preview with atomic check-and-delete
// Returns the preview if successful, or an error if already consumed/expired
func (s *OperationStore) TryConsumePreview(operationID string) (*OperationPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	preview, exists := s.operations[operationID]
	if !exists {
		return nil, fmt.Errorf("operation already consumed or invalid ID")
	}

	if time.Now().After(preview.ExpiresAt) {
		delete(s.operations, operationID)
		return nil, fmt.Errorf("operation expired")
	}

	// Atomic remove - if this fails, the operation was already consumed
	delete(s.operations, operationID)
	return preview, nil
}

// cleanupExpired removes expired operations (must be called with write lock)
func (s *OperationStore) cleanupExpired() {
	now := time.Now()
	for id, preview := range s.operations {
		if now.After(preview.ExpiresAt) {
			delete(s.operations, id)
		}
	}
}

// CleanupExpired removes all expired operations
func (s *OperationStore) CleanupExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	now := time.Now()
	for id, preview := range s.operations {
		if now.After(preview.ExpiresAt) {
			delete(s.operations, id)
			count++
		}
	}
	return count
}
