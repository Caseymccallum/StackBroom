package main

import "time"

// The desktop bridge exposes these DTOs instead of model.Snapshot. They are
// deliberately narrower: no command lines, executable paths, project roots or
// arbitrary system APIs cross into the webview.

type AppInfo struct {
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	SchemaVersion  string   `json:"schemaVersion"`
	Toolchain      string   `json:"toolchain"`
	ReadOnly       bool     `json:"readOnly"`
	Privacy        string   `json:"privacy"`
	Limitations    []string `json:"limitations"`
	Preview        bool     `json:"preview,omitempty"`
	PreviewTheme   string   `json:"previewTheme,omitempty"`
	PreviewScreen  string   `json:"previewScreen,omitempty"`
	PreviewDetail  bool     `json:"previewDetail,omitempty"`
	PreviewConfirm bool     `json:"previewConfirm,omitempty"`
}

type QuickResult struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	Mode        string    `json:"mode"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
	DurationMS  int64     `json:"durationMs"`
	Partial     bool      `json:"partial"`
	Overview    Overview  `json:"overview"`
	Warnings    []string  `json:"warnings"`
}

type FullResult struct {
	ID            string        `json:"id"`
	Status        string        `json:"status"`
	Mode          string        `json:"mode"`
	StartedAt     time.Time     `json:"startedAt"`
	CompletedAt   time.Time     `json:"completedAt"`
	DurationMS    int64         `json:"durationMs"`
	Partial       bool          `json:"partial"`
	Overview      Overview      `json:"overview"`
	Processes     []ProcessRow  `json:"processes"`
	Developer     Developer     `json:"developer"`
	Opportunities []Opportunity `json:"opportunities"`
	Warnings      []string      `json:"warnings"`
}

type Overview struct {
	Hostname             string       `json:"hostname"`
	WindowsVersion       string       `json:"windowsVersion"`
	BuildNumber          uint32       `json:"buildNumber"`
	LogicalProcessors    int          `json:"logicalProcessors"`
	TotalMemoryBytes     uint64       `json:"totalMemoryBytes"`
	AvailableMemoryBytes uint64       `json:"availableMemoryBytes"`
	MemoryKnown          bool         `json:"memoryKnown"`
	MemoryUsedPercent    float64      `json:"memoryUsedPercent"`
	CPUPercent           float64      `json:"cpuPercent"`
	CPUKnown             bool         `json:"cpuKnown"`
	ProcessCount         int          `json:"processCount"`
	DevProcessCount      int          `json:"devProcessCount"`
	DevServerCount       int          `json:"devServerCount"`
	TopProcesses         []ProcessRow `json:"topProcesses"`
	Volumes              []VolumeRow  `json:"volumes"`
	CapturedAt           time.Time    `json:"capturedAt"`
	ScanMode             string       `json:"scanMode"`
}

type VolumeRow struct {
	Root        string  `json:"root"`
	Label       string  `json:"label"`
	FileSystem  string  `json:"fileSystem"`
	TotalBytes  uint64  `json:"totalBytes"`
	FreeBytes   uint64  `json:"freeBytes"`
	UsedBytes   uint64  `json:"usedBytes"`
	UsedPercent float64 `json:"usedPercent"`
}

type ProcessRow struct {
	PID               int32  `json:"pid"`
	ParentPID         int32  `json:"parentPid"`
	Name              string `json:"name"`
	Runtime           string `json:"runtime"`
	Category          string `json:"category"`
	DevServerKind     string `json:"devServerKind,omitempty"`
	MemoryBytes       uint64 `json:"memoryBytes"`
	MemoryKnown       bool   `json:"memoryKnown"`
	StartedAt         string `json:"startedAt,omitempty"`
	StartTimeKnown    bool   `json:"startTimeKnown"`
	ProjectName       string `json:"projectName,omitempty"`
	ProjectConfidence string `json:"projectConfidence,omitempty"`
	RequestedPorts    []int  `json:"requestedPorts,omitempty"`
	ListeningPorts    []int  `json:"listeningPorts,omitempty"`
	PortOwnership     string `json:"portOwnership,omitempty"`
	TargetToken       string `json:"targetToken,omitempty"`
}

type Developer struct {
	Docker     DockerView     `json:"docker"`
	WSL        WSLView        `json:"wsl"`
	Listeners  []ListenerRow  `json:"listeners"`
	Duplicates []DuplicateRow `json:"duplicates"`
}
