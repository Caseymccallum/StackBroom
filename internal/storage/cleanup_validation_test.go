package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestValidateCleanupTargetFailsClosed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "old.tmp"), 300, 'x')
	result := scanFixture(t, root, nil)
	if len(result.Files) != 1 {
		t.Fatalf("fixture files = %d", len(result.Files))
	}
	record := *result.Files[0]
	if err := ValidateCleanupTarget(&record); err != nil {
		t.Fatalf("valid target rejected: %v", err)
	}

	missingID := record
	missingID.FileID = ""
	if code := ValidateTargetErrorCode(ValidateCleanupTarget(&missingID)); code != "identity_missing" {
		t.Errorf("missing identity code = %s", code)
	}

	hardLink := record
	hardLink.LinkCount = 2
	if code := ValidateTargetErrorCode(ValidateCleanupTarget(&hardLink)); code != "hard_link" {
		t.Errorf("hard link code = %s", code)
	}

	changed := record
	changed.LogicalBytes++
	if code := ValidateTargetErrorCode(ValidateCleanupTarget(&changed)); code != "metadata_changed" {
		t.Errorf("changed metadata code = %s", code)
	}
}

func TestValidateCleanupTargetRejectsHiddenFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "hidden.tmp")
	writeFile(t, path, 300, 'x')
	setHiddenAttribute(t, path)
	request := DefaultRequest(root)
	request.IncludeHidden = true
	result, err := NewScanner().Scan(context.Background(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("hidden fixture files = %d", len(result.Files))
	}
	if code := ValidateTargetErrorCode(ValidateCleanupTarget(result.Files[0])); code != "hidden" {
		t.Errorf("hidden target code = %s", code)
	}
}
