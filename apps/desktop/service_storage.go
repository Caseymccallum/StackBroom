package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	maxStorageNodesPerResponse = 200
	maxStorageScans            = 8
	maxStorageRoots            = 12
	storageScanLifetime        = time.Hour
	storageRootLifetime        = 24 * time.Hour
)

type storageRootGrant struct {
	ID          string
	Path        string
	Label       string
	DisplayPath string
	WholeDrive  bool
	CreatedAt   time.Time
}

type storageScanSession struct {
	ID          string
	RootID      string
	Status      string
	Progress    storage.Progress
	Result      *storage.Result
	Error       string
	StartedAt   time.Time
	CompletedAt time.Time
	Cancel      context.CancelFunc
}

// StorageService exposes only user-authorised, read-only storage analysis.
type StorageService struct {
	mu        sync.Mutex
	roots     map[string]*storageRootGrant
	scans     map[string]*storageScanSession
	active    string
	selectDir func() (string, error)
	scanFn    func(context.Context, storage.Request, func(storage.Progress)) (*storage.Result, error)
}

func NewStorageService() *StorageService {
	return &StorageService{
		roots:     make(map[string]*storageRootGrant),
		scans:     make(map[string]*storageScanSession),
		selectDir: selectStorageFolder,
		scanFn: func(ctx context.Context, request storage.Request, progress func(storage.Progress)) (*storage.Result, error) {
			return storage.NewScanner().Scan(ctx, request, progress)
		},
	}
}

func randomStorageID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("cannot create storage identifier: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func selectStorageFolder() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("desktop application is not available")
	}
	path, err := app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetTitle("Choose a folder to analyse").
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}

// SelectStorageRoot asks the user for a folder and returns an opaque grant ID.
func (s *StorageService) SelectStorageRoot() (StorageRoot, error) {
	path, err := s.selectDir()
	if err != nil {
		return StorageRoot{}, err
	}
	if path == "" {
		return StorageRoot{}, fmt.Errorf("no folder selected")
	}
	root, err := storage.ValidateRoot(path)
	if err != nil {
		return StorageRoot{}, err
	}
	id, err := randomStorageID()
	if err != nil {
		return StorageRoot{}, err
	}
	label := filepath.Base(root)
	if label == "." || label == string(filepath.Separator) || label == "" {
		label = root
	}
	volume := filepath.VolumeName(root)
	wholeDrive := root == volume || root == volume+string(filepath.Separator)
	grant := &storageRootGrant{
		ID:          id,
		Path:        root,
		Label:       label,
		DisplayPath: root,
		WholeDrive:  wholeDrive,
		CreatedAt:   time.Now(),
	}
	s.mu.Lock()
	s.pruneRootsLocked()
	s.roots[id] = grant
	s.mu.Unlock()
	return StorageRoot{ID: grant.ID, Label: grant.Label, DisplayPath: grant.DisplayPath, WholeDrive: grant.WholeDrive, CreatedAt: grant.CreatedAt}, nil
}

func (s *StorageService) pruneRootsLocked() {
	if len(s.roots) < maxStorageRoots {
		return
	}
	oldestID := ""
	var oldest time.Time
	for id, root := range s.roots {
		if oldestID == "" || root.CreatedAt.Before(oldest) {
			oldestID = id
			oldest = root.CreatedAt
		}
	}
	if oldestID != "" {
		delete(s.roots, oldestID)
	}
}

