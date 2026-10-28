//go:build !cleanup_execution

package execution

import "testing"

func TestOrdinaryBuildHasNoExecutionCapability(t *testing.T) {
	if BuildEnabled() {
		t.Fatal("ordinary build unexpectedly enabled cleanup execution")
	}
}
