package main

import "time"

// Storage bridge DTOs. Raw filesystem paths remain inside the Go engine. The
// selected root is the only absolute path shown, because the user chose it.

type StorageRoot struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	DisplayPath string    `json:"displayPath"`
	WholeDrive  bool      `json:"wholeDrive,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type StorageScanOptions struct {
	IncludeHidden bool `json:"includeHidden"`
	IncludeSystem bool `json:"includeSystem"`
	MaxDepth      int  `json:"maxDepth"`
	MaxEntries    int  `json:"maxEntries"`
}

type StorageProgress struct {
	Phase   string `json:"phase"`
	Items   int    `json:"items"`
	Bytes   uint64 `json:"bytes"`
	Errors  int    `json:"errors"`
	Partial bool   `json:"partial"`
	Message string `json:"message,omitempty"`
}

type StorageNode struct {
	ID             string `json:"id"`
	ParentID       string `json:"parentId,omitempty"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	LogicalBytes   uint64 `json:"logicalBytes"`
	AllocatedBytes uint64 `json:"allocatedBytes"`
	FileCount      int    `json:"fileCount"`
	DirCount       int    `json:"dirCount"`
	LinkCount      uint32 `json:"linkCount,omitempty"`
	ChildCount     int    `json:"childCount"`
	IsPlaceholder  bool   `json:"isPlaceholder,omitempty"`
	IsReparse      bool   `json:"isReparse,omitempty"`
	IsSparse       bool   `json:"isSparse,omitempty"`
	Truncated      bool   `json:"truncated,omitempty"`
}

type StorageFile struct {
	NodeID         string `json:"nodeId"`
	Name           string `json:"name"`
	RelativePath   string `json:"relativePath"`
	Extension      string `json:"extension"`
	LogicalBytes   uint64 `json:"logicalBytes"`
	AllocatedBytes uint64 `json:"allocatedBytes"`
	AllocatedKnown bool   `json:"allocatedKnown"`
	LinkCount      uint32 `json:"linkCount,omitempty"`
	IsPlaceholder  bool   `json:"isPlaceholder,omitempty"`
	IsSparse       bool   `json:"isSparse,omitempty"`
}

type StorageTypeStat struct {
	Extension    string `json:"extension"`
	Files        int    `json:"files"`
	LogicalBytes uint64 `json:"logicalBytes"`
}

type StorageDuplicateFile struct {
	NodeID       string `json:"nodeId"`
	Name         string `json:"name"`
	RelativePath string `json:"relativePath"`
	LinkCount    uint32 `json:"linkCount,omitempty"`
}

type StorageDuplicateGroup struct {
	ID    string                 `json:"id"`
	Size  uint64                 `json:"size"`
	Hash  string                 `json:"hash"`
	Note  string                 `json:"note"`
	Files []StorageDuplicateFile `json:"files"`
}

type StorageScanView struct {
	ScanID         string                  `json:"scanId"`
	RootID         string                  `json:"rootId"`
	RootLabel      string                  `json:"rootLabel"`
	Status         string                  `json:"status"`
	Progress       StorageProgress         `json:"progress"`
	Partial        bool                    `json:"partial"`
	PartialReason  string                  `json:"partialReason,omitempty"`
	TotalLogical   uint64                  `json:"totalLogical"`
	TotalAllocated uint64                  `json:"totalAllocated"`
	TotalFiles     int                     `json:"totalFiles"`
	TotalDirs      int                     `json:"totalDirs"`
	SkippedCount   int                     `json:"skippedCount"`
	DurationMS     int64                   `json:"durationMs"`
	HashDurationMS int64                   `json:"hashDurationMs"`
	StartedAt      time.Time               `json:"startedAt"`
	CompletedAt    time.Time               `json:"completedAt"`
	Nodes          []StorageNode           `json:"nodes"`
	LargestFolders []StorageNode           `json:"largestFolders"`
	LargestFiles   []StorageFile           `json:"largestFiles"`
	Types          []StorageTypeStat       `json:"types"`
	Duplicates     []StorageDuplicateGroup `json:"duplicates"`
	Skipped        []StorageSkip           `json:"skipped,omitempty"`
	Warnings       []string                `json:"warnings,omitempty"`
}

type StorageSkip struct {
	RelativePath string `json:"relativePath"`
	Reason       string `json:"reason"`
}

type StorageChildren struct {
	Parent    StorageNode   `json:"parent"`
	Children  []StorageNode `json:"children"`
	Truncated bool          `json:"truncated"`
}
