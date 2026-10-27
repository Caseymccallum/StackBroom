package model

import "strings"

// DockerStatus reports whether the Docker CLI and daemon could be queried.
// Available is false when Docker is absent, its daemon is unreachable, or the
// configured endpoint was refused because it is not a local IPC channel.
type DockerStatus struct {
	Available     bool   `json:"available"`
	Reason        string `json:"reason,omitempty"`
	ServerVersion string `json:"serverVersion,omitempty"`
	Context       string `json:"context,omitempty"`
	// Endpoint is the resolved daemon endpoint with credentials removed.
	Endpoint string `json:"endpoint,omitempty"`
	// EndpointLocal is true only for local IPC channels (npipe, unix, fd).
	EndpointLocal bool `json:"endpointLocal"`
	// RemoteApproved is true when a network endpoint was inspected because the
	// user passed --allow-remote-docker.
	RemoteApproved bool `json:"remoteApproved,omitempty"`
}

// DockerContainer is a container observed through `docker ps -a`.
type DockerContainer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`
	Status string `json:"status"`
	// CreatedAt is the raw Docker timestamp string; layout varies by platform
	// and Docker version, so StackBroom does not depend on parsing it.
	CreatedAt string `json:"createdAt,omitempty"`
	Ports     string `json:"ports,omitempty"`

	ComposeProject    string `json:"composeProject,omitempty"`
	ComposeService    string `json:"composeService,omitempty"`
	ComposeWorkingDir string `json:"composeWorkingDir,omitempty"`

	// Resource usage comes from `docker stats` and is only available for
	// running containers. The strings are Docker's own human-readable values.
	StatsKnown  bool   `json:"statsKnown"`
	CPUPercent  string `json:"cpuPercent,omitempty"`
	MemoryUsage string `json:"memoryUsage,omitempty"`
	MemoryPerc  string `json:"memoryPercent,omitempty"`

	// StoppedForText is Docker's reported age of the last state change, e.g.
	// "23 hours" when Status reads "Exited (0) 23 hours ago". Empty when the
	// age could not be parsed; it is an approximation, not a timestamp.
	StoppedForText string `json:"stoppedForText,omitempty"`
	StoppedForDays int    `json:"stoppedForDays,omitempty"`
}

// Running reports whether Docker considers the container running.
func (c DockerContainer) Running() bool { return c.State == "running" }

// DockerDiskUsage is one row of `docker system df`. Size and Reclaimable are
// nil when Docker's value could not be parsed; no value is invented.
type DockerDiskUsage struct {
	Type     string  `json:"type"`
	Total    int     `json:"total"`
	Active   int     `json:"active"`
	SizeText string  `json:"sizeText,omitempty"`
	Size     *uint64 `json:"size,omitempty"`
	// ReclaimableText is what Docker reported, e.g. "12.41GB (79%)".
	ReclaimableText    string   `json:"reclaimableText,omitempty"`
	Reclaimable        *uint64  `json:"reclaimable,omitempty"`
	ReclaimablePercent *float64 `json:"reclaimablePercent,omitempty"`
}

// DockerImage is one entry from `docker image ls`.
type DockerImage struct {
	ID         string  `json:"id,omitempty"`
	Repository string  `json:"repository"`
	Tag        string  `json:"tag"`
	SizeText   string  `json:"sizeText,omitempty"`
	Size       *uint64 `json:"size,omitempty"`
	CreatedFor string  `json:"createdFor,omitempty"`
	InUse      bool    `json:"inUse"`
}

// DockerInfo is the complete read-only Docker observation.
type DockerInfo struct {
	Status     DockerStatus      `json:"status"`
	Containers []DockerContainer `json:"containers"`
	DiskUsage  []DockerDiskUsage `json:"diskUsage"`
	Images     []DockerImage     `json:"images"`
	// ImagesTruncated reports that the image list was capped for readability.
	ImagesTruncated bool `json:"imagesTruncated,omitempty"`
	// Warnings records Docker responses that could not be fully parsed. The scan
	// continues with whatever was understood.
	Warnings []string `json:"warnings,omitempty"`
}

// RunningContainers counts containers in the running state.
func (d DockerInfo) RunningContainers() int {
	n := 0
	for _, c := range d.Containers {
		if c.Running() {
			n++
		}
	}
	return n
}

// StoppedContainers counts containers that exist but are not running.
func (d DockerInfo) StoppedContainers() int { return len(d.Containers) - d.RunningContainers() }

// DiskUsageOf returns the row with the given Docker type name ("Images",
// "Containers", "Local Volumes", "Build Cache").
func (d DockerInfo) DiskUsageOf(kind string) (DockerDiskUsage, bool) {
	for _, row := range d.DiskUsage {
		if strings.EqualFold(row.Type, kind) {
			return row, true
		}
	}
	return DockerDiskUsage{}, false
}
