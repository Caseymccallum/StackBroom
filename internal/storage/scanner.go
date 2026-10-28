package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Scanner performs one read-only traversal. It is not safe for concurrent Scan
// calls; the desktop bridge owns scan lifecycle and cancellation.
type Scanner struct{}

func NewScanner() *Scanner { return &Scanner{} }

type scanState struct {
	request         Request
	root            string
	rootNode        *Node
	nodes           map[string]*Node
	files           []*FileRecord
	seenFileIDs     map[string]bool
	skipped         []Skip
	skippedCount    int
	totalLogical    uint64
	totalAlloc      uint64
	totalFiles      int
	totalDirs       int
	items           int
	limit           bool
	cancelled       bool
	accessDenied    int
	hiddenExcluded  int
	systemExcluded  int
	reparseExcluded int
	depthLimit      bool
	progress        func(Progress)
	lastProgress    time.Time
}

// ValidateRoot resolves an explicitly selected folder and refuses reparse
// points. The returned path is the normal user-facing absolute path.
func ValidateRoot(path string) (string, error) {
	root, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("invalid root: %w", err)
	}
	info, err := os.Lstat(platformPath(root))
	if err != nil {
		return "", fmt.Errorf("cannot read root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("root is not a directory")
	}
	if inspectFile(root, info).Reparse {
		return "", fmt.Errorf("root is a reparse point; choose a normal folder")
	}
	return root, nil
}

// Scan traverses the approved root only. Reparse points are never followed.
func (s *Scanner) Scan(ctx context.Context, request Request, progress func(Progress)) (*Result, error) {
	start := time.Now()
	request = normalizeRequest(request)
	root, err := filepath.Abs(filepath.Clean(request.Root))
	if err != nil {
		return nil, fmt.Errorf("invalid root: %w", err)
	}
	info, err := os.Lstat(platformPath(root))
	if err != nil {
		return nil, fmt.Errorf("cannot read scan root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scan root is not a directory")
	}
	rootMeta := inspectFile(root, info)
	if rootMeta.Reparse {
		return nil, fmt.Errorf("scan root is a reparse point; choose a normal folder")
	}

	state := &scanState{
		request:     request,
		root:        root,
		nodes:       make(map[string]*Node),
		seenFileIDs: make(map[string]bool),
		progress:    progress,
	}
	state.rootNode = state.newNode("", root, "dir")
	state.emit("enumerating", "")
	state.scanDirectory(ctx, state.rootNode, 0)

	coverage := Coverage{
		AccessDenied:    state.accessDenied,
		HiddenExcluded:  state.hiddenExcluded,
		SystemExcluded:  state.systemExcluded,
		ReparseExcluded: state.reparseExcluded,
		DepthLimit:      state.depthLimit,
		EntryLimit:      state.limit,
		Cancelled:       errors.Is(ctx.Err(), context.Canceled),
		TimedOut:        errors.Is(ctx.Err(), context.DeadlineExceeded),
	}
	coverage.Exhaustive = coverage.AccessDenied == 0 && coverage.HiddenExcluded == 0 &&
		coverage.SystemExcluded == 0 && coverage.ReparseExcluded == 0 && !coverage.DepthLimit &&
		!coverage.EntryLimit && !coverage.Cancelled && !coverage.TimedOut
	result := &Result{
		Root:             state.rootNode,
		Nodes:            state.nodes,
		Files:            state.files,
		TotalLogical:     state.totalLogical,
		TotalAllocated:   state.totalAlloc,
		TotalFiles:       state.totalFiles,
		TotalDirectories: state.totalDirs,
		Skipped:          state.skipped,
		SkippedCount:     state.skippedCount,
		Coverage:         coverage,
		Partial:          state.limit || state.cancelled || ctx.Err() != nil,
		Duration:         time.Since(start),
	}
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		result.PartialReason = "cancelled"
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		result.PartialReason = "timeout"
	case state.limit:
		result.PartialReason = fmt.Sprintf("entry limit reached (%d)", request.MaxEntries)
	case ctx.Err() != nil:
		result.PartialReason = "scan stopped"
	}
	if coverage.Cancelled {
		result.Coverage.Reasons = append(result.Coverage.Reasons, "cancelled")
	}
	if coverage.TimedOut {
		result.Coverage.Reasons = append(result.Coverage.Reasons, "timeout")
	}
	if coverage.AccessDenied > 0 {
		result.Coverage.Reasons = append(result.Coverage.Reasons, fmt.Sprintf("access denied on %d path(s)", coverage.AccessDenied))
	}
	if coverage.HiddenExcluded > 0 {
		result.Coverage.Reasons = append(result.Coverage.Reasons, fmt.Sprintf("%d hidden item(s) excluded", coverage.HiddenExcluded))
	}
	if coverage.SystemExcluded > 0 {
		result.Coverage.Reasons = append(result.Coverage.Reasons, fmt.Sprintf("%d system item(s) excluded", coverage.SystemExcluded))
	}
	if coverage.ReparseExcluded > 0 {
		result.Coverage.Reasons = append(result.Coverage.Reasons, fmt.Sprintf("%d reparse point(s) not followed", coverage.ReparseExcluded))
	}
	if coverage.DepthLimit {
		result.Coverage.Reasons = append(result.Coverage.Reasons, "depth limit reached")
	}
	if coverage.EntryLimit {
		result.Coverage.Reasons = append(result.Coverage.Reasons, "entry limit reached")
	}
	state.finalize(result)
	state.emit("aggregating", result.PartialReason)
	if len(state.files) > 0 && ctx.Err() == nil {
		hashStart := time.Now()
		var duplicateCoverage DuplicateCoverage
		result.Duplicates, duplicateCoverage = s.findDuplicates(ctx, state.files, request, func(done, total int) {
			state.emitDetailed("hashing", fmt.Sprintf("%d/%d candidate files", done, total), false)
		})
		result.DuplicateCoverage = duplicateCoverage
		result.HashDuration = time.Since(hashStart)
		if ctx.Err() == context.Canceled {
			result.Partial = true
			result.PartialReason = "cancelled during duplicate hashing"
		}
	} else if ctx.Err() != nil {
		result.DuplicateCoverage = DuplicateCoverage{
			Enumerated: len(state.files),
			Truncated:  true,
		}
	}
	result.Duration = time.Since(start)
	state.emit("complete", result.PartialReason)
	return result, nil
}

