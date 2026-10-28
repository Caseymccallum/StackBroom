//go:build !windows || !cleanup_execution

package execution

import "context"

func NewNativeExecutor() Executor { return NewDisabledExecutor() }

type unavailableVerifier struct{}

func (unavailableVerifier) Available() error { return ErrDisabled }

func (unavailableVerifier) Verify(context.Context, Target) (RecoveryEvidence, error) {
	return RecoveryEvidence{}, ErrDisabled
}

func NewRecoveryVerifier() RecoveryVerifier { return unavailableVerifier{} }
