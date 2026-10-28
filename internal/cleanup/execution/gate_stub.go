//go:build !cleanup_execution

package execution

// executionBuildEnabled is false in ordinary builds. Even an environment
// override cannot expose a mutation path because the native executor is not
// compiled into the binary.
const executionBuildEnabled = false
