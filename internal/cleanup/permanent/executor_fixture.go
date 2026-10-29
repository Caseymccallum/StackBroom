//go:build windows && cleanup_execution && m3c2_fixture

package permanent

// NewFixtureExecutor is available only in the disposable fixture test build.
// Production callers cannot supply a scope to NewNativeExecutor.
func NewFixtureExecutor(root string) Executor {
	return windowsExecutor{trustedRoot: root}
}
