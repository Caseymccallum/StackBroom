// Package privacy removes sensitive detail from StackBroom output.
//
// Inspection needs full command lines to classify processes and associate them
// with projects. That data must not reach ordinary output: command lines can
// contain passwords, API tokens, connection strings and private paths. This
// package applies a single, explicit policy at the presentation boundary, so
// text, JSON, evidence and warnings all obey the same rules.
package privacy

import (
	"os"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// Mode selects how much detail may be emitted.
type Mode string

const (
	// ModeDefault omits command lines entirely. This is the default for every
	// command.
	ModeDefault Mode = "default"
	// ModeDiagnostics emits command lines after masking secrets and tokenising
	// home directories. It must be requested explicitly.
	ModeDiagnostics Mode = "diagnostics"
)

// Policy controls redaction.
type Policy struct {
	Mode Mode
	// UserProfile is the home directory to replace with %USERPROFILE%. When
	// empty, the current user's home is used.
	UserProfile string
}

// DefaultPolicy returns the policy for a mode using the current user's profile.
func DefaultPolicy(mode Mode) Policy {
	home, _ := os.UserHomeDir()
	return Policy{Mode: mode, UserProfile: home}
}

// Apply returns a redacted deep copy of a snapshot. The input is never modified,
// so internal analysis keeps the detail it needs.
func Apply(snapshot model.Snapshot, policy Policy) model.Snapshot {
	out := cloneSnapshot(snapshot)
	home := policy.UserProfile
	if home == "" {
		home, _ = os.UserHomeDir()
	}

	info := model.RedactionInfo{
		Mode:         string(policy.Mode),
		CommandLines: "omitted",
		Paths:        "user-profile-tokenised",
		Secrets:      "masked",
	}
	if policy.Mode == ModeDiagnostics {
		info.CommandLines = "redacted"
	}
	out.Redaction = &info

	for i := range out.Processes {
		process := &out.Processes[i]
		if process.CommandLine != "" {
			if policy.Mode == ModeDiagnostics {
				process.CommandLine = redactText(process.CommandLine, home)
			} else {
				process.CommandLine = ""
				process.CommandLineOmitted = true
			}
		}
		process.ExePath = redactText(process.ExePath, home)
		process.ParentName = redactText(process.ParentName, home)
		if process.Project != nil {
			process.Project.Root = redactText(process.Project.Root, home)
			process.Project.Evidence = redactSlice(process.Project.Evidence, home)
		}
	}

	out.Network.Reason = redactText(out.Network.Reason, home)
	out.Network.Warnings = redactSlice(out.Network.Warnings, home)
	for i := range out.Network.Listeners {
		listener := &out.Network.Listeners[i]
		listener.ProcessName = redactText(listener.ProcessName, home)
		listener.ProjectName = redactText(listener.ProjectName, home)
	}

	out.Docker.Status.Reason = redactText(out.Docker.Status.Reason, home)
	out.Docker.Status.Endpoint = redactText(out.Docker.Status.Endpoint, home)
	out.Docker.Status.Context = redactText(out.Docker.Status.Context, home)
	for i := range out.Docker.Containers {
		c := &out.Docker.Containers[i]
		c.Name = redactText(c.Name, home)
		c.Image = redactText(c.Image, home)
		c.Status = redactText(c.Status, home)
		c.ComposeProject = redactText(c.ComposeProject, home)
		c.ComposeService = redactText(c.ComposeService, home)
		c.ComposeWorkingDir = redactText(c.ComposeWorkingDir, home)
	}
	for i := range out.Docker.DiskUsage {
		out.Docker.DiskUsage[i].SizeText = redactText(out.Docker.DiskUsage[i].SizeText, home)
		out.Docker.DiskUsage[i].ReclaimableText = redactText(out.Docker.DiskUsage[i].ReclaimableText, home)
	}
	for i := range out.Docker.Images {
		img := &out.Docker.Images[i]
		img.Repository = redactText(img.Repository, home)
		img.Tag = redactText(img.Tag, home)
	}
	out.Docker.Warnings = redactSlice(out.Docker.Warnings, home)

	out.WSL.Reason = redactText(out.WSL.Reason, home)
	for i := range out.WSL.Distributions {
		out.WSL.Distributions[i].Name = redactText(out.WSL.Distributions[i].Name, home)
		out.WSL.Distributions[i].State = redactText(out.WSL.Distributions[i].State, home)
	}

	for i := range out.Duplicates {
		g := &out.Duplicates[i]
		g.Key = redactText(g.Key, home)
		g.ProjectRoot = redactText(g.ProjectRoot, home)
		g.Evidence = redactSlice(g.Evidence, home)
		for j := range g.Listeners {
			g.Listeners[j].Address = redactText(g.Listeners[j].Address, home)
		}
	}

	for i := range out.Recommendations {
		rec := &out.Recommendations[i]
		rec.Title = redactText(rec.Title, home)
		rec.Detected = redactText(rec.Detected, home)
		rec.WhyUnnecessary = redactText(rec.WhyUnnecessary, home)
		rec.Evidence = redactSlice(rec.Evidence, home)
		rec.Resource = redactText(rec.Resource, home)
		rec.Review = redactSlice(rec.Review, home)
		rec.RiskDetail = redactText(rec.RiskDetail, home)
		for j := range rec.Subjects {
			subject := &rec.Subjects[j]
			subject.ID = redactText(subject.ID, home)
			if subject.Kind == "process" && subject.Detail != "" {
				// A process subject's detail is its command line.
				if policy.Mode == ModeDiagnostics {
					subject.Detail = redactText(subject.Detail, home)
				} else {
					subject.Detail = ""
				}
			} else {
				subject.Detail = redactText(subject.Detail, home)
			}
		}
	}

	out.Warnings = redactSlice(out.Warnings, home)
	return out
}

func redactSlice(values []string, home string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = redactText(value, home)
	}
	return out
}