// StartStorageScan starts one cancellable scan against an authorised root.
func (s *StorageService) StartStorageScan(rootID string, options StorageScanOptions) (string, error) {
	s.mu.Lock()
	grant := s.roots[rootID]
	if grant == nil || time.Since(grant.CreatedAt) > storageRootLifetime {
		if grant != nil {
			delete(s.roots, rootID)
		}
		s.mu.Unlock()
		return "", fmt.Errorf("storage root authorisation expired; choose the folder again")
	}
	if s.active != "" {
		if active := s.scans[s.active]; active != nil && active.Status == "running" {
			s.mu.Unlock()
			return "", fmt.Errorf("a storage scan is already running")
		}
	}
	s.pruneScansLocked()
	scanID, err := randomStorageID()
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	request := storage.DefaultRequest(grant.Path)
	request.IncludeHidden = options.IncludeHidden
	request.IncludeSystem = options.IncludeSystem
	if options.MaxDepth > 0 {
		request.MaxDepth = options.MaxDepth
	}
	if options.MaxEntries > 0 {
		request.MaxEntries = options.MaxEntries
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	session := &storageScanSession{
		ID:        scanID,
		RootID:    rootID,
		Status:    "running",
		StartedAt: time.Now(),
		Cancel:    cancel,
	}
	s.scans[scanID] = session
	s.active = scanID
	s.mu.Unlock()

	go func() {
		result, scanErr := s.scanFn(ctx, request, func(progress storage.Progress) {
			s.mu.Lock()
			if current := s.scans[scanID]; current != nil {
				current.Progress = progress
			}
			s.mu.Unlock()
		})
		contextErr := ctx.Err()
		cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		session.CompletedAt = time.Now()
		if scanErr != nil {
			session.Status = "error"
			session.Error = scanErr.Error()
		} else {
			session.Result = result
			switch {
			case result.PartialReason == "cancelled" || contextErr == context.Canceled:
				session.Status = "cancelled"
			case result.PartialReason == "timeout" || contextErr == context.DeadlineExceeded:
				session.Status = "timeout"
			default:
				session.Status = "completed"
			}
		}
		if s.active == scanID {
			s.active = ""
		}
	}()
	return scanID, nil
}

func (s *StorageService) pruneScansLocked() {
	if len(s.scans) < maxStorageScans {
		return
	}
	oldestID := ""
	var oldest time.Time
	for id, session := range s.scans {
		if session.Status == "running" {
			continue
		}
		if oldestID == "" || session.StartedAt.Before(oldest) {
			oldestID = id
			oldest = session.StartedAt
		}
	}
	if oldestID != "" {
		delete(s.scans, oldestID)
	}
}

// CancelStorageScan requests cancellation and leaves workers to exit.
func (s *StorageService) CancelStorageScan(scanID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.scans[scanID]
	if session == nil || session.Status != "running" {
		return false
	}
	session.Cancel()
	return true
}

// GetStorageScan returns bounded progress or results for a scan context.
func (s *StorageService) GetStorageScan(scanID string) (StorageScanView, error) {
	s.mu.Lock()
	session := s.scans[scanID]
	if session == nil {
		s.mu.Unlock()
		return StorageScanView{}, fmt.Errorf("storage scan not found")
	}
	if session.Status != "running" && !session.CompletedAt.IsZero() && time.Since(session.CompletedAt) > storageScanLifetime {
		s.mu.Unlock()
		return StorageScanView{}, fmt.Errorf("storage scan result expired")
	}
	view := StorageScanView{
		ScanID:      session.ID,
		RootID:      session.RootID,
		Status:      session.Status,
		StartedAt:   session.StartedAt,
		CompletedAt: session.CompletedAt,
		Progress: StorageProgress{
			Phase:   session.Progress.Phase,
			Items:   session.Progress.Items,
			Bytes:   session.Progress.Bytes,
			Errors:  session.Progress.Errors,
			Partial: session.Progress.Partial,
			Message: session.Progress.Message,
		},
	}
	if session.Error != "" {
		view.Warnings = append(view.Warnings, session.Error)
	}
	result := session.Result
	s.mu.Unlock()
	if result == nil {
		return view, nil
	}
	return projectStorageResult(view, result), nil
}

func projectStorageResult(view StorageScanView, result *storage.Result) StorageScanView {
	view.RootLabel = result.Root.Name
	view.Partial = result.Partial
	view.PartialReason = result.PartialReason
	view.TotalLogical = result.TotalLogical
	view.TotalAllocated = result.TotalAllocated
	view.TotalFiles = result.TotalFiles
	view.TotalDirs = result.TotalDirectories
	view.SkippedCount = result.SkippedCount
	view.DurationMS = result.Duration.Milliseconds()
	view.HashDurationMS = result.HashDuration.Milliseconds()
	if len(result.Root.Children) > maxStorageNodesPerResponse {
		view.Warnings = append(view.Warnings, "Additional root children are not shown in this result page")
	}
	view.Nodes = mapStorageNodes(result.Root.Children, maxStorageNodesPerResponse)
	view.LargestFolders = mapStorageNodes(result.LargestFolders, 50)
	view.LargestFiles = mapStorageFiles(result.LargestFiles, 50)
	view.Types = make([]StorageTypeStat, 0, len(result.Types))
	for index, stat := range result.Types {
		if index >= 100 {
			view.Warnings = append(view.Warnings, "Additional file types are not shown")
			break
		}
		view.Types = append(view.Types, StorageTypeStat{Extension: stat.Extension, Files: stat.Files, LogicalBytes: stat.LogicalBytes})
	}
	duplicateLimit := 100
	if len(result.Duplicates) > duplicateLimit {
		view.Warnings = append(view.Warnings, fmt.Sprintf("%d additional duplicate groups are not shown", len(result.Duplicates)-duplicateLimit))
	}
	for index, group := range result.Duplicates {
		if index >= duplicateLimit {
			break
		}
		files := make([]StorageDuplicateFile, 0, len(group.Files))
		for fileIndex, file := range group.Files {
			if fileIndex >= 25 {
				break
			}
			files = append(files, StorageDuplicateFile{NodeID: file.NodeID, Name: file.Name, RelativePath: file.RelativePath, LinkCount: file.LinkCount})
		}
		view.Duplicates = append(view.Duplicates, StorageDuplicateGroup{
			ID: group.ID, Size: group.Size, Hash: group.Hash, Note: group.Note, Files: files,
		})
	}
	for _, skip := range result.Skipped {
		view.Skipped = append(view.Skipped, StorageSkip{RelativePath: skip.RelativePath, Reason: skip.Reason})
	}
	if result.Partial {
		view.Warnings = append(view.Warnings, "Partial result: "+result.PartialReason)
	}
	if result.SkippedCount > len(result.Skipped) {
		view.Warnings = append(view.Warnings, fmt.Sprintf("%d additional skipped paths are not shown", result.SkippedCount-len(result.Skipped)))
	}
	return view
}

func mapStorageNodes(nodes []*storage.Node, limit int) []StorageNode {
	if len(nodes) > limit {
		nodes = nodes[:limit]
	}
	out := make([]StorageNode, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, mapStorageNode(node))
	}
	return out
}

