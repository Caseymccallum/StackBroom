package storage

import (
	"os"
	"path/filepath"
	"strings"
)

// TargetValidationError identifies a fail-closed cleanup eligibility failure
// without exposing an absolute path.
type TargetValidationError struct {
	Code    string
	Message string
}

func (e *TargetValidationError) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Message
}

func (f FileRecord) RootEqual(root string) bool {
	if f.Root == "" || root == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(f.Root), filepath.Clean(root))
}

func validationFailure(code, message string) error {
	return &TargetValidationError{Code: code, Message: message}
}

// ValidateCleanupTarget is the stricter, fail-closed check used by execution
// previews. Unlike duplicate-file validation, a missing identity is refused.
func ValidateCleanupTarget(file *FileRecord) error {
	if file == nil {
		return validationFailure("missing_record", "target record is missing")
	}
	if file.Root == "" || file.Path == "" || !filepath.IsAbs(file.Root) || !filepath.IsAbs(file.Path) {
		return validationFailure("missing_scope", "target scope is missing")
	}
	if file.FileID == "" {
		return validationFailure("identity_missing", "target identity is unavailable")
	}
	if file.Name != filepath.Base(file.Path) {
		return validationFailure("name_mismatch", "target name does not match its path")
	}
	if file.LinkCount != 1 {
		return validationFailure("hard_link", "target has multiple hard links")
	}
	if !pathWithinRoot(file.Root, file.Path) || !resolvedPathWithinRoot(file.Root, file.Path) {
		return validationFailure("scope", "target is outside the approved root or has a reparse ancestor")
	}
	info, err := os.Lstat(platformPath(file.Path))
	if err != nil {
		return validationFailure("inaccessible", "target is no longer accessible")
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
		return validationFailure("not_regular", "target is no longer a regular file")
	}
	if uint64(info.Size()) != file.LogicalBytes || !info.ModTime().Equal(file.ModTime) {
		return validationFailure("metadata_changed", "target metadata changed")
	}
	meta := inspectFile(file.Path, info)
	if meta.Reparse {
		return validationFailure("reparse", "target is a reparse point")
	}
	if meta.Placeholder {
		return validationFailure("placeholder", "target is a cloud placeholder")
	}
	if meta.Hidden {
		return validationFailure("hidden", "target is hidden")
	}
	if meta.System {
		return validationFailure("system", "target is a system file")
	}
	if meta.Sparse || file.Sparse {
		return validationFailure("sparse", "target is sparse and its reclaimable size is uncertain")
	}
	if meta.LinkCount != 1 {
		return validationFailure("hard_link", "target link count changed")
	}
	if meta.FileID == "" || meta.FileID != file.FileID {
		return validationFailure("identity_changed", "target identity changed")
	}
	return nil
}

func ValidateTargetErrorCode(err error) string {
	if typed, ok := err.(*TargetValidationError); ok {
		return typed.Code
	}
	return "validation_failed"
}
