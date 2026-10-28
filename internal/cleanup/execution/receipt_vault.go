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
//
// Lookup is side-effect free. A caller must Reserve a receipt before beginning
// restoration, Release it if planning or execution is abandoned, and Commit it
// only after restoration and post-restore verification succeed. This prevents a
// read used for display or inspection from consuming the recovery authorisation.
type ReceiptVault interface {
	Store(RecycleReceipt) (string, error)
	Lookup(token string, now time.Time) (RecycleReceipt, error)
	Reserve(token string, now time.Time, lease time.Duration) (ReceiptReservation, error)
	Release(reservation ReceiptReservation) error
	Commit(reservation ReceiptReservation, now time.Time) error
	Len() int
	Clear()
}

// ReceiptReservation is the capability returned by Reserve. It contains no
// path or provider secret. The random reservation ID prevents one concurrent
// restore from releasing or committing another's reservation.
type ReceiptReservation struct {
	Token     string
	ID        string
	ExpiresAt time.Time
}

type receiptEntry struct {
	receipt      RecycleReceipt
	reserved     bool
	reservation  string
	reservedTill time.Time
}

type memoryReceiptVault struct {
	mu      sync.Mutex
	entries map[string]receiptEntry
	max     int
}

func NewMemoryReceiptVault(max int) ReceiptVault {
	if max <= 0 {
		max = 128
	}
	return &memoryReceiptVault{entries: make(map[string]receiptEntry), max: max}
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
	v.entries[token] = receiptEntry{receipt: receipt}
	return token, nil
}

// Lookup never consumes or reserves a receipt. Expired receipts are refused but
// are not removed here; Clear and Commit own storage cleanup.
func (v *memoryReceiptVault) Lookup(token string, now time.Time) (RecycleReceipt, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, ok := v.entries[token]
	if !ok {
		return RecycleReceipt{}, fmt.Errorf("recovery receipt not found")
	}
	if !now.Before(entry.receipt.ExpiresAt) {
		return RecycleReceipt{}, fmt.Errorf("recovery receipt expired")
	}
	return entry.receipt, nil
}

// Reserve atomically grants a single restore lease. An expired reservation can
// be replaced, but a live reservation cannot; this is the replay barrier.
func (v *memoryReceiptVault) Reserve(token string, now time.Time, lease time.Duration) (ReceiptReservation, error) {
	if lease <= 0 {
		lease = 30 * time.Second
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, ok := v.entries[token]
	if !ok {
		return ReceiptReservation{}, fmt.Errorf("recovery receipt not found")
	}
	if !now.Before(entry.receipt.ExpiresAt) {
		return ReceiptReservation{}, fmt.Errorf("recovery receipt expired")
	}
	if entry.reserved && now.Before(entry.reservedTill) {
		return ReceiptReservation{}, fmt.Errorf("recovery receipt is reserved")
	}
	expires := now.Add(lease)
	if expires.After(entry.receipt.ExpiresAt) {
		expires = entry.receipt.ExpiresAt
	}
	reservationBytes := make([]byte, 32)
	if _, err := rand.Read(reservationBytes); err != nil {
		return ReceiptReservation{}, fmt.Errorf("recovery reservation could not be generated")
	}
	reservation := ReceiptReservation{
		Token:     token,
		ID:        hex.EncodeToString(reservationBytes),
		ExpiresAt: expires,
	}
	entry.reserved = true
	entry.reservation = reservation.ID
	entry.reservedTill = expires
	v.entries[token] = entry
	return reservation, nil
}

// Release abandons a reservation after a failed preflight, cancellation or
// application restart planning failure. It never consumes the receipt.
func (v *memoryReceiptVault) Release(reservation ReceiptReservation) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, ok := v.entries[reservation.Token]
	if !ok {
		return fmt.Errorf("recovery receipt not found")
	}
	if !entry.reserved || entry.reservation != reservation.ID {
		return fmt.Errorf("recovery reservation is not current")
	}
	entry.reserved = false
	entry.reservation = ""
	entry.reservedTill = time.Time{}
	v.entries[reservation.Token] = entry
	return nil
}

// Commit is the final consumption step after restoration and post-restore
// verification. It also removes an expired receipt, but returns an error so a
// caller cannot report success for an expired recovery authorisation.
func (v *memoryReceiptVault) Commit(reservation ReceiptReservation, now time.Time) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, ok := v.entries[reservation.Token]
	if !ok {
		return fmt.Errorf("recovery receipt not found")
	}
	if !entry.reserved || entry.reservation != reservation.ID {
		return fmt.Errorf("recovery reservation is not current")
	}
	delete(v.entries, reservation.Token)
	if !now.Before(entry.receipt.ExpiresAt) {
		return fmt.Errorf("recovery receipt expired")
	}
	return nil
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
