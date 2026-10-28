//go:build windows

package storage

import "strings"

// platformPath adds the extended-length prefix for Win32 calls. The traversal
// keeps the normal user-facing path; only file APIs see this form.
func platformPath(path string) string {
	if strings.HasPrefix(path, `\\?\`) {
		return path
	}
	if strings.HasPrefix(path, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(path, `\\`)
	}
	return `\\?\` + path
}
