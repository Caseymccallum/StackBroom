package storage

import (
	"path/filepath"
	"strings"
)

func resolvedPathWithinRoot(root, path string) bool {
	if root == "" {
		return true
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	return pathWithinRoot(root, resolved) && strings.EqualFold(filepath.Clean(resolved), filepath.Clean(path))
}

func pathWithinRoot(root, path string) bool {
	if root == "" {
		return true
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

type fileMeta struct {
	Hidden         bool
	System         bool
	Reparse        bool
	Placeholder    bool
	Sparse         bool
	LinkCount      uint32
	FileID         string
	AllocatedBytes uint64
	AllocatedKnown bool
}