func normalizeRequest(request Request) Request {
	defaults := DefaultRequest(request.Root)
	if request.MaxDepth <= 0 {
		request.MaxDepth = defaults.MaxDepth
	}
	if request.MaxDepth > 256 {
		request.MaxDepth = 256
	}
	if request.MaxEntries <= 0 {
		request.MaxEntries = defaults.MaxEntries
	}
	if request.MaxEntries > 2000000 {
		request.MaxEntries = 2000000
	}
	if request.Workers <= 0 {
		request.Workers = defaults.Workers
	}
	if request.Workers > 32 {
		request.Workers = 32
	}
	if request.MinDuplicateSize == 0 {
		request.MinDuplicateSize = defaults.MinDuplicateSize
	}
	if request.MaxHashBytes == 0 {
		request.MaxHashBytes = defaults.MaxHashBytes
	}
	if request.MaxDuplicateFiles <= 0 {
		request.MaxDuplicateFiles = defaults.MaxDuplicateFiles
	}
	return request
}

func (s *scanState) newNode(parentID, path, kind string) *Node {
	node := &Node{
		ID:       fmt.Sprintf("n%d", len(s.nodes)+1),
		ParentID: parentID,
		Name:     filepath.Base(path),
		Path:     path,
		Kind:     kind,
	}
	if parentID == "" {
		node.Name = rootLabel(path)
	}
	s.nodes[node.ID] = node
	return node
}