func mapStorageNode(node *storage.Node) StorageNode {
	return StorageNode{
		ID:             node.ID,
		ParentID:       node.ParentID,
		Name:           node.Name,
		Kind:           node.Kind,
		LogicalBytes:   node.LogicalBytes,
		AllocatedBytes: node.AllocatedBytes,
		FileCount:      node.FileCount,
		DirCount:       node.DirCount,
		LinkCount:      node.LinkCount,
		ChildCount:     len(node.Children),
		IsPlaceholder:  node.IsPlaceholder,
		IsReparse:      node.IsReparse,
		IsSparse:       node.IsSparse,
		Truncated:      node.Truncated,
	}
}

func mapStorageFiles(files []*storage.FileRecord, limit int) []StorageFile {
	if len(files) > limit {
		files = files[:limit]
	}
	out := make([]StorageFile, 0, len(files))
	for _, file := range files {
		out = append(out, StorageFile{
			NodeID:         file.NodeID,
			Name:           file.Name,
			RelativePath:   file.RelativePath,
			Extension:      file.Extension,
			LogicalBytes:   file.LogicalBytes,
			AllocatedBytes: file.AllocatedBytes,
			AllocatedKnown: file.AllocatedKnown,
			LinkCount:      file.LinkCount,
			IsPlaceholder:  file.Placeholder,
			IsSparse:       file.Sparse,
		})
	}
	return out
}

// GetStorageNode returns bounded children for one scan-scoped node ID.
func (s *StorageService) GetStorageNode(scanID, nodeID string) (StorageChildren, error) {
	s.mu.Lock()
	session := s.scans[scanID]
	if session == nil || session.Result == nil {
		s.mu.Unlock()
		return StorageChildren{}, fmt.Errorf("storage scan result not found")
	}
	if !session.CompletedAt.IsZero() && time.Since(session.CompletedAt) > storageScanLifetime {
		s.mu.Unlock()
		return StorageChildren{}, fmt.Errorf("storage scan result expired")
	}
	result := session.Result
	s.mu.Unlock()
	node := result.Nodes[nodeID]
	if node == nil {
		return StorageChildren{}, fmt.Errorf("storage node not found")
	}
	children := node.Children
	truncated := false
	if len(children) > maxStorageNodesPerResponse {
		children = children[:maxStorageNodesPerResponse]
		truncated = true
	}
	return StorageChildren{
		Parent:    mapStorageNode(node),
		Children:  mapStorageNodes(children, maxStorageNodesPerResponse),
		Truncated: truncated,
	}, nil
}

// GetStorageRoot returns the display metadata for an authorised root only.
func (s *StorageService) GetStorageRoot(rootID string) (StorageRoot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	root := s.roots[rootID]
	if root == nil || time.Since(root.CreatedAt) > storageRootLifetime {
		if root != nil {
			delete(s.roots, rootID)
		}
		return StorageRoot{}, fmt.Errorf("storage root authorisation expired")
	}
	return StorageRoot{ID: root.ID, Label: root.Label, DisplayPath: root.DisplayPath, WholeDrive: root.WholeDrive, CreatedAt: root.CreatedAt}, nil
}
