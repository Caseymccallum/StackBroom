//go:build !windows || !cleanup_execution

package permanent

import (
	"context"

	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
)

type disabledExecutor struct{}

func (disabledExecutor) Available() error { return ErrDisabled }

func (disabledExecutor) Execute(context.Context, []execution.Target, func(Progress)) (Result, error) {
	return Result{State: StateFailed, ReasonCode: "execution_disabled", Message: ErrDisabled.Error()}, ErrDisabled
}

// NewNativeExecutor is the ordinary-build executor. It has no filesystem or
// native disposition path.
func NewNativeExecutor() Executor { return disabledExecutor{} }
