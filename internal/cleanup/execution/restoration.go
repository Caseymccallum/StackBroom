package execution

import (
	"context"
	"fmt"
	"time"
)

// RecycleReceipt is private backend evidence, never action history or UI data.
// A provider must bind the Shell item's durable identity to this exact target;
// a raw COM pointer or original-path absence is not an acceptable token.
type RecycleReceipt struct {
	TargetID       string
	OriginalFileID string
	Provider       string
	OpaqueToken    string
	ObservedAt     time.Time
	ExpiresAt      time.Time
}

// RestoreInspector is read-only. Native implementation is deliberately absent.
// Inspect must rebind the item in the current user's Recycle Bin and verify
// identity, original location, metadata, and an unoccupied guarded destination.
type RestoreInspector interface {
	Available() error
	Inspect(context.Context, RecycleReceipt, Target) error
}

// RestorePlan has no mutation method. Future execution must consume it once,
// revalidate the receipt and destination under held ancestor guards, refuse
// collisions/elevation, and verify the restored identity and content.
type RestorePlan struct {
	target    Target
	receipt   RecycleReceipt
	checkedAt time.Time
}

func PrepareRestore(ctx context.Context, inspector RestoreInspector, receipt RecycleReceipt, target Target, now time.Time) (*RestorePlan, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if inspector == nil || inspector.Available() != nil {
		return nil, fmt.Errorf("restoration verification unavailable")
	}
	if target.ID == "" || target.File.FileID == "" || receipt.TargetID != target.ID || receipt.OriginalFileID != target.File.FileID ||
		receipt.Provider == "" || receipt.OpaqueToken == "" || receipt.ObservedAt.IsZero() || receipt.ObservedAt.After(now) || !now.Before(receipt.ExpiresAt) {
		return nil, fmt.Errorf("restoration evidence is incomplete, mismatched or expired")
	}
	if err := inspector.Inspect(ctx, receipt, target); err != nil {
		return nil, fmt.Errorf("restoration evidence or destination could not be verified")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return &RestorePlan{target: target, receipt: receipt, checkedAt: now}, nil
}
