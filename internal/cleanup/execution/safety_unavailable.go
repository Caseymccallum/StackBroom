package execution

import (
	"context"
	"fmt"
	"time"
)

type unavailablePreflight struct{}

func (unavailablePreflight) Available() error {
	return fmt.Errorf("no-fallback preflight is unavailable")
}
func (unavailablePreflight) Check(context.Context, Target) (PreflightEvidence, error) {
	return PreflightEvidence{}, fmt.Errorf("no-fallback preflight is unavailable")
}

type unavailableFinalGuard struct{}

func (unavailableFinalGuard) Available() error {
	return fmt.Errorf("final-file guard is unavailable")
}
func (unavailableFinalGuard) Acquire(context.Context, Target) (Guard, error) {
	return nil, fmt.Errorf("final-file guard is unavailable")
}

type unavailableReceiptProvider struct{}

func (unavailableReceiptProvider) Available() error {
	return fmt.Errorf("durable recovery receipts are unavailable")
}
func (unavailableReceiptProvider) Capture(context.Context, Target, RecoveryEvidence, time.Time) (RecycleReceipt, error) {
	return RecycleReceipt{}, fmt.Errorf("durable recovery receipts are unavailable")
}
func (unavailableReceiptProvider) Rebind(context.Context, RecycleReceipt, Target, time.Time) (RecoveryEvidence, error) {
	return RecoveryEvidence{}, fmt.Errorf("durable recovery receipts are unavailable")
}

func NewUnavailablePreflight() NoFallbackPreflight   { return unavailablePreflight{} }
func NewUnavailableFinalGuard() FinalFileGuard       { return unavailableFinalGuard{} }
func NewUnavailableReceiptProvider() ReceiptProvider { return unavailableReceiptProvider{} }
