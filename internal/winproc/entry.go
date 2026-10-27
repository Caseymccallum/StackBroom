package winproc

import (
	"encoding/binary"

	"github.com/Caseymccallum/StackBroom/internal/wintext"
)

// Layout of the Windows PROCESSENTRY32W structure on 64-bit Windows.
//
//	dwSize              0
//	cntUsage            4
//	th32ProcessID       8
//	th32DefaultHeapID  16 (pointer)
//	th32ModuleID       24
//	cntThreads         28
//	th32ParentProcessID 32
//	pcPriClassBase     36
//	dwFlags            40
//	szExeFile          44 (WCHAR[MAX_PATH])
const (
	processEntry32Size = 568
	exeNameOffset      = 44
	maxPathChars       = 260
	pidOffset          = 8
	parentPIDOffset    = 32
)

// Entry is the minimal process record read from a Toolhelp32 snapshot.
type Entry struct {
	PID       int32
	ParentPID int32
	Name      string
}

// decodeProcessEntry32 reads one PROCESSENTRY32W buffer.
//
// It is a pure function so that parsing can be tested with deterministic byte
// fixtures rather than by inspecting live processes.
func decodeProcessEntry32(raw []byte) (Entry, bool) {
	if len(raw) < processEntry32Size {
		return Entry{}, false
	}
	pid := int32(binary.LittleEndian.Uint32(raw[pidOffset : pidOffset+4]))
	if pid <= 0 {
		return Entry{}, false
	}
	parent := int32(binary.LittleEndian.Uint32(raw[parentPIDOffset : parentPIDOffset+4]))
	name := wintext.DecodeUTF16LE(raw[exeNameOffset : exeNameOffset+maxPathChars*2])
	if name == "" {
		return Entry{}, false
	}
	return Entry{PID: pid, ParentPID: parent, Name: name}, true
}
