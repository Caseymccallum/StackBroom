package winproc

import (
	"strings"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func TestParseCIMProcessesHandlesPowerShellShapes(t *testing.T) {
	data := []byte(`[
		{"ProcessId":100,"ParentProcessId":4,"CommandLine":"node C:\\proj\\server.js","CreationDate":"\/Date(1793115510041)\/","WorkingSetSize":62042112},
		{"ProcessId":101,"ParentProcessId":100,"CommandLine":null,"CreationDate":"2026-10-27T10:00:00.0000000+01:00"},
		{"ProcessId":0,"CommandLine":"ignored"},
		{"ParentProcessId":5}
	]`)

	enrich, warnings, err := ParseCIMProcesses(data)
	if err != nil {
		t.Fatalf("ParseCIMProcesses returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(enrich) != 2 {
		t.Fatalf("parsed %d processes, want 2", len(enrich))
	}

	first := enrich[100]
	if first.CommandLine != `node C:\proj\server.js` {
		t.Errorf("command line = %q", first.CommandLine)
	}
	if !first.MemoryKnown || first.MemoryBytes != 62042112 {
		t.Errorf("memory = %d known=%v, want 62042112 known", first.MemoryBytes, first.MemoryKnown)
	}
	if first.StartedAt == nil {
		t.Fatal("start time was not parsed from /Date(...)/")
	}
	if want := time.UnixMilli(1793115510041); !first.StartedAt.Equal(want) {
		t.Errorf("start time = %s, want %s", first.StartedAt, want)
	}

	second := enrich[101]
	if second.CommandLine != "" {
		t.Errorf("null command line became %q", second.CommandLine)
	}
	if second.StartedAt == nil || second.StartedAt.Year() != 2026 {
		t.Errorf("ISO start time was not parsed: %+v", second.StartedAt)
	}

	if _, ok := enrich[0]; ok {
		t.Error("process id 0 should have been skipped")
	}
}

func TestParseCIMProcessesSingleObjectAndDateFallback(t *testing.T) {
	data := []byte(`{"ProcessId":7,"CommandLine":"go run .","CreationDate":"10/27/2026 09:15:00"}`)
	enrich, _, err := ParseCIMProcesses(data)
	if err != nil {
		t.Fatalf("ParseCIMProcesses returned error: %v", err)
	}
	got, ok := enrich[7]
	if !ok {
		t.Fatal("single-object response was not parsed")
	}
	if got.CommandLine != "go run ." {
		t.Errorf("command line = %q", got.CommandLine)
	}
	if got.StartedAt == nil || got.StartedAt.Year() != 2026 {
		t.Errorf("US-format date was not parsed: %+v", got.StartedAt)
	}
}

func TestParseCIMProcessesErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"whitespace", "   \n\t "},
		{"truncated", `[{"ProcessId":`},
		{"wrong type", `"not a process list"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := ParseCIMProcesses([]byte(tc.in)); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestParseCIMProcessesStripsBOM(t *testing.T) {
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`[{"ProcessId":1,"CommandLine":"x"}]`)...)
	if _, _, err := ParseCIMProcesses(data); err != nil {
		t.Fatalf("BOM handling failed: %v", err)
	}
}

func TestApplyEnrichment(t *testing.T) {
	started := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	procs := []model.Process{
		{PID: 1},
		{PID: 2, MemoryKnown: true, WorkingSetBytes: 10},
		{PID: 3},
	}
	enrich := map[int32]Enrichment{
		1: {CommandLine: "node server.js", MemoryBytes: 99, MemoryKnown: true},
		2: {MemoryBytes: 50, MemoryKnown: true},
		3: {StartedAt: &started},
	}
	ApplyEnrichment(procs, enrich)

	if procs[0].CommandLine != "node server.js" || !procs[0].CommandLineKnown {
		t.Errorf("command line was not applied: %+v", procs[0])
	}
	if procs[0].WorkingSetBytes != 99 || !procs[0].MemoryKnown {
		t.Errorf("memory fallback not applied: %+v", procs[0])
	}
	if procs[1].WorkingSetBytes != 10 {
		t.Errorf("known memory was overwritten: %+v", procs[1])
	}
	if !procs[2].StartTimeKnown || procs[2].StartedAt == nil {
		t.Errorf("start time was not applied: %+v", procs[2])
	}
}

func TestApplyEnrichmentDoesNotOverwriteNativeStartTime(t *testing.T) {
	native := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	cim := native.Add(time.Hour)
	procs := []model.Process{{PID: 1, StartedAt: &native, StartTimeKnown: true}}
	ApplyEnrichment(procs, map[int32]Enrichment{1: {StartedAt: &cim}})
	if !procs[0].StartedAt.Equal(native) {
		t.Errorf("native start time was overwritten by CIM: got %s want %s", procs[0].StartedAt, native)
	}
}

func TestCIMScriptIsStatic(t *testing.T) {
	// The query must never be assembled from observed data; it is a fixed
	// string with no formatting verbs.
	if strings.Contains(cimScript, "%") {
		t.Fatal("cimScript appears to be formatted at runtime")
	}
	if !strings.Contains(cimScript, "Get-CimInstance") {
		t.Fatal("cimScript does not perform the expected read-only query")
	}
	for _, forbidden := range []string{"Stop-Process", "Remove-", "kill", "Stop-"} {
		if strings.Contains(cimScript, forbidden) {
			t.Fatalf("cimScript contains a mutating verb %q", forbidden)
		}
	}
}
