package execution

import (
	"fmt"
	"sync"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/storage"
)

type Permission string

const (
	// RecycleOnly is the Shell recovery path. It remains gated by the M3B
	// architecture and is not used by the permanent prototype.
	RecycleOnly Permission = "recycle_only"
	// PermanentOnly is reserved for the narrow crash-dump handle-bound
	// prototype. It grants no recovery and is compiled out of ordinary builds.
	PermanentOnly Permission = "permanent_delete"
)

// Target is a native-only exact identity. It must never be reconstructed from
// frontend display fields.
type Target struct {
	ID   string
	File storage.FileRecord
}

// Context is immutable after construction and can be consumed exactly once.
type Context struct {
	mu          sync.Mutex
	id          string
	ruleID      string
	ruleVersion string
	root        string
	targets     []Target
	expiresAt   time.Time
	permission  Permission
	consumed    bool
}

func NewContext(id, ruleID, ruleVersion, root string, targets []Target, expiresAt time.Time, permission Permission) (*Context, error) {
	if id == "" || ruleID == "" || ruleVersion == "" || root == "" {
		return nil, fmt.Errorf("execution context identity is incomplete")
	}
	if permission != RecycleOnly && permission != PermanentOnly {
		return nil, fmt.Errorf("unsupported execution permission")
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("execution context has no targets")
	}
	if !expiresAt.After(time.Now()) {
		return nil, fmt.Errorf("execution context is already expired")
	}
	seenIDs := make(map[string]bool, len(targets))
	seenFiles := make(map[string]bool, len(targets))
	copied := make([]Target, 0, len(targets))
	for _, target := range targets {
		if target.ID == "" || target.File.FileID == "" {
			return nil, fmt.Errorf("execution target identity is incomplete")
		}
		if !target.File.RootEqual(root) {
			return nil, fmt.Errorf("execution target is outside the approved root")
		}
		if seenIDs[target.ID] || seenFiles[target.File.FileID] {
			return nil, fmt.Errorf("execution context contains a duplicate target")
		}
		seenIDs[target.ID] = true
		seenFiles[target.File.FileID] = true
		copied = append(copied, Target{ID: target.ID, File: target.File})
	}
	return &Context{
		id: id, ruleID: ruleID, ruleVersion: ruleVersion, root: root,
		targets: copied, expiresAt: expiresAt, permission: permission,
	}, nil
}

func (c *Context) ID() string             { return c.id }
func (c *Context) RuleID() string         { return c.ruleID }
func (c *Context) RuleVersion() string    { return c.ruleVersion }
func (c *Context) Root() string           { return c.root }
func (c *Context) ExpiresAt() time.Time   { return c.expiresAt }
func (c *Context) Permission() Permission { return c.permission }

func (c *Context) Targets() []Target {
	copied := make([]Target, len(c.targets))
	copy(copied, c.targets)
	return copied
}

// Consume is the single atomic use of an execution context.
func (c *Context) Consume(now time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.consumed {
		return fmt.Errorf("execution context was already consumed")
	}
	if !now.Before(c.expiresAt) {
		c.consumed = true
		return fmt.Errorf("execution context expired")
	}
	c.consumed = true
	return nil
}
