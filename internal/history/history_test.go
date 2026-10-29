package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
)

func TestHistoryIsBoundedAndPrivacyFiltered(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history", "events.jsonl")
	store := NewStore(path, 30, 2)
	private := `C:\Users\Private\secret.tmp`
	for index := 0; index < 3; index++ {
		store.Record(execution.Record{
			At: time.Now(), Type: "execution", RuleID: "user-temp-old", RuleVersion: "1.0.0",
			State: execution.StateCompleted, TargetCount: 1, LogicalBytes: 10, AllocatedBytes: 10,
			DeletionMode: "recycle", ReasonCode: "recycled",
		})
	}
	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("retention bound = %d", len(entries))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), private) || strings.Contains(string(raw), "secret.tmp") {
		t.Error("history contains a private path")
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if entries, _ := store.List(); len(entries) != 0 {
		t.Error("history was not cleared")
	}
}

func TestPermanentHistoryNeverClaimsRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history", "permanent.jsonl")
	store := NewStore(path, 30, 10)
	store.Record(execution.Record{
		At: time.Now(), Type: "execution", RuleID: "crash-dumps-old", RuleVersion: "1.0.0",
		State: execution.StateCompleted, TargetCount: 1, LogicalBytes: 100, AllocatedBytes: 100,
		DeletionMode: "permanent", ReasonCode: "deleted", RecoveryAvailable: true,
	})
	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].DeletionMode != "permanent" || entries[0].RecoveryAvailable {
		t.Fatalf("permanent history = %+v", entries)
	}
}
