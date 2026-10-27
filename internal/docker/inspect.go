package docker

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/run"
)

// DefaultMaxImages caps the image list so JSON and terminal output stay
// readable. The largest images are kept.
const DefaultMaxImages = 40

// Inspector collects Docker observations through the Docker CLI.
type Inspector struct {
	// Runner executes commands. Tests inject a fake.
	Runner run.Runner
	// Timeout bounds each command. Zero uses run.DefaultTimeout.
	Timeout time.Duration
	// Binary is the Docker executable name; empty means "docker".
	Binary string
	// MaxImages overrides DefaultMaxImages when positive.
	MaxImages int
}

// Collect gathers container, storage and, for running containers, usage data.
//
// Every failure mode (CLI missing, daemon stopped, timeout, unparsable output)
// is reported inside the returned struct. Collect never returns an error and
// never changes Docker state.
func (i Inspector) Collect(ctx context.Context) model.DockerInfo {
	binary := i.Binary
	if binary == "" {
		binary = "docker"
	}
	runner := i.Runner
	if runner == nil {
		runner = run.Exec{Timeout: i.Timeout}
	}

	info := model.DockerInfo{
		Containers: []model.DockerContainer{},
		DiskUsage:  []model.DockerDiskUsage{},
		Images:     []model.DockerImage{},
	}

	version := runner.Run(ctx, binary, "version", "--format", "{{.Server.Version}}")
	if !version.OK() {
		info.Status = model.DockerStatus{Available: false, Reason: describeFailure(version)}
		return info
	}
	info.Status = model.DockerStatus{
		Available:     true,
		ServerVersion: strings.TrimSpace(version.Stdout),
	}
	if res := runner.Run(ctx, binary, "context", "show"); res.OK() {
		info.Status.Context = strings.TrimSpace(res.Stdout)
	}

	if res := runner.Run(ctx, binary, "ps", "-a", "--no-trunc", "--format", "{{json .}}"); res.OK() {
		containers, warnings := ParseContainerList(res.Stdout)
		info.Containers = containers
		info.Warnings = append(info.Warnings, warnings...)
	} else {
		info.Warnings = append(info.Warnings, "Container list unavailable: "+res.FailureReason())
	}

	if info.RunningContainers() > 0 {
		// --no-stream makes exactly one measurement, so this cannot hang while
		// watching a live stream.
		res := runner.Run(ctx, binary, "stats", "--no-stream", "--format", "{{json .}}")
		if res.OK() {
			stats, warnings := ParseStats(res.Stdout)
			info.Warnings = append(info.Warnings, warnings...)
			for idx := range info.Containers {
				c := &info.Containers[idx]
				if !c.Running() {
					continue
				}
				s, ok := stats[c.ID]
				if !ok {
					s, ok = stats[ShortID(c.ID)]
				}
				if !ok {
					s, ok = stats[c.Name]
				}
				if ok {
					c.StatsKnown = true
					c.CPUPercent = s.CPUPercent
					c.MemoryUsage = s.MemoryUsage
					c.MemoryPerc = s.MemoryPerc
				}
			}
		} else {
			info.Warnings = append(info.Warnings, "Container resource usage unavailable: "+res.FailureReason())
		}
	}

	info.DiskUsage = i.collectDiskUsage(ctx, runner, binary, &info)
	info.Images = i.collectImages(ctx, runner, binary, &info)
	return info
}

func (i Inspector) collectDiskUsage(ctx context.Context, runner run.Runner, binary string, info *model.DockerInfo) []model.DockerDiskUsage {
	if res := runner.Run(ctx, binary, "system", "df", "--format", "{{json .}}"); res.OK() {
		rows, warnings := ParseDiskUsageJSON(res.Stdout)
		info.Warnings = append(info.Warnings, warnings...)
		if len(rows) > 0 {
			return rows
		}
	}
	// Older Docker versions do not support --format for `system df`; fall back
	// to the fixed-width table.
	res := runner.Run(ctx, binary, "system", "df")
	if !res.OK() {
		info.Warnings = append(info.Warnings, "Docker storage usage unavailable: "+res.FailureReason())
		return nil
	}
	rows, warnings := ParseDiskUsageTable(res.Stdout)
	info.Warnings = append(info.Warnings, warnings...)
	return rows
}

func (i Inspector) collectImages(ctx context.Context, runner run.Runner, binary string, info *model.DockerInfo) []model.DockerImage {
	res := runner.Run(ctx, binary, "image", "ls", "--no-trunc", "--format", "{{json .}}")
	if !res.OK() {
		info.Warnings = append(info.Warnings, "Docker image list unavailable: "+res.FailureReason())
		return nil
	}
	images, warnings := ParseImageList(res.Stdout)
	info.Warnings = append(info.Warnings, warnings...)

	for idx := range images {
		for _, c := range info.Containers {
			if c.Image != "" && (c.Image == images[idx].Repository+":"+images[idx].Tag || c.Image == images[idx].ID) {
				images[idx].InUse = true
				break
			}
		}
	}

	sort.SliceStable(images, func(a, b int) bool {
		var sizeA, sizeB uint64
		if images[a].Size != nil {
			sizeA = *images[a].Size
		}
		if images[b].Size != nil {
			sizeB = *images[b].Size
		}
		return sizeA > sizeB
	})

	limit := i.MaxImages
	if limit <= 0 {
		limit = DefaultMaxImages
	}
	if len(images) > limit {
		images = images[:limit]
		info.ImagesTruncated = true
	}
	return images
}

// describeFailure turns a Docker CLI failure into a short explanation a
// developer can act on, without implying that StackBroom tried to fix it.
func describeFailure(res run.Result) string {
	output := strings.ToLower(res.Stdout + " " + res.Stderr)
	switch {
	case res.Missing:
		return "Docker CLI not found on PATH"
	case res.TimedOut:
		return "Docker CLI did not respond within " + res.Timeout.String()
	case strings.Contains(output, "cannot connect") ||
		strings.Contains(output, "error during connect") ||
		strings.Contains(output, "is the docker daemon running") ||
		strings.Contains(output, "open //./pipe/docker") ||
		strings.Contains(output, "dockerdesktoplinuxengine"):
		return "Docker daemon is not reachable (Docker Desktop may not be running); StackBroom did not start it"
	case strings.Contains(output, "permission denied"):
		return "Docker daemon refused access (permission denied)"
	}
	return res.FailureReason()
}
