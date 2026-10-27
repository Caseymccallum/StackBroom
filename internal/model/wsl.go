package model

import "strings"

// WSLDistribution is one distribution reported by `wsl.exe --list --verbose`.
type WSLDistribution struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	Version   string `json:"version"`
	IsDefault bool   `json:"isDefault"`
}

// Running reports whether the distribution is currently running.
func (d WSLDistribution) Running() bool { return strings.EqualFold(d.State, "running") }

// WSLInfo is the read-only WSL observation.
//
// Memory is only known for the WSL2 utility VM as a whole (the vmmem or
// vmmemWSL process). StackBroom does not attribute that memory to individual
// distributions, because it cannot measure it without executing commands
// inside a distribution.
type WSLInfo struct {
	Available     bool              `json:"available"`
	Reason        string            `json:"reason,omitempty"`
	Distributions []WSLDistribution `json:"distributions"`

	VMMemoryBytes uint64 `json:"vmMemoryBytes"`
	VMMemoryKnown bool   `json:"vmMemoryKnown"`
	VMProcessName string `json:"vmProcessName,omitempty"`
	VMNote        string `json:"vmNote,omitempty"`
	// Warnings reports incomplete or truncated inspection output.
	Warnings []string `json:"warnings,omitempty"`
}

// RunningDistributions counts distributions in the running state.
func (w WSLInfo) RunningDistributions() int {
	n := 0
	for _, d := range w.Distributions {
		if d.Running() {
			n++
		}
	}
	return n
}
