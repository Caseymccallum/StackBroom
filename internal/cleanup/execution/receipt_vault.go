package execution

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ReceiptVault is separate from public action history. Receipts contain an
// internal target binding and opaque provider token, never a path. The current
// implementation is deliberately in-memory; durable storage needs a separate
// privacy/integrity review.
type ReceiptVault interface {
	Store(RecycleReceipt) (string, error)
	Take(token string, now time.Time) (RecycleReceipt, error)
	Len() int
	Clear()
}

type memoryReceiptVault struct {
	mu      sync.Mutex
	entries map[string]RecycleReceipt
	max     int
}

func NewMemoryReceiptVault(max int) ReceiptVault {
	if max <= 0 {
		max = 128
	}
	return &memoryReceiptVault{entries: make(map[string]RecycleReceipt), max: max}
}

func (v *memoryReceiptVault) Store(receipt RecycleReceipt) (string, error) {
	if receipt.TargetID == "" || receipt.OriginalFileID == "" || receipt.Provider == "" ||
		receipt.ObservedAt.IsZero() || !receipt.ExpiresAt.After(receipt.ObservedAt) {
		return "", fmt.Errorf("recovery receipt is incomplete")
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("recovery token could not be generated")
	}
	token := hex.EncodeToString(tokenBytes)
	receipt.OpaqueToken = token
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.entries) >= v.max {
		return "", fmt.Errorf("recovery receipt store is full")
	}
	v.entries[token] = receipt
	return token, nil
}

func (v *memoryReceiptVault) Take(token string, now time.Time) (RecycleReceipt, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	receipt, ok := v.entries[token]
	if !ok {
		return RecycleReceipt{}, fmt.Errorf("recovery receipt not found")
	}
	delete(v.entries, token)
	if !now.Before(receipt.ExpiresAt) {
		return RecycleReceipt{}, fmt.Errorf("recovery receipt expired")
	}
	return receipt, nil
}

func (v *memoryReceiptVault) Len() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.entries)
}

func (v *memoryReceiptVault) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()
	for token := range v.entries {
		delete(v.entries, token)
	}
}
