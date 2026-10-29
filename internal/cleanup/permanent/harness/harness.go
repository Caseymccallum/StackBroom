//go:build windows && m3c2_fixture

// Package harness prepares an isolated disposable fixture for the first
// owner-authorised native cleanup test. It never calls the deletion worker.
package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
	"github.com/Caseymccallum/StackBroom/internal/storage"
)

const (
	EnvHarnessAuthorized = "STACKBROOM_M3C2_HARNESS_AUTHORIZED"
	EnvLiveAuthorized    = "STACKBROOM_M3C2_LIVE_DELETE_AUTHORIZED"
)

var (
	ErrNotAuthorized     = errors.New("M3C2 fixture harness is not authorised")
	ErrLiveNotAuthorized = errors.New("M3C2 live deletion is not authorised")
)

type Fixture struct {
	Root      string
	Target    execution.Target
	Contents  []byte
	Digest    string
	Sentinels map[string]string
	lock      syscall.Handle
}

var (
	kernel32Harness   = syscall.NewLazyDLL("kernel32.dll")
	procVolumeHarness = kernel32Harness.NewProc("GetVolumeInformationW")
)

func Prepare() (*Fixture, error) {
	if os.Getenv(EnvHarnessAuthorized) != "1" {
		return nil, ErrNotAuthorized
	}
	root, err := os.MkdirTemp("", "StackBroom-M3C2-")
	if err != nil {
		return nil, fmt.Errorf("create fixture root: %w", err)
	}
	root, err = storage.ValidateRoot(root)
	if err != nil {
		return nil, fmt.Errorf("validate fixture root: %w", err)
	}
	if !fixtureRootValid(root) {
		return nil, fmt.Errorf("fixture root is not an isolated temporary M3C2 directory")
	}
	if !isNTFS(root) {
		return nil, fmt.Errorf("fixture root is not on NTFS")
	}
	contents := make([]byte, 32768)
	for index := range contents {
		contents[index] = byte((index*31 + 7) % 251)
	}
	targetPath := filepath.Join(root, "m3c2-single.dmp")
	if err := os.WriteFile(targetPath, contents, 0o600); err != nil {
		return nil, err
	}
	when := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(targetPath, when, when); err != nil {
		return nil, err
	}
	sentinels := map[string]string{
		"keep-a.txt": "sentinel-a",
		"keep-b.txt": "sentinel-b",
	}
	for name, body := range sentinels {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			return nil, err
		}
	}
	request := storage.DefaultRequest(root)
	request.MinDuplicateSize = ^uint64(0)
	scan, err := storage.NewScanner().Scan(context.Background(), request, nil)
	if err != nil {
		return nil, err
	}
	var record *storage.FileRecord
	for _, file := range scan.Files {
		if strings.EqualFold(file.Name, "m3c2-single.dmp") {
			record = file
			break
		}
	}
	if record == nil || record.FileID == "" {
		return nil, fmt.Errorf("fixture target identity was not captured")
	}
	lock, err := lockDirectory(root)
	if err != nil {
		return nil, fmt.Errorf("lock fixture root: %w", err)
	}
	return &Fixture{
		Root: root, Target: execution.Target{ID: "m3c2-single", File: *record},
		Contents: contents, Digest: digest(contents), Sentinels: sentinelDigests(sentinels), lock: lock,
	}, nil
}

