package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
)

func TestEveryStringIsFilteredOnWriteAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := NewStore(path, 30, 500)
	secret := "C:\\Users\\Private\\secret.dmp"
	store.Record(execution.Record{At: time.Now(), Type: secret, RuleID: secret, RuleVersion: secret, State: execution.State(secret), DeletionMode: secret, ReasonCode: secret})
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret") {
		t.Fatal("private caller string persisted")
	}
	legacy := Record{At: time.Now(), Type: secret, RuleID: secret, RuleVersion: secret, State: secret, DeletionMode: secret, ReasonCode: secret}
	raw, _ = json.Marshal(legacy)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := store.List()
	if err != nil || len(entries) != 1 {
		t.Fatal(err)
	}
	filtered, _ := json.Marshal(entries)
	if strings.Contains(string(filtered), "secret") {
		t.Fatal("legacy journal leaked")
	}
}
func TestRetentionAppliesWithoutNewWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := NewStore(path, 30, 500)
	now := time.Now()
	store.now = func() time.Time { return now }
	store.Record(execution.Record{At: now, Type: "execution"})
	store.now = func() time.Time { return now.Add(31 * 24 * time.Hour) }
	entries, err := store.List()
	if err != nil || len(entries) != 0 {
		t.Fatalf("%+v %v", entries, err)
	}
}
