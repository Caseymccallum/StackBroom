package winproc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// Enrichment is the extra information PowerShell's CIM provider can supply for
// a process. Fields are optional: CIM may refuse access to other users'
// processes.
type Enrichment struct {
	CommandLine string
	StartedAt   *time.Time
	// MemoryBytes is CIM's WorkingSetSize. It is used only when the direct
	// Windows API could not read the process (for example vmmem).
	MemoryBytes uint64
	MemoryKnown bool
}

type cimProcess struct {
	ProcessID       *int64    `json:"ProcessId"`
	ParentProcessID *int64    `json:"ParentProcessId"`
	CommandLine     *string   `json:"CommandLine"`
	CreationDate    *flexTime `json:"CreationDate"`
	WorkingSetSize  *int64    `json:"WorkingSetSize"`
}

// cimScript is a fixed, read-only WMI query. It is passed as a single argument
// to powershell.exe; no observed system data is interpolated into it, so
// process names and paths can never become script content.
const cimScript = `$ErrorActionPreference='Stop';` +
	`[Console]::OutputEncoding=[System.Text.Encoding]::UTF8;` +
	`Get-CimInstance -ClassName Win32_Process | ` +
	`Select-Object ProcessId,ParentProcessId,CommandLine,CreationDate,WorkingSetSize | ` +
	`ConvertTo-Json -Compress -Depth 2`

// flexTime accepts the several date shapes PowerShell may emit for a CIM
// DateTime: ISO 8601, a legacy "/Date(ms)/" wrapper, a US-style string, or an
// epoch millisecond number. An unrecognised shape leaves the value unknown
// instead of guessing.
type flexTime struct {
	Time  time.Time
	Valid bool
}

func (f *flexTime) UnmarshalJSON(b []byte) error {
	raw := strings.TrimSpace(string(b))
	if raw == "null" || raw == `""` {
		return nil
	}
	raw = strings.Trim(raw, `"`)
	// PowerShell 5.1 escapes forward slashes ("\/Date(...)\/"), so the raw
	// JSON text must be normalised before the /Date(...)/ branch can match.
	raw = strings.ReplaceAll(raw, `\/`, `/`)

	if strings.HasPrefix(raw, "/Date(") && strings.HasSuffix(raw, ")/") {
		inner := strings.TrimSuffix(strings.TrimPrefix(raw, "/Date("), ")/")
		// Optional timezone suffix, e.g. /Date(1698000000000+0100)/.
		if idx := strings.IndexAny(inner, "+-"); idx > 0 {
			inner = inner[:idx]
		}
		if ms, err := strconv.ParseInt(inner, 10, 64); err == nil {
			f.Time = time.UnixMilli(ms)
			f.Valid = true
		}
		return nil
	}

	if raw != "" && (raw[0] == '-' || (raw[0] >= '0' && raw[0] <= '9')) {
		if ms, err := strconv.ParseInt(raw, 10, 64); err == nil && ms > 100000000000 {
			f.Time = time.UnixMilli(ms)
			f.Valid = true
			return nil
		}
		// Not a bare epoch number; fall through to the string layouts below
		// (an ISO timestamp also starts with a digit).
	}

	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.9999999",
		"01/02/2006 15:04:05",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			f.Time = t
			f.Valid = true
			return nil
		}
	}
	return nil
}

// ParseCIMProcesses reads the JSON produced by
// `Get-CimInstance Win32_Process | Select ... | ConvertTo-Json`.
//
// ConvertTo-Json emits an object rather than an array when only one process is
// returned, and PowerShell may prefix output with a BOM, so both cases are
// handled. Rows without a usable process ID are skipped and reported.
func ParseCIMProcesses(data []byte) (map[int32]Enrichment, []string, error) {
	out := map[int32]Enrichment{}
	text := strings.TrimSpace(string(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})))
	text = strings.Trim(text, "\x00")
	if text == "" {
		return out, nil, fmt.Errorf("PowerShell returned no process data")
	}

	var rows []cimProcess
	if strings.HasPrefix(text, "[") {
		if err := json.Unmarshal([]byte(text), &rows); err != nil {
			return out, nil, fmt.Errorf("could not parse process JSON: %w", err)
		}
	} else {
		var one cimProcess
		if err := json.Unmarshal([]byte(text), &one); err != nil {
			return out, nil, fmt.Errorf("could not parse process JSON: %w", err)
		}
		rows = []cimProcess{one}
	}

	var warnings []string
	for _, row := range rows {
		if row.ProcessID == nil || *row.ProcessID <= 0 {
			continue
		}
		pid := int32(*row.ProcessID)
		e := Enrichment{}
		if row.CommandLine != nil {
			e.CommandLine = strings.TrimSpace(*row.CommandLine)
		}
		if row.CreationDate != nil && row.CreationDate.Valid {
			t := row.CreationDate.Time
			e.StartedAt = &t
		}
		if row.WorkingSetSize != nil && *row.WorkingSetSize > 0 {
			e.MemoryBytes = uint64(*row.WorkingSetSize)
			e.MemoryKnown = true
		}
		out[pid] = e
	}
	if len(rows) > 0 && len(out) == 0 {
		warnings = append(warnings, "PowerShell returned process records without usable process IDs")
	}
	return out, warnings, nil
}

// ApplyEnrichment merges command lines and start times into collected
// processes. Processes missing from the map keep CommandLineKnown=false.
func ApplyEnrichment(procs []model.Process, enrich map[int32]Enrichment) {
	for i := range procs {
		e, ok := enrich[procs[i].PID]
		if !ok {
			continue
		}
		if e.CommandLine != "" {
			procs[i].CommandLine = e.CommandLine
			procs[i].CommandLineKnown = true
		}
		if e.StartedAt != nil {
			procs[i].StartedAt = e.StartedAt
			procs[i].StartTimeKnown = true
		}
		// Only fill memory that the direct Windows API could not read. The API
		// value is preferred because CIM's WorkingSetSize is sampled separately.
		if !procs[i].MemoryKnown && e.MemoryKnown {
			procs[i].WorkingSetBytes = e.MemoryBytes
			procs[i].MemoryKnown = true
		}
	}
}
