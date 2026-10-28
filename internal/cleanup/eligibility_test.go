package cleanup

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/storage"
)

func TestRuleEligibilityUsesPathDepthCountAndAge(t *testing.T) {
	rule, _ := RuleByID("crash-dumps-old")
	now := time.Now()
	root := t.TempDir()
	file := storage.FileRecord{Root: root, Path: filepath.Join(root, "old.dmp"), Name: "old.dmp", ModTime: now.Add(-30 * 24 * time.Hour)}
	if err := rule.ValidateEligibility(file, 1, now); err != nil {
		t.Fatal(err)
	}
	wrong := file
	wrong.Path = filepath.Join(root, "private.txt")
	if rule.ValidateEligibility(wrong, 1, now) == nil {
		t.Fatal("forged display name")
	}
	wrong = file
	wrong.Path = filepath.Join(root, "a", "b", "old.dmp")
	if rule.ValidateEligibility(wrong, 1, now) == nil {
		t.Fatal("depth bypass")
	}
	if rule.ValidateEligibility(file, rule.MaxCandidates+1, now) == nil {
		t.Fatal("count bypass")
	}
	wrong = file
	wrong.ModTime = now.Add(time.Hour)
	if rule.ValidateEligibility(wrong, 1, now) == nil {
		t.Fatal("future timestamp")
	}
}