func (f *Fixture) VerifyInitialState() error {
	if f == nil || f.Root == "" || f.Target.ID == "" {
		return fmt.Errorf("fixture is not initialized")
	}
	abs, err := filepath.Abs(f.Root)
	if err != nil || !strings.EqualFold(abs, f.Root) {
		return fmt.Errorf("fixture root identity changed")
	}
	if !fixtureRootValid(f.Root) {
		return fmt.Errorf("fixture root escaped the isolated temporary location")
	}
	if !isNTFS(f.Root) {
		return fmt.Errorf("fixture root is not NTFS")
	}
	contents, err := os.ReadFile(f.Target.File.Path)
	if err != nil {
		return err
	}
	if len(contents) != len(f.Contents) || digest(contents) != f.Digest {
		return fmt.Errorf("fixture target contents changed")
	}
	if err := storage.ValidateCleanupTarget(&f.Target.File); err != nil {
		return fmt.Errorf("fixture target identity changed: %w", err)
	}
	if err := f.VerifySentinels(); err != nil {
		return err
	}
	entries, err := os.ReadDir(f.Root)
	if err != nil {
		return err
	}
	if len(entries) != 3 {
		return fmt.Errorf("fixture root contains unexpected entries: %d", len(entries))
	}
	return nil
}

func (f *Fixture) VerifySentinels() error {
	if f == nil || f.Root == "" {
		return fmt.Errorf("fixture is not initialized")
	}
	for name, expected := range f.Sentinels {
		path := filepath.Join(f.Root, name)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("sentinel %s is no longer a regular file", name)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if digest(body) != expected {
			return fmt.Errorf("sentinel %s changed", name)
		}
	}
	return nil
}

func (f *Fixture) LiveAuthorized() error {
	if f == nil {
		return ErrLiveNotAuthorized
	}
	if os.Getenv(EnvHarnessAuthorized) != "1" || os.Getenv(EnvLiveAuthorized) != "1" {
		return ErrLiveNotAuthorized
	}
	return nil
}

func (f *Fixture) ReleaseLock() error {
	if f == nil || f.lock == 0 {
		return nil
	}
	err := syscall.CloseHandle(f.lock)
	f.lock = 0
	return err
}

func (f *Fixture) Close() error { return f.ReleaseLock() }

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sentinelDigests(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for name, body := range values {
		out[name] = digest([]byte(body))
	}
	return out
}

func lockDirectory(root string) (syscall.Handle, error) {
	path, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return 0, err
	}
	const fileListDirectory = 0x0001
	const fileReadAttributes = 0x0080
	const openExisting = 3
	const fileFlagBackupSemantics = 0x02000000
	const fileFlagOpenReparsePoint = 0x00200000
	const fileShareRead = 0x00000001
	return syscall.CreateFile(path, fileListDirectory|fileReadAttributes, fileShareRead, nil, openExisting,
		fileFlagBackupSemantics|fileFlagOpenReparsePoint, 0)
}

func fixtureRootValid(root string) bool {
	temp, err := filepath.Abs(os.TempDir())
	if err != nil {
		return false
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	resolvedTemp, err := filepath.EvalSymlinks(temp)
	if err != nil {
		return false
	}
	tempName := strings.ToLower(filepath.Base(resolvedTemp))
	if tempName != "temp" && tempName != "tmp" {
		return false
	}
	relative, err := filepath.Rel(resolvedTemp, resolvedRoot)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	lower := strings.ToLower(resolvedRoot)
	for _, sensitive := range []string{`\documents\`, `\desktop\`, `\downloads\`, `\pictures\`, `\videos\`, `\music\`, `\crashdumps\`} {
		if strings.Contains(lower, sensitive) {
			return false
		}
	}
	return strings.HasPrefix(strings.ToLower(filepath.Base(resolvedRoot)), "stackbroom-m3c2-")
}

func isNTFS(path string) bool {
	volume := filepath.VolumeName(path)
	if len(volume) != 2 || volume[1] != ':' {
		return false
	}
	root, err := syscall.UTF16PtrFromString(volume + string(filepath.Separator))
	if err != nil {
		return false
	}
	var fileSystem [64]uint16
	r, _, _ := procVolumeHarness.Call(
		uintptr(unsafe.Pointer(root)), 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&fileSystem[0])), uintptr(len(fileSystem)),
	)
	if r == 0 {
		return false
	}
	return strings.EqualFold(syscall.UTF16ToString(fileSystem[:]), "NTFS")
}
