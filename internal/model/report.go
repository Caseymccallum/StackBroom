package model

import "time"

// RuntimeTotal is the aggregate of development processes for one runtime.
type RuntimeTotal struct {
	Runtime      Runtime `json:"runtime"`
	ProcessCount int     `json:"processCount"`
	MemoryBytes  uint64  `json:"memoryBytes"`
	// MemoryKnownCount is the number of processes whose memory could be read.
	// When it is lower than ProcessCount the total is a lower bound.
	MemoryKnownCount int `json:"memoryKnownCount"`
}

// TopProcess is one entry in the "what is using my RAM" list.
type TopProcess struct {
	PID         int32   `json:"pid"`
	Name        string  `json:"name"`
	MemoryBytes uint64  `json:"memoryBytes"`
	MemoryKnown bool    `json:"memoryKnown"`
	Category    string  `json:"category"`
	ProjectName string  `json:"projectName,omitempty"`
	Runtime     Runtime `json:"runtime,omitempty"`
}

// Summary aggregates observations for quick reading. Every byte figure carries
// an explicit "known" flag rather than defaulting to zero.
type Summary struct {
	TotalProcesses  int    `json:"totalProcesses"`
	DevProcessCount int    `json:"devProcessCount"`
	DevServerCount  int    `json:"devServerCount"`
	DevMemoryBytes  uint64 `json:"devMemoryBytes"`
	DevMemoryKnown  int    `json:"devMemoryKnownCount"`

	TopProcesses   []TopProcess   `json:"topProcesses"`
	TopMemoryBytes uint64         `json:"topMemoryBytes"`
	RuntimeTotals  []RuntimeTotal `json:"runtimeTotals"`

	DockerRunning int `json:"dockerRunningContainers"`
	DockerStopped int `json:"dockerStoppedContainers"`

	DockerTotalBytes       uint64 `json:"dockerTotalBytes"`
	DockerTotalKnown       bool   `json:"dockerTotalKnown"`
	DockerReclaimableBytes uint64 `json:"dockerReclaimableBytes"`
	DockerReclaimKnown     bool   `json:"dockerReclaimableKnown"`
	// Row counts make partial Docker storage reports explicit instead of
	// silently presenting a partial sum as a total. Size and reclaimable
	// completeness are tracked separately because Docker can report one without
	// the other.
	DockerSizeRowsKnown    int `json:"dockerSizeRowsKnown"`
	DockerSizeRowsTotal    int `json:"dockerSizeRowsTotal"`
	DockerReclaimRowsKnown int `json:"dockerReclaimRowsKnown"`
	DockerReclaimRowsTotal int `json:"dockerReclaimRowsTotal"`

	WSLRunningDistributions int    `json:"wslRunningDistributions"`
	WSLVMMemoryBytes        uint64 `json:"wslVmMemoryBytes"`
	WSLVMMemoryKnown        bool   `json:"wslVmMemoryKnown"`
}

// DuplicateKind distinguishes how a potential duplicate was identified.
const (
	DuplicateByPort    = "port"
	DuplicateByProject = "project"
)

// DuplicateGroup is a set of development server processes that appear to serve
// the same purpose. A group is always a prompt to check, never a verdict.
type DuplicateGroup struct {
	Kind        string     `json:"kind"`
	Key         string     `json:"key"`
	Runtime     Runtime    `json:"runtime"`
	Port        int        `json:"port,omitempty"`
	ProjectRoot string     `json:"projectRoot,omitempty"`
	PIDs        []int32    `json:"pids"`
	MemoryBytes uint64     `json:"memoryBytes"`
	MemoryKnown int        `json:"memoryKnownCount"`
	Confidence  Confidence `json:"confidence"`
	Evidence    []string   `json:"evidence"`
}

// Risk levels for recommendations. Nothing above Low is ever acted upon by
// StackBroom; the level exists to guide the user's own review.
type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

// Subject identifies something a recommendation refers to, so the user can find
// it without guessing.
type Subject struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Detail string `json:"detail,omitempty"`
}

// Recommendation is a read-only suggestion. It carries the six pieces of
// information StackBroom promises: what was detected, why it might be
// unnecessary, the supporting evidence, known or estimated resource use, what to
// review, and the risk of any clean-up.
type Recommendation struct {
	ID              string     `json:"id"`
	Kind            string     `json:"kind"`
	Title           string     `json:"title"`
	Detected        string     `json:"detected"`
	WhyUnnecessary  string     `json:"whyItMightBeUnnecessary"`
	Evidence        []string   `json:"evidence"`
	Resource        string     `json:"resource"`
	ResourceBytes   uint64     `json:"resourceBytes,omitempty"`
	ResourceUnknown bool       `json:"resourceUnknown"`
	Review          []string   `json:"reviewBeforeActing"`
	Risk            Risk       `json:"risk"`
	RiskDetail      string     `json:"riskDetail"`
	Confidence      Confidence `json:"confidence"`
	Subjects        []Subject  `json:"subjects"`
}

// RedactionInfo tells consumers exactly how the output was reduced. It is
// always present in CLI output so a reader cannot mistake omitted data for
// missing data.
type RedactionInfo struct {
	// Mode is "default" or "diagnostics".
	Mode string `json:"mode"`
	// CommandLines is "omitted" or "redacted".
	CommandLines string `json:"commandLines"`
	// Paths is "user-profile-tokenised".
	Paths string `json:"paths"`
	// Secrets is "masked".
	Secrets string `json:"secrets"`
}

// Timings records how long each stage of a scan took. It exists so performance
// claims can be measured rather than guessed; values are zero when a stage was
// skipped or a fake clock was injected.
type Timings struct {
	ProcessesMS int64 `json:"processesMs"`
	DockerMS    int64 `json:"dockerMs"`
	WSLMS       int64 `json:"wslMs"`
	AnalysisMS  int64 `json:"analysisMs"`
}

// Snapshot is the complete result of one StackBroom scan.
type Snapshot struct {
	SchemaVersion   string           `json:"schemaVersion"`
	ReadOnly        bool             `json:"readOnly"`
	Redaction       *RedactionInfo   `json:"redaction,omitempty"`
	Tool            Tool             `json:"tool"`
	GeneratedAt     time.Time        `json:"generatedAt"`
	DurationMS      int64            `json:"durationMs"`
	Timings         *Timings         `json:"timings,omitempty"`
	Host            Host             `json:"host"`
	Processes       []Process        `json:"processes"`
	Docker          DockerInfo       `json:"docker"`
	WSL             WSLInfo          `json:"wsl"`
	Summary         Summary          `json:"summary"`
	Duplicates      []DuplicateGroup `json:"duplicateSuspects"`
	Recommendations []Recommendation `json:"recommendations"`
	Warnings        []string         `json:"warnings"`
}

// PlanDocument is the JSON shape emitted by `stackbroom plan --json`. It is
// deliberately narrower than Snapshot: a plan contains no process table.
type PlanDocument struct {
	SchemaVersion   string           `json:"schemaVersion"`
	ReadOnly        bool             `json:"readOnly"`
	Redaction       *RedactionInfo   `json:"redaction,omitempty"`
	Executes        bool             `json:"executesCleanup"`
	Tool            Tool             `json:"tool"`
	GeneratedAt     time.Time        `json:"generatedAt"`
	Summary         Summary          `json:"summary"`
	Recommendations []Recommendation `json:"recommendations"`
	Warnings        []string         `json:"warnings"`
}
