// Package model holds the data types shared by StackBroom's collectors, the
// analysis layer and the presentation layer.
//
// Everything in this package is observation data only. Nothing here is capable
// of changing the state of the machine.
package model

import "time"

// SchemaVersion identifies the JSON output contract. Bump it when a field is
// removed, renamed or changed in meaning.
//
// Version 2 (v0.1.1) omits command lines by default and adds explicit redaction
// metadata. Consumers must not assume commandLine is present.
const SchemaVersion = "2"

// Confidence describes how strongly an association is supported by evidence.
//
// The three levels are deliberately conservative:
//   - Confirmed: two independent, verifiable signals agree (for example a
//     project manifest exists on disk and the process references a file inside
//     that project).
//   - Inferred: one plausible signal exists but it could not be fully verified.
//   - Unknown: no defensible evidence was found.
type Confidence string

const (
	Confirmed Confidence = "confirmed"
	Inferred  Confidence = "inferred"
	Unknown   Confidence = "unknown"
)

// Runtime is the development runtime a process was recognised as.
type Runtime string

const (
	RuntimeNode    Runtime = "node"
	RuntimeGo      Runtime = "go"
	RuntimePython  Runtime = "python"
	RuntimeDotnet  Runtime = "dotnet"
	RuntimeJava    Runtime = "java"
	RuntimeRuby    Runtime = "ruby"
	RuntimePHP     Runtime = "php"
	RuntimeDeno    Runtime = "deno"
	RuntimeBun     Runtime = "bun"
	RuntimeElixir  Runtime = "elixir"
	RuntimeRust    Runtime = "rust"
	RuntimeUnknown Runtime = "unknown"
)

// Process categories. CategoryDevServer is only assigned when the command line
// contains a recognisable development-server invocation; a plain runtime
// process is never promoted to "dev server" on age or memory alone.
const (
	CategoryDevServer = "dev-server"
	CategoryDevTool   = "dev-tool"
	CategoryRuntime   = "runtime"
	CategoryOther     = "other"
)

// ProjectRef is a project association for a process or container.
type ProjectRef struct {
	Root       string     `json:"root"`
	Name       string     `json:"name"`
	Marker     string     `json:"marker,omitempty"`
	Confidence Confidence `json:"confidence"`
	Evidence   []string   `json:"evidence"`
}

// Process is a single observed Windows process.
type Process struct {
	PID        int32  `json:"pid"`
	ParentPID  int32  `json:"parentPid"`
	Name       string `json:"name"`
	ParentName string `json:"parentName,omitempty"`
	ExePath    string `json:"exePath,omitempty"`

	CommandLine      string `json:"commandLine,omitempty"`
	CommandLineKnown bool   `json:"commandLineKnown"`
	// CommandLineOmitted is true when a command line was observed but withheld
	// from output by the privacy policy.
	CommandLineOmitted bool `json:"commandLineOmitted,omitempty"`

	// WorkingSetBytes is the process working set. Zero with MemoryKnown=false
	// means the measurement was refused or unavailable; it is never estimated.
	WorkingSetBytes uint64 `json:"workingSetBytes"`
	MemoryKnown     bool   `json:"memoryKnown"`

	StartedAt      *time.Time `json:"startedAt,omitempty"`
	StartTimeKnown bool       `json:"startTimeKnown"`

	Runtime       Runtime `json:"runtime"`
	Category      string  `json:"category"`
	DevServerKind string  `json:"devServerKind,omitempty"`
	Ports         []int   `json:"ports,omitempty"`

	Project *ProjectRef `json:"project,omitempty"`
}

// IsDevelopment reports whether the process is a recognised development
// process (server or tool). Plain runtime processes are not counted here.
func (p Process) IsDevelopment() bool {
	return p.Category == CategoryDevServer || p.Category == CategoryDevTool
}

// Host describes the machine and its physical memory.
type Host struct {
	OS                   string `json:"os"`
	Arch                 string `json:"arch"`
	Hostname             string `json:"hostname,omitempty"`
	TotalMemoryBytes     uint64 `json:"totalMemoryBytes"`
	AvailableMemoryBytes uint64 `json:"availableMemoryBytes"`
	MemoryKnown          bool   `json:"memoryKnown"`
}

// Tool identifies the producing build.
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
