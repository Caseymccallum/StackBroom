package storage

import "time"

// Request describes one explicitly authorised, read-only storage scan.
type Request struct {
	Root              string
	IncludeHidden     bool
	IncludeSystem     bool
	MaxDepth          int
	MaxEntries        int
	Workers           int
	MinDuplicateSize  uint64
	MaxHashBytes      uint64
	MaxDuplicateFiles int
}

// DefaultRequest returns conservative bounds for a user-selected root.
func DefaultRequest(root string) Request {
	return Request{
		Root:              root,
		MaxDepth:          64,
		MaxEntries:        250000,
		Workers:           4,
		MinDuplicateSize:  4096,
		MaxHashBytes:      8 << 30,
		MaxDuplicateFiles: 4000,
	}
}

// Progress is measured work only. It never contains a fabricated percentage.
type Progress struct {
	Phase   string
	Items   int
	Bytes   uint64
	Errors  int
	Partial bool
	Message string
}

// Node is one directory or file in the in-memory tree. Path is internal and is
// never sent across the desktop bridge.
type Node struct {
	ID             string
	ParentID       string
	Name           string
	Path           string
	Kind           string
	LogicalBytes   uint64
	AllocatedBytes uint64
	FileCount      int
	DirCount       int
	LinkCount      uint32
	IsPlaceholder  bool
	IsReparse      bool
	IsSparse       bool
	IsHidden       bool
	IsSystem       bool
	Truncated      bool
	Children       []*Node
}

// FileRecord keeps the metadata needed for large-file and duplicate evidence.
type FileRecord struct {
	NodeID         string
	Root           string
	Path           string
	RelativePath   string
	Name           string
	Extension      string
	LogicalBytes   uint64
	AllocatedBytes uint64
	AllocatedKnown bool
	LinkCount      uint32
	FileID         string
	ModTime        time.Time
	Placeholder    bool
	Reparse        bool
	Sparse         bool
	Accounted      bool
}

// Skip records a path that could not be read or was deliberately excluded.
type Skip struct {
	RelativePath string
	Reason       string
}

// TypeStat is a measured extension summary.
type TypeStat struct {
	Extension    string
	Files        int
	LogicalBytes uint64
}

// DuplicateFile identifies one independently allocated candidate.
type DuplicateFile struct {
	NodeID       string
	RelativePath string
	Name         string
	LinkCount    uint32
	FileID       string
}

// DuplicateGroup is verified by complete content hashing.
type DuplicateGroup struct {
	ID    string
	Size  uint64
	Hash  string
	Files []DuplicateFile
	Note  string
}

// Coverage distinguishes an exhaustive traversal from one with excluded or
// unread data. A result with hidden/system exclusions is not exhaustive.
type Coverage struct {
	Exhaustive      bool
	AccessDenied    int
	HiddenExcluded  int
	SystemExcluded  int
	ReparseExcluded int
	DepthLimit      bool
	EntryLimit      bool
	Cancelled       bool
	TimedOut        bool
	Reasons         []string
}

// DuplicateCoverage makes duplicate-analysis limits explicit.
type DuplicateCoverage struct {
	Enumerated          int
	Eligible            int
	PolicySkipped       int
	CandidateConsidered int
	CandidateCap        int
	HashAttempted       int
	HashFailed          int
	Verified            int
	Changed             int
	Truncated           bool
}

// Result is the complete internal scan result. Only a bounded projection of it
// crosses the bridge.
type Result struct {
	Root              *Node
	Nodes             map[string]*Node
	Files             []*FileRecord
	TotalLogical      uint64
	TotalAllocated    uint64
	TotalFiles        int
	TotalDirectories  int
	Skipped           []Skip
	SkippedCount      int
	Types             []TypeStat
	LargestFolders    []*Node
	LargestFiles      []*FileRecord
	Duplicates        []DuplicateGroup
	Coverage          Coverage
	DuplicateCoverage DuplicateCoverage
	Partial           bool
	PartialReason     string
	Duration          time.Duration
	HashDuration      time.Duration
}
