package execution

import (
	"strings"
	"testing"
	"time"
)

func TestReceiptLookupDoesNotConsumeOrReserve(t *testing.T) {
	vault := NewMemoryReceiptVault(2)
	now := time.Now()
	receipt := RecycleReceipt{
		TargetID: "target-1", OriginalFileID: "volume:file", Provider: "fake",
		ObservedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute),
	}
	token, err := vault.Store(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || strings.Contains(token, "target") || strings.Contains(token, "volume") {
		t.Fatalf("receipt token is not opaque: %q", token)
	}
	for attempt := 0; attempt < 2; attempt++ {
		found, err := vault.Lookup(token, now)
		if err != nil || found.TargetID != receipt.TargetID {
			t.Fatalf("lookup %d = %+v, %v", attempt, found, err)
		}
	}
	if vault.Len() != 1 {
		t.Fatalf("lookup consumed receipt, len=%d", vault.Len())
	}
	reservation, err := vault.Reserve(token, now, time.Minute)
	if err != nil {
		t.Fatalf("reserve after lookup: %v", err)
	}
	if reservation.ID == "" || reservation.ID == token {
		t.Fatal("reservation capability is not opaque")
	}
}

func TestReceiptReservationIsSingleOwnerAndReusableAfterRelease(t *testing.T) {
	vault := NewMemoryReceiptVault(1)
	now := time.Now()
	token, err := vault.Store(RecycleReceipt{
		TargetID: "target-1", OriginalFileID: "volume:file", Provider: "fake",
		ObservedAt: now.Add(-time.Minute), ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := vault.Reserve(token, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Reserve(token, now.Add(time.Second), time.Minute); err == nil {
		t.Fatal("concurrent reservation was granted")
	}
	if err := vault.Release(first); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Lookup(token, now); err != nil {
		t.Fatalf("release consumed receipt: %v", err)
	}
	second, err := vault.Reserve(token, now.Add(2*time.Second), time.Minute)
	if err != nil {
		t.Fatalf("reserve after release: %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("reservation capability was reused")
	}
}

func TestReceiptCommitIsFinalAndExpiryFailsClosed(t *testing.T) {
	vault := NewMemoryReceiptVault(2)
	now := time.Now()
	receipt := RecycleReceipt{
		TargetID: "target-1", OriginalFileID: "volume:file", Provider: "fake",
		ObservedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute),
	}
	token, err := vault.Store(receipt)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := vault.Reserve(token, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.Commit(reservation, now); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Lookup(token, now); err == nil {
		t.Fatal("committed receipt remained available")
	}
	if _, err := vault.Reserve(token, now, time.Minute); err == nil {
		t.Fatal("committed receipt was reserved again")
	}

	expiring, err := vault.Store(receipt)
	if err != nil {
		t.Fatal(err)
	}
	expiredReservation, err := vault.Reserve(expiring, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.Commit(expiredReservation, now.Add(2*time.Minute)); err == nil {
		t.Fatal("expired receipt committed as success")
	}
	if _, err := vault.Lookup(expiring, now); err == nil {
		t.Fatal("expired committed receipt remained available")
	}
}

func TestReceiptExpiredReservationCanBeReplacedButExpiredReceiptCannot(t *testing.T) {
	vault := NewMemoryReceiptVault(1)
	now := time.Now()
	token, err := vault.Store(RecycleReceipt{
		TargetID: "target-1", OriginalFileID: "volume:file", Provider: "fake",
		ObservedAt: now.Add(-time.Minute), ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Reserve(token, now, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Reserve(token, now.Add(2*time.Minute), time.Minute); err != nil {
		t.Fatalf("expired lease was not recoverable: %v", err)
	}
	if _, err := vault.Lookup(token, now.Add(11*time.Minute)); err == nil {
		t.Fatal("expired receipt was accepted")
	}
	if _, err := vault.Reserve(token, now.Add(11*time.Minute), time.Minute); err == nil {
		t.Fatal("expired receipt was reserved")
	}
	vault.Clear()
	if vault.Len() != 0 {
		t.Errorf("vault not cleared")
	}
}
