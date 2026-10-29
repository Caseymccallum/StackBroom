//go:build windows && cleanup_execution && m3c2_fixture

package permanent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Caseymccallum/StackBroom/internal/storage"
)

// NewFixtureExecutor is available only in the disposable fixture test build.
// Production callers cannot supply a scope to NewNativeExecutor. The extra
// runtime gate prevents a tagged binary from using an arbitrary root merely by
// importing the fixture build.
func NewFixtureExecutor(root string) Executor {
	if os.Getenv("STACKBROOM_M3C2_HARNESS_AUTHORIZED") != "1" {
		return windowsExecutor{initErr: fmt.Errorf("fixture executor is not authorised")}
	}
	validated, err := storage.ValidateRoot(root)
	if err != nil {
		return windowsExecutor{initErr: fmt.Errorf("fixture root is not valid: %w", err)}
	}
	if !fixtureRootLooksDisposable(validated) {
		return windowsExecutor{initErr: fmt.Errorf("fixture root is not a disposable M3C2 directory")}
	}
	return windowsExecutor{trustedRoot: validated}
}

func fixtureRootLooksDisposable(root string) bool {
	temp, err := filepath.Abs(os.TempDir())
	if err != nil {
		return false
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(temp, root)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	return strings.HasPrefix(strings.ToLower(filepath.Base(root)), "stackbroom-m3c2-")
}
