package execution

import (
	"strings"
	"testing"
	"time"
)

func TestReceiptVaultIsOpaqueSingleUseAndExpiring(t *testing.T) {
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
	taken, err := vault.Take(token, now)
	if err != nil || taken.TargetID != receipt.TargetID {
		t.Fatalf("take = %+v, %v", taken, err)
	}
	if _, err := vault.Take(token, now); err == nil {
		t.Error("receipt was reusable")
	}

	second, err := vault.Store(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Take(second, now.Add(2*time.Minute)); err == nil {
		t.Error("expired receipt was accepted")
	}
	vault.Clear()
	if vault.Len() != 0 {
		t.Errorf("vault not cleared")
	}
}
