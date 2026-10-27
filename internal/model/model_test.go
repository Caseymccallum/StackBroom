package model

import "testing"

func TestProcessIsDevelopment(t *testing.T) {
	tests := map[string]bool{
		CategoryDevServer: true,
		CategoryDevTool:   true,
		CategoryRuntime:   false,
		CategoryOther:     false,
	}
	for category, want := range tests {
		p := Process{Category: category}
		if got := p.IsDevelopment(); got != want {
			t.Errorf("IsDevelopment() for %q = %v, want %v", category, got, want)
		}
	}
}

func TestDockerContainerRunning(t *testing.T) {
	if !(DockerContainer{State: "running"}).Running() {
		t.Error("running container not recognised")
	}
	for _, state := range []string{"exited", "created", "paused", ""} {
		if (DockerContainer{State: state}).Running() {
			t.Errorf("state %q should not count as running", state)
		}
	}
}

func TestDockerInfoCountsAndLookup(t *testing.T) {
	info := DockerInfo{
		Containers: []DockerContainer{
			{Name: "web", State: "running"},
			{Name: "db", State: "exited"},
			{Name: "cache", State: "created"},
		},
		DiskUsage: []DockerDiskUsage{
			{Type: "Images"},
			{Type: "Build Cache"},
		},
	}

	if got := info.RunningContainers(); got != 1 {
		t.Errorf("RunningContainers() = %d, want 1", got)
	}
	if got := info.StoppedContainers(); got != 2 {
		t.Errorf("StoppedContainers() = %d, want 2", got)
	}
	if row, ok := info.DiskUsageOf("build cache"); !ok || row.Type != "Build Cache" {
		t.Errorf("DiskUsageOf is not case-insensitive: %+v %v", row, ok)
	}
	if _, ok := info.DiskUsageOf("Local Volumes"); ok {
		t.Error("DiskUsageOf reported a row that does not exist")
	}
}

func TestWSLRunningDistributions(t *testing.T) {
	info := WSLInfo{
		Distributions: []WSLDistribution{
			{Name: "Ubuntu", State: "Running"},
			{Name: "Debian", State: "Stopped"},
			{Name: "Other", State: "running"},
		},
	}
	if got := info.RunningDistributions(); got != 2 {
		t.Errorf("RunningDistributions() = %d, want 2", got)
	}
}
