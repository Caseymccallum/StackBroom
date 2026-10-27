package analysis

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// stoppedContainerRecommendation offers long-stopped containers for review. It
// never proposes a specific container for deletion and never suggests removing
// volumes.
func stoppedContainerRecommendation(dockerInfo model.DockerInfo, opts Options) []model.Recommendation {
	if !dockerInfo.Status.Available {
		return nil
	}

	var stale []model.DockerContainer
	for _, c := range dockerInfo.Containers {
		if c.Running() || c.State == "" {
			continue
		}
		if c.StoppedForDays >= opts.StaleContainerDays {
			stale = append(stale, c)
		}
	}
	if len(stale) == 0 {
		return nil
	}

	sort.SliceStable(stale, func(i, j int) bool {
		if stale[i].StoppedForDays != stale[j].StoppedForDays {
			return stale[i].StoppedForDays > stale[j].StoppedForDays
		}
		return stale[i].Name < stale[j].Name
	})

	rec := model.Recommendation{
		ID:         KindStoppedContainers,
		Kind:       KindStoppedContainers,
		Risk:       model.RiskMedium,
		Confidence: model.Confirmed,
	}

	if len(stale) == 1 {
		c := stale[0]
		rec.Title = fmt.Sprintf("Container %q has been stopped for %s", c.Name, ageOrDefault(c.StoppedForText))
		rec.Detected = fmt.Sprintf("Docker reports container %q with status %q.", c.Name, c.Status)
	} else {
		rec.Title = fmt.Sprintf("%d containers have been stopped for at least %d days", len(stale), opts.StaleContainerDays)
		rec.Detected = fmt.Sprintf("Docker reports %d containers stopped for at least %d days; the oldest reports %s.",
			len(stale), opts.StaleContainerDays, ageOrDefault(stale[0].StoppedForText))
	}

	rec.WhyUnnecessary = "Stopped containers keep their writable layer on disk and keep a service definition in the environment. " +
		"An old container is often a leftover from a finished project, which makes the current environment harder to read."

	shown := stale
	if len(shown) > opts.MaxContainerNames {
		shown = shown[:opts.MaxContainerNames]
	}
	for _, c := range shown {
		detail := fmt.Sprintf("%s - %s", c.Name, c.Status)
		if c.ComposeProject != "" {
			detail += " (compose project " + c.ComposeProject + ")"
		}
		rec.Evidence = append(rec.Evidence, detail)
		rec.Subjects = append(rec.Subjects, model.Subject{Kind: "container", ID: c.Name, Detail: c.Status})
	}
	if len(stale) > len(shown) {
		rec.Evidence = append(rec.Evidence, fmt.Sprintf("%d more stopped containers are not listed.", len(stale)-len(shown)))
	}
	rec.Evidence = append(rec.Evidence,
		"Ages come from Docker's rounded status text (for example \"Exited (0) 5 months ago\"), not from timestamps.")

	if row, ok := dockerInfo.DiskUsageOf("Containers"); ok && strings.TrimSpace(row.SizeText) != "" {
		rec.Resource = fmt.Sprintf("Docker reports %s used by container writable layers across all %d containers; per-container sizes were not measured.",
			row.SizeText, len(dockerInfo.Containers))
		if row.Size != nil {
			rec.ResourceBytes = *row.Size
		}
	} else {
		rec.Resource = "Disk use for these containers is unknown: Docker did not report a container size."
		rec.ResourceUnknown = true
	}

	rec.Review = []string{
		"Check whether you still need logs, test data or the writable layer inside these containers.",
		"Confirm whether the project's Compose file still defines them; `docker compose ps` in that project lists the services it expects.",
		"Note that named volumes are separate from containers and are not included in the container size.",
		"Keep anything whose contents you could not recreate.",
	}
	rec.RiskDetail = "Removing a container deletes its writable layer and any anonymous volumes. Named volumes survive container removal, " +
		"but a blanket volume prune would delete them. Stopping an already stopped container changes nothing."
	return []model.Recommendation{rec}
}

func ageOrDefault(s string) string {
	if strings.TrimSpace(s) == "" {
		return "an unknown period"
	}
	return s
}
