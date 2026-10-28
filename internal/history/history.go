package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/Caseymccallum/StackBroom/internal/cleanup"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
)

// Store is a local, privacy-filtered action journal. It stores no paths,
// command lines, file contents, credentials or target names.
type Store struct {
	mu            sync.Mutex
	path          string
	retentionDays int
	maxRecords    int
	now           func() time.Time
}

type Record struct {
	At                time.Time `json:"at"`
	Type              string    `json:"type"`
	RuleID            string    `json:"ruleId"`
	RuleVersion       string    `json:"ruleVersion"`
	State             string    `json:"state"`
	TargetCount       int       `json:"targetCount"`
	LogicalBytes      uint64    `json:"logicalBytes"`
	AllocatedBytes    uint64    `json:"allocatedBytes"`
	DeletionMode      string    `json:"deletionMode"`
	ReasonCode        string    `json:"reasonCode,omitempty"`
	RecoveryAvailable bool      `json:"recoveryAvailable"`
}

func NewStore(path string, retentionDays, maxRecords int) *Store {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	if maxRecords <= 0 {
		maxRecords = 500
	}
	return &Store{path: path, retentionDays: retentionDays, maxRecords: maxRecords, now: time.Now}
}

// Record implements execution.Recorder.
func (s *Store) Record(event execution.Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return
	}
	entry := Record{
		At: event.At, Type: event.Type, RuleID: event.RuleID, RuleVersion: event.RuleVersion,
		State: string(event.State), TargetCount: event.TargetCount, LogicalBytes: event.LogicalBytes,
		AllocatedBytes: event.AllocatedBytes, DeletionMode: event.DeletionMode,
		ReasonCode: event.ReasonCode, RecoveryAvailable: event.RecoveryAvailable,
	}
	entry = privacyFilter(entry)
	entries, readErr := s.readLocked()
	if readErr != nil {
		return
	}
	entries = append(entries, entry)
	entries = s.pruneLocked(entries)
	if err := s.writeLocked(entries); err != nil {
		// History failure must not alter cleanup outcomes.
		return
	}
}

func (s *Store) List() ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.readLocked()
	return s.pruneLocked(entries), err
}

func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return nil
	}
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Store) readLocked() ([]Record, error) {
	if s.path == "" {
		return nil, nil
	}
	file, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var entries []Record
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry Record
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("history contains an invalid record")
		}
		entries = append(entries, privacyFilter(entry))
	}
	return entries, scanner.Err()
}

func (s *Store) writeLocked(entries []Record) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	temporary := s.path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			file.Close()
			_ = os.Remove(temporary)
			return err
		}
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, s.path)
}

func (s *Store) pruneLocked(entries []Record) []Record {
	cutoff := s.now().Add(-time.Duration(s.retentionDays) * 24 * time.Hour)
	kept := make([]Record, 0, len(entries))
	for _, entry := range entries {
		if entry.At.After(cutoff) {
			kept = append(kept, entry)
		}
	}
	if len(kept) > s.maxRecords {
		kept = kept[len(kept)-s.maxRecords:]
	}
	return kept
}

func (s *Store) String() string {
	return fmt.Sprintf("history(%s, %d days, max %d)", s.path, s.retentionDays, s.maxRecords)
}

// Closed vocabularies filter caller data and journals from older builds.
func privacyFilter(entry Record) Record {
	if entry.Type != "preview" && entry.Type != "execution" {
		entry.Type = "unknown"
	}
	rule, ok := cleanup.RuleByID(entry.RuleID)
	if !ok {
		entry.RuleID = "unknown"
		entry.RuleVersion = "unknown"
	} else if entry.RuleVersion != rule.Version {
		entry.RuleVersion = "unknown"
	}
	switch execution.State(entry.State) {
	case execution.StatePreviewed, execution.StateConfirmed, execution.StateValidating, execution.StateExecuting, execution.StateVerifying, execution.StateCompleted, execution.StatePartial, execution.StateFailed, execution.StateCancelled:
	default:
		entry.State = "unknown"
	}
	if entry.DeletionMode != "recycle" {
		entry.DeletionMode = "unknown"
	}
	switch entry.ReasonCode {
	case "", "recycled", "missing_execution", "context", "rule_changed", "deletion_mode", "prerequisite_unverified", "scope_changed", "execution_unavailable", "recovery_unavailable", "target_changed", "executor_failed", "cancelled", "aborted", "partial", "no_verified_recovery":
	default:
		entry.ReasonCode = "unknown"
	}
	if entry.State != "completed" {
		entry.RecoveryAvailable = false
	}
	if entry.TargetCount < 0 {
		entry.TargetCount = 0
	}
	return entry
}
