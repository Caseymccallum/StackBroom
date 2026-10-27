package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/privacy"
	"github.com/Caseymccallum/StackBroom/internal/scan"
)

// CancelScan requests cancellation of the active scan. It never mutates the
// system and returns false when nothing is running.
func (s *DesktopService) CancelScan() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return false
	}
	s.active.cancel()
	return true
}

func (s *DesktopService) begin() (context.Context, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		return nil, "", ErrScanInProgress
	}
	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	id := fmt.Sprintf("scan-%d", s.sequence.Add(1))
	s.active = &activeScan{id: id, cancel: cancel}
	return ctx, id, nil
}

func (s *DesktopService) finish(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil && s.active.id == id {
		s.active.cancel()
		s.active = nil
	}
}

func runEngine(ctx context.Context, mode string) model.Snapshot {
	full := mode == "full"
	scanner := scan.New(scan.Options{
		Timeout:            20 * time.Second,
		EnrichCommandLines: full,
		SkipDocker:         !full,
		SkipWSL:            !full,
		TopN:               8,
	})
	return scanner.Run(ctx)
}

// sanitize is defense in depth: the DTO mapper never reads command lines or
// project roots, but the privacy policy is applied before any data can cross
// the bridge.
func sanitize(snapshot model.Snapshot) model.Snapshot {
	return privacy.Apply(snapshot, privacy.DefaultPolicy(privacy.ModeDefault))
}

func scanStatus(ctx context.Context) string {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return "timeout"
	case ctx.Err() != nil:
		return "cancelled"
	default:
		return "ok"
	}
}

func mergeWarnings(first, second []string, mode string) []string {
	seen := map[string]bool{}
	var out []string
	for _, warning := range append(append([]string{}, first...), second...) {
		if mode == "quick" && (strings.Contains(warning, "Docker inspection was skipped") ||
			strings.Contains(warning, "WSL inspection was skipped")) {
			continue
		}
		warning = strings.TrimSpace(warning)
		if warning == "" || seen[warning] {
			continue
		}
		seen[warning] = true
		out = append(out, warning)
	}
	return out
}