func rootLabel(path string) string {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	if clean == volume || clean == volume+string(filepath.Separator) {
		return clean
	}
	label := filepath.Base(clean)
	if label == "." || label == string(filepath.Separator) || label == "" {
		return clean
	}
	return label
}
func (s *scanState) scanDirectory(ctx context.Context, directory *Node, depth int) {
	if s.limit || s.cancelled {
		return
	}
	if ctx.Err() != nil {
		s.cancelled = true
		return
	}
	if !pathWithinRoot(s.root, directory.Path) || !resolvedPathWithinRoot(s.root, directory.Path) {
		directory.Truncated = true
		s.accessDenied++
		s.addSkip(directory.Path, "folder is outside the approved root or has a reparse ancestor")
		return
	}
	before, err := os.Lstat(platformPath(directory.Path))
	if err != nil || !before.IsDir() || inspectFile(directory.Path, before).Reparse {
		directory.Truncated = true
		s.accessDenied++
		s.addSkip(directory.Path, "folder became unreadable or a reparse point")
		return
	}
	directoryHandle, err := openVerifiedDirectory(directory.Path)
	if err != nil {
		directory.Truncated = true
		s.accessDenied++
		s.addSkip(directory.Path, "folder could not be enumerated")
		return
	}
	defer directoryHandle.Close()
	for {
		names, readErr := directoryHandle.Readdirnames(256)
		for _, entryName := range names {
			if s.limit || s.cancelled {
				return
			}
			if ctx.Err() != nil {
				s.cancelled = true
				return
			}
			if s.items >= s.request.MaxEntries {
				s.limit = true
				return
			}
			s.items++
			fullPath := filepath.Join(directory.Path, entryName)
			info, err := os.Lstat(platformPath(fullPath))
			if err != nil {
				s.accessDenied++
				s.addSkip(fullPath, "path could not be inspected")
				continue
			}
			meta := inspectFile(fullPath, info)
			if !s.request.IncludeHidden && meta.Hidden {
				s.hiddenExcluded++
				s.addSkip(fullPath, "hidden item excluded")
				continue
			}
			if !s.request.IncludeSystem && meta.System {
				s.systemExcluded++
				s.addSkip(fullPath, "system item excluded")
				continue
			}
			if meta.Reparse || info.Mode()&os.ModeSymlink != 0 {
				link := s.newNode(directory.ID, fullPath, "link")
				link.IsReparse = true
				link.Truncated = true
				directory.Children = append(directory.Children, link)
				directory.DirCount++
				s.reparseExcluded++
				s.addSkip(fullPath, "reparse point not followed")
				continue
			}
			if info.IsDir() {
				if depth >= s.request.MaxDepth {
					directory.Truncated = true
					s.depthLimit = true
					s.addSkip(fullPath, fmt.Sprintf("maximum depth %d reached", s.request.MaxDepth))
					continue
				}
				child := s.newNode(directory.ID, fullPath, "dir")
				child.IsHidden = meta.Hidden
				child.IsSystem = meta.System
				directory.Children = append(directory.Children, child)
				directory.DirCount++
				s.totalDirs++
				s.scanDirectory(ctx, child, depth+1)
				directory.LogicalBytes += child.LogicalBytes
				directory.AllocatedBytes += child.AllocatedBytes
				continue
			}

			size := uint64(0)
			if info.Size() > 0 {
				size = uint64(info.Size())
			}
			allocated := size
			if meta.AllocatedKnown {
				allocated = meta.AllocatedBytes
			}
			accounted := true
			if meta.FileID != "" {
				if s.seenFileIDs[meta.FileID] {
					accounted = false
				} else {
					s.seenFileIDs[meta.FileID] = true
				}
			}
			fileNode := s.newNode(directory.ID, fullPath, "file")
			fileNode.LogicalBytes = size
			fileNode.AllocatedBytes = allocated
			fileNode.LinkCount = meta.LinkCount
			fileNode.IsPlaceholder = meta.Placeholder
			fileNode.IsSparse = meta.Sparse
			fileNode.IsHidden = meta.Hidden
			fileNode.IsSystem = meta.System
			directory.Children = append(directory.Children, fileNode)
			directory.FileCount++
			s.totalFiles++
			if accounted {
				directory.LogicalBytes += size
				directory.AllocatedBytes += allocated
				s.totalLogical += size
				s.totalAlloc += allocated
			}
			s.files = append(s.files, &FileRecord{
				NodeID:         fileNode.ID,
				Root:           s.root,
				Path:           fullPath,
				RelativePath:   s.relative(fullPath),
				Name:           entryName,
				Extension:      normalizeExtension(filepath.Ext(entryName)),
				LogicalBytes:   size,
				AllocatedBytes: allocated,
				AllocatedKnown: meta.AllocatedKnown,
				LinkCount:      meta.LinkCount,
				FileID:         meta.FileID,
				ModTime:        info.ModTime(),
				Placeholder:    meta.Placeholder,
				Reparse:        meta.Reparse,
				Sparse:         meta.Sparse,
				Accounted:      accounted,
			})
			s.maybeProgress()
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			directory.Truncated = true
			s.accessDenied++
			s.addSkip(directory.Path, "folder enumeration stopped unexpectedly")
			break
		}
	}
}

