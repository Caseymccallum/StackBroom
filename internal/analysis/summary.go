// Package analysis turns raw observations into a summary, duplicate suspects
// and conservative recommendations.
//
// Nothing in this package can change the system: it only reads the collected
// model and produces text or numbers for the user to review.
package analysis

import (
	"sort"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// DefaultTopN is how many memory-heavy processes are listed by default.
const DefaultTopN = 8

// Summarize builds the overview numbers for a scan.
func Summarize(procs []model.Process, dockerInfo model.DockerInfo, wslInfo model.WSLInfo, topN int) model.Summary {
	if topN <= 0 {
		topN = DefaultTopN
	}

	summary := model.Summary{
		TotalProcesses:          len(procs),
		TopProcesses:            []model.TopProcess{},
		RuntimeTotals:           []model.RuntimeTotal{},
		DockerRunning:           dockerInfo.RunningContainers(),
		DockerStopped:           dockerInfo.StoppedContainers(),
		WSLRunningDistributions: wslInfo.RunningDistributions(),
		WSLVMMemoryBytes:        wslInfo.VMMemoryBytes,
		WSLVMMemoryKnown:        wslInfo.VMMemoryKnown,
	}

	runtimeMemory := map[model.Runtime]uint64{}
	runtimeCount := map[model.Runtime]int{}
	runtimeKnown := map[model.Runtime]int{}

	for _, p := range procs {
		if p.Category == model.CategoryDevServer {
			summary.DevServerCount++
		}
		if !p.IsDevelopment() {
			continue
		}
		summary.DevProcessCount++
		if p.MemoryKnown {
			summary.DevMemoryBytes += p.WorkingSetBytes
			summary.DevMemoryKnown++
		}
		runtimeCount[p.Runtime]++
		if p.MemoryKnown {
			runtimeMemory[p.Runtime] += p.WorkingSetBytes
			runtimeKnown[p.Runtime]++
		}
	}

	for rt, count := range runtimeCount {
		summary.RuntimeTotals = append(summary.RuntimeTotals, model.RuntimeTotal{
			Runtime:          rt,
			ProcessCount:     count,
			MemoryBytes:      runtimeMemory[rt],
			MemoryKnownCount: runtimeKnown[rt],
		})
	}
	sort.Slice(summary.RuntimeTotals, func(i, j int) bool {
		a, b := summary.RuntimeTotals[i], summary.RuntimeTotals[j]
		if a.MemoryBytes != b.MemoryBytes {
			return a.MemoryBytes > b.MemoryBytes
		}
		if a.ProcessCount != b.ProcessCount {
			return a.ProcessCount > b.ProcessCount
		}
		return a.Runtime < b.Runtime
	})

	summary.TopProcesses = topMemoryProcesses(procs, topN)
	for _, tp := range summary.TopProcesses {
		summary.TopMemoryBytes += tp.MemoryBytes
	}

	for _, row := range dockerInfo.DiskUsage {
		summary.DockerSizeRowsTotal++
		if row.Size != nil {
			summary.DockerSizeRowsKnown++
			summary.DockerTotalBytes += *row.Size
		}
		summary.DockerReclaimRowsTotal++
		if row.Reclaimable != nil {
			summary.DockerReclaimRowsKnown++
			summary.DockerReclaimableBytes += *row.Reclaimable
		}
	}
	summary.DockerTotalKnown = summary.DockerSizeRowsTotal > 0 && summary.DockerSizeRowsKnown == summary.DockerSizeRowsTotal
	// Reclaimable completeness is independent of size completeness: Docker can
	// report a total without a reclaimable figure (and vice versa).
	summary.DockerReclaimKnown = summary.DockerReclaimRowsTotal > 0 && summary.DockerReclaimRowsKnown == summary.DockerReclaimRowsTotal

	return summary
}

func topMemoryProcesses(procs []model.Process, n int) []model.TopProcess {
	known := make([]model.Process, 0, len(procs))
	for _, p := range procs {
		if p.MemoryKnown && p.PID > 0 {
			known = append(known, p)
		}
	}
	sort.SliceStable(known, func(i, j int) bool {
		if known[i].WorkingSetBytes != known[j].WorkingSetBytes {
			return known[i].WorkingSetBytes > known[j].WorkingSetBytes
		}
		return known[i].PID < known[j].PID
	})
	if len(known) > n {
		known = known[:n]
	}

	out := make([]model.TopProcess, 0, len(known))
	for _, p := range known {
		tp := model.TopProcess{
			PID:         p.PID,
			Name:        p.Name,
			MemoryBytes: p.WorkingSetBytes,
			MemoryKnown: p.MemoryKnown,
			Category:    p.Category,
			Runtime:     p.Runtime,
		}
		if p.Project != nil {
			tp.ProjectName = p.Project.Name
		}
		out = append(out, tp)
	}
	return out
}
