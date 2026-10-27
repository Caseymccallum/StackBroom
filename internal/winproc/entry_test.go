package winproc

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

// processEntryFixture builds a PROCESSENTRY32W buffer for tests.
func processEntryFixture(pid, parent int32, name string) []byte {
	raw := make([]byte, processEntry32Size)
	binary.LittleEndian.PutUint32(raw[0:], processEntry32Size)
	binary.LittleEndian.PutUint32(raw[pidOffset:], uint32(pid))
	binary.LittleEndian.PutUint32(raw[parentPIDOffset:], uint32(parent))

	units := utf16.Encode([]rune(name))
	for i, u := range units {
		if i >= maxPathChars-1 {
			break
		}
		binary.LittleEndian.PutUint16(raw[exeNameOffset+i*2:], u)
	}
	return raw
}

func TestDecodeProcessEntry32(t *testing.T) {
	raw := processEntryFixture(4321, 987, "node.exe")
	entry, ok := decodeProcessEntry32(raw)
	if !ok {
		t.Fatal("decodeProcessEntry32 rejected a valid entry")
	}
	if entry.PID != 4321 {
		t.Errorf("PID = %d, want 4321", entry.PID)
	}
	if entry.ParentPID != 987 {
		t.Errorf("ParentPID = %d, want 987", entry.ParentPID)
	}
	if entry.Name != "node.exe" {
		t.Errorf("Name = %q, want node.exe", entry.Name)
	}
}

func TestDecodeProcessEntry32RejectsInvalidBuffers(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
	}{
		{"too short", make([]byte, processEntry32Size-1)},
		{"zero pid", processEntryFixture(0, 1, "system")},
		{"empty name", processEntryFixture(5, 1, "")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := decodeProcessEntry32(tc.raw); ok {
				t.Fatal("expected the buffer to be rejected")
			}
		})
	}
}