func normalizeExtension(ext string) string {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if ext == "" {
		return "(none)"
	}
	return ext
}

func (s *scanState) relative(path string) string {
	relative, err := filepath.Rel(s.root, path)
	if err != nil {
		return path
	}
	return relative
}

func (s *scanState) addSkip(path, reason string) {
	s.skippedCount++
	if len(s.skipped) < 200 {
		s.skipped = append(s.skipped, Skip{RelativePath: s.relative(path), Reason: reason})
	}
}

func (s *scanState) maybeProgress() {
	if s.progress == nil {
		return
	}
	if time.Since(s.lastProgress) < 150*time.Millisecond && s.items%1000 != 0 {
		return
	}
	s.lastProgress = time.Now()
	s.emit("enumerating", "")
}

func (s *scanState) emit(phase, message string) {
	s.emitDetailed(phase, message, s.limit || s.cancelled)
}

func (s *scanState) emitDetailed(phase, message string, partial bool) {
	if s.progress == nil {
		return
	}
	s.progress(Progress{
		Phase:   phase,
		Items:   s.items,
		Bytes:   s.totalLogical,
		Errors:  s.skippedCount,
		Partial: partial,
		Message: message,
	})
}

func (s *scanState) finalize(result *Result) {
	for _, node := range s.nodes {
		sort.Slice(node.Children, func(i, j int) bool {
			if node.Children[i].LogicalBytes != node.Children[j].LogicalBytes {
				return node.Children[i].LogicalBytes > node.Children[j].LogicalBytes
			}
			return node.Children[i].Name < node.Children[j].Name
		})
	}
	typeTotals := make(map[string]*TypeStat)
	for _, file := range s.files {
		stat := typeTotals[file.Extension]
		if stat == nil {
			stat = &TypeStat{Extension: file.Extension}
			typeTotals[file.Extension] = stat
		}
		stat.Files++
		stat.LogicalBytes += file.LogicalBytes
	}
	result.Types = make([]TypeStat, 0, len(typeTotals))
	for _, stat := range typeTotals {
		result.Types = append(result.Types, *stat)
	}
	sort.Slice(result.Types, func(i, j int) bool {
		if result.Types[i].LogicalBytes != result.Types[j].LogicalBytes {
			return result.Types[i].LogicalBytes > result.Types[j].LogicalBytes
		}
		return result.Types[i].Extension < result.Types[j].Extension
	})
	result.LargestFiles = append(result.LargestFiles, s.files...)
	sort.Slice(result.LargestFiles, func(i, j int) bool {
		if result.LargestFiles[i].LogicalBytes != result.LargestFiles[j].LogicalBytes {
			return result.LargestFiles[i].LogicalBytes > result.LargestFiles[j].LogicalBytes
		}
		return result.LargestFiles[i].RelativePath < result.LargestFiles[j].RelativePath
	})
	if len(result.LargestFiles) > 50 {
		result.LargestFiles = result.LargestFiles[:50]
	}
	for _, node := range s.nodes {
		if node.Kind == "dir" && node.ID != s.rootNode.ID {
			result.LargestFolders = append(result.LargestFolders, node)
		}
	}
	sort.Slice(result.LargestFolders, func(i, j int) bool {
		if result.LargestFolders[i].LogicalBytes != result.LargestFolders[j].LogicalBytes {
			return result.LargestFolders[i].LogicalBytes > result.LargestFolders[j].LogicalBytes
		}
		return result.LargestFolders[i].Path < result.LargestFolders[j].Path
	})
	if len(result.LargestFolders) > 50 {
		result.LargestFolders = result.LargestFolders[:50]
	}
}
