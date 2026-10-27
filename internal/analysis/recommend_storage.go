package analysis

import (
	"fmt"
	"strings"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func buildCacheRecommendation(dockerInfo model.DockerInfo, opts Options) []model.Recommendation {
	if !dockerInfo.Status.Available {
		return nil
	}
	row, ok := dockerInfo.DiskUsageOf("Build Cache")
	if !ok || row.Reclaimable == nil || *row.Reclaimable < opts.BuildCacheWarnBytes {
		return nil
	}

	rec := model.Recommendation{
		ID:            KindBuildCache,
		Kind:          KindBuildCache,
		Risk:          model.RiskLow,
		Confidence:    model.Confirmed,
		ResourceBytes: *row.Reclaimable,
		Title:         "Docker reports " + row.ReclaimableText + " of build cache as reclaimable",
		Detected: fmt.Sprintf("Docker's storage report lists %s of build cache in total, of which %s is reclaimable.",
			orUnknown(row.SizeText), row.ReclaimableText),
		WhyUnnecessary: "Build cache only exists to make image builds faster. Cache entries Docker considers unused provide no benefit " +
			"while occupying disk. Reclaiming them costs rebuild time, not correctness.",
		Evidence: []string{
			"Docker system df: type=Build Cache, total=" + orUnknown(row.SizeText) + ", reclaimable=" + row.ReclaimableText + ".",
			"Reclaimable is Docker's own estimate, based on cache entries not needed by current images.",
			"Build cache is separate from images, containers and volumes.",
		},
		Resource: fmt.Sprintf("%s reclaimable of %s total build cache.", row.ReclaimableText, orUnknown(row.SizeText)),
		Review: []string{
			"Check whether you are actively iterating on Dockerfiles; the next build will be slower.",
			"Prefer clearing build cache only, rather than pruning everything Docker can find.",
			"Confirm no CI job on this machine depends on a warm build cache.",
		},
		RiskDetail: "Low for data: build cache contains no project files or volumes. The cost is rebuild time and extra network " +
			"traffic the next time an image is built.",
		Subjects: []model.Subject{{Kind: "docker-storage", ID: "Build Cache", Detail: row.ReclaimableText}},
	}
	return []model.Recommendation{rec}
}

func unusedImageRecommendation(dockerInfo model.DockerInfo, opts Options) []model.Recommendation {
	if !dockerInfo.Status.Available {
		return nil
	}
	row, ok := dockerInfo.DiskUsageOf("Images")
	if !ok || row.Reclaimable == nil || *row.Reclaimable < opts.ImageReclaimWarnBytes {
		return nil
	}

	rec := model.Recommendation{
		ID:            KindUnusedImages,
		Kind:          KindUnusedImages,
		Risk:          model.RiskMedium,
		Confidence:    model.Confirmed,
		ResourceBytes: *row.Reclaimable,
		Title:         "Docker reports " + row.ReclaimableText + " of image storage as reclaimable",
		Detected: fmt.Sprintf("Docker's storage report lists %s of images in total, of which %s is reclaimable.",
			orUnknown(row.SizeText), row.ReclaimableText),
		WhyUnnecessary: "Images that no container references still occupy disk. They can usually be pulled or rebuilt, but that " +
			"needs network access and time, so the decision is yours.",
		Evidence: []string{
			"Docker system df: type=Images, total=" + orUnknown(row.SizeText) + ", reclaimable=" + row.ReclaimableText + ".",
			"Reclaimable is Docker's own estimate and already excludes images used by existing containers, including stopped ones.",
		},
		Resource: fmt.Sprintf("%s reclaimable of %s total image storage.", row.ReclaimableText, orUnknown(row.SizeText)),
		Review: []string{
			"Confirm you can re-pull or rebuild any image you still need.",
			"Check whether a project relies on a locally built image that has no registry copy.",
			"Review image tags before removing anything; tags are the only human-friendly identifier.",
		},
		RiskDetail: "Medium: removing images costs bandwidth and rebuild time, and a locally built image with no registry copy cannot be " +
			"restored. Images used by stopped containers are not counted as reclaimable by Docker.",
		Subjects: []model.Subject{{Kind: "docker-storage", ID: "Images", Detail: row.ReclaimableText}},
	}

	// Name the largest images that no existing container references. This is
	// context for the review, not a deletion list.
	if !dockerInfo.ImagesTruncated {
		limit := 3
		for _, img := range dockerInfo.Images {
			if img.InUse || img.Size == nil || limit == 0 {
				continue
			}
			name := strings.TrimSpace(img.Repository + ":" + img.Tag)
			rec.Evidence = append(rec.Evidence, fmt.Sprintf("No container references %s (%s).", name, img.SizeText))
			limit--
		}
	}
	return []model.Recommendation{rec}
}

func orUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "an unreported amount"
	}
	return s
}
