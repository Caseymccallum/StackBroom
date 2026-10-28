package processcontrol

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// VerifyProcessIdentity verifies that a PID corresponds to the expected process
// and returns detailed identity information.
func VerifyProcessIdentity(pid int32, expectedCreateTime time.Time) (ProcessIdentity, error) {
	handle, err := openProcess(pid, PROCESS_QUERY_LIMITED_INFORMATION|PROCESS_QUERY_INFORMATION)
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("cannot open process: %w", err)
	}
	defer closeHandle(handle)

	createTime, err := getProcessTimes(handle)
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("cannot get process times: %w", err)
	}

	// PID reuse detection: verify creation time matches
	if !createTime.Equal(expectedCreateTime) {
		return ProcessIdentity{}, fmt.Errorf("PID reuse detected: expected %v, got %v", expectedCreateTime, createTime)
	}

	exePath, err := getProcessExePath(handle)
	if err != nil {
		// Some processes may not expose their path; continue with empty
		exePath = ""
	}

	isCritical, err := isProcessCritical(handle)
	if err != nil {
		// Fail closed: if we can't determine critical status, treat as critical
		// This prevents accidentally terminating critical system processes
		isCritical = true
	}

	name := filepath.Base(exePath)

	return ProcessIdentity{
		PID:         pid,
		Name:        name,
		ExePath:     exePath,
		CreateTime:  createTime,
		IsCritical:  isCritical,
		IsProtected: isProtectedProcess(name),
	}, nil
}

// isProtectedProcess checks if a process name is in the protected list
func isProtectedProcess(name string) bool {
	protected := []string{
		"System Idle Process",
		"System",
		"smss.exe",
		"csrss.exe",
		"wininit.exe",
		"winlogon.exe",
		"lsass.exe",
		"svchost.exe",
		"services.exe",
		"spoolsv.exe",
		"MsMpEng.exe",      // Windows Defender
		"SecurityHealthService.exe",
	}

	nameLower := strings.ToLower(name)
	for _, protected := range protected {
		if nameLower == strings.ToLower(protected) {
			return true
		}
	}
	return false
}

// IsStackBroomProcess checks if a process is StackBroom or its children
func IsStackBroomProcess(name string) bool {
	nameLower := strings.ToLower(name)
	return strings.Contains(nameLower, "stackbroom")
}
