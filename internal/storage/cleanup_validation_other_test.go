//go:build !windows

package storage

import "testing"

func setHiddenAttribute(t *testing.T, path string) {
	t.Skip("hidden attributes are Windows-only")
}
