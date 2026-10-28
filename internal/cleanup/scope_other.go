//go:build !windows

package cleanup

import "fmt"

func platformTrustedRoot(string) (string, error) {
	return "", fmt.Errorf("cleanup scopes are Windows-only")
}
