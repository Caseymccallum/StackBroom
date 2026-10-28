package cleanup

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/storage"
)

// ValidateEligibility applies the same rule constraints at preview and execution.
// The filesystem identity must be checked separately, immediately before use.
func (r Rule) ValidateEligibility(file storage.FileRecord, count int, now time.Time) error {
	if r.MaxCandidates <= 0 || count <= 0 || count > r.MaxCandidates {
		return fmt.Errorf("candidate limit")
	}
	relative, err := filepath.Rel(file.Root, file.Path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("scope")
	}
	if strings.Count(relative, string(filepath.Separator)) > r.MaxDepth {
		return fmt.Errorf("depth")
	}
	if file.Name != filepath.Base(file.Path) || !r.AllowsExtension(filepath.Base(file.Path)) {
		return fmt.Errorf("extension")
	}
	if now.Sub(file.ModTime) < time.Duration(r.MinAgeDays)*24*time.Hour {
		return fmt.Errorf("age")
	}
	return nil
}
