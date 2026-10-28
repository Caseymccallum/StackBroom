package main

import "time"

type CleanupRule struct {
	ID               string   `json:"id"`
	Version          string   `json:"version"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Owner            string   `json:"owner"`
	Category         string   `json:"category"`
	ScopeDisplay     string   `json:"scopeDisplay"`
	SupportedWindows string   `json:"supportedWindows"`
	MinAgeDays       int      `json:"minAgeDays"`
	DeletionMode     string   `json:"deletionMode"`
	Recoverable      string   `json:"recoverable"`
	RequiresClosed   []string `json:"requiresClosed,omitempty"`
	Risks            []string `json:"risks"`
	Evidence         []string `json:"evidence"`
	WhySafe          string   `json:"whySafe"`
}

type CleanupProgress struct {
	Phase   string `json:"phase"`
	Items   int    `json:"items"`
	Bytes   uint64 `json:"bytes"`
	Errors  int    `json:"errors"`
	Partial bool   `json:"partial"`
	Message string `json:"message,omitempty"`
}

type CleanupCandidate struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	RelativePath    string    `json:"relativePath"`
	LogicalBytes    uint64    `json:"logicalBytes"`
	AllocatedBytes  uint64    `json:"allocatedBytes"`
	AllocatedKnown  bool      `json:"allocatedKnown"`
	ModifiedAt      time.Time `json:"modifiedAt"`
	LinkCount       uint32    `json:"linkCount,omitempty"`
	Uncertain       bool      `json:"uncertain"`
	UncertainReason string    `json:"uncertainReason,omitempty"`
}

type CleanupExclusion struct {
	RelativePath string `json:"relativePath"`
	Reason       string `json:"reason"`
}

type CleanupDiscoveryView struct {
	DiscoveryID      string             `json:"discoveryId"`
	Rule             CleanupRule        `json:"rule"`
	Status           string             `json:"status"`
	ErrorCode        string             `json:"errorCode,omitempty"`
	ErrorMessage     string             `json:"errorMessage,omitempty"`
	Progress         CleanupProgress    `json:"progress"`
	Candidates       []CleanupCandidate `json:"candidates"`
	Excluded         []CleanupExclusion `json:"excluded"`
	ExcludedCounts   map[string]int     `json:"excludedCounts"`
	ExcludedTotal    int                `json:"excludedTotal"`
	ScannedFiles     int                `json:"scannedFiles"`
	Eligible         int                `json:"eligible"`
	TotalLogical     uint64             `json:"totalLogical"`
	TotalAllocated   uint64             `json:"totalAllocated"`
	AllocatedUnknown int                `json:"allocatedUnknown"`
	Truncated        bool               `json:"truncated"`
	Partial          bool               `json:"partial"`
	PartialReason    string             `json:"partialReason,omitempty"`
	Coverage         StorageCoverage    `json:"coverage"`
	SkippedCount     int                `json:"skippedCount"`
	StartedAt        time.Time          `json:"startedAt"`
	CompletedAt      time.Time          `json:"completedAt"`
}

type CleanupPreviewView struct {
	PreviewID        string             `json:"previewId"`
	RuleID           string             `json:"ruleId"`
	RuleVersion      string             `json:"ruleVersion"`
	RuleTitle        string             `json:"ruleTitle"`
	Selected         []CleanupCandidate `json:"selected"`
	Excluded         []CleanupExclusion `json:"excluded"`
	FileCount        int                `json:"fileCount"`
	TotalLogical     uint64             `json:"totalLogical"`
	TotalAllocated   uint64             `json:"totalAllocated"`
	RecoverableBytes uint64             `json:"recoverableBytes"`
	RecoverableKnown bool               `json:"recoverableKnown"`
	AllocatedUnknown int                `json:"allocatedUnknown"`
	DeletionMode     string             `json:"deletionMode"`
	RecoveryNote     string             `json:"recoveryNote"`
	Risks            []string           `json:"risks"`
	SideEffects      []string           `json:"sideEffects"`
	ExpiresAt        time.Time          `json:"expiresAt"`
}
