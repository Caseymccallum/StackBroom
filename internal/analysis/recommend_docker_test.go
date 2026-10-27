package analysis

import (
	"strings"
	"testing"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func TestRecommendStoppedContainers(t *testing.T) {
	dockerInfo := model.DockerInfo{
		Status: model.DockerStatus{Available: true},
		Containers: []model.DockerContainer{
			{Name: "old-db", State: "exited", Status: "Exited (0) 5 months ago", StoppedForDays: 150, StoppedForText: "5 months"},
			{Name: "recent-db", State: "exited", Status: "Exited (0) 5 days ago", StoppedForDays: 5, StoppedForText: "5 days"},
			{Name: "live-web", State: "running", Status: "Up 2 hours"},
		},
		DiskUsage: []model.DockerDiskUsage{{
			Type: "Containers", SizeText: "30.09MB", Size: ptrU64(30 << 20),
			ReclaimableText: "30.09MB (100%)", Reclaimable: ptrU64(30 << 20),
		}},
	}

	recs := Recommend(nil, dockerInfo, model.WSLInfo{}, nil, testNow, DefaultOptions())
	if len(recs) != 1 {
		t.Fatalf("recommendations = %+v, want 1", recs)
	}
	assertContract(t, recs)

	rec := recs[0]
	if rec.Kind != KindStoppedContainers {
		t.Fatalf("kind = %q", rec.Kind)
	}
	if !strings.Contains(rec.Title, "old-db") {
		t.Errorf("title = %q", rec.Title)
	}
	evidence := strings.Join(rec.Evidence, "\n")
	if !strings.Contains(evidence, "old-db") {
		t.Errorf("evidence missing the stale container: %v", rec.Evidence)
	}
	if strings.Contains(evidence, "recent-db") || strings.Contains(evidence, "live-web") {
		t.Errorf("evidence includes containers that did not meet the threshold: %v", rec.Evidence)
	}
	if rec.ResourceBytes != 30<<20 {
		t.Errorf("resource bytes = %d", rec.ResourceBytes)
	}
	if !strings.Contains(strings.ToLower(strings.Join(rec.Review, " ")), "volume") {
		t.Errorf("review should mention volumes: %v", rec.Review)
	}
}

func TestRecommendStoppedContainersRequiresDockerAvailable(t *testing.T) {
	dockerInfo := model.DockerInfo{
		Status: model.DockerStatus{Available: false, Reason: "daemon not running"},
		Containers: []model.DockerContainer{
			{Name: "old", State: "exited", StoppedForDays: 150, StoppedForText: "5 months"},
		},
	}
	if recs := Recommend(nil, dockerInfo, model.WSLInfo{}, nil, testNow, DefaultOptions()); len(recs) != 0 {
		t.Fatalf("unexpected recommendations when Docker is unavailable: %+v", recs)
	}
}

func TestRecommendBuildCache(t *testing.T) {
	tests := []struct {
		name        string
		reclaimable uint64
		wantCount   int
	}{
		{"below threshold", 100 << 20, 0},
		{"above threshold", 400 << 20, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dockerInfo := model.DockerInfo{
				Status: model.DockerStatus{Available: true},
				DiskUsage: []model.DockerDiskUsage{{
					Type: "Build Cache", SizeText: "630.1MB", Size: ptrU64(630 << 20),
					ReclaimableText: "400MB", Reclaimable: ptrU64(tc.reclaimable),
				}},
			}
			recs := Recommend(nil, dockerInfo, model.WSLInfo{}, nil, testNow, DefaultOptions())
			if len(recs) != tc.wantCount {
				t.Fatalf("recommendations = %d, want %d", len(recs), tc.wantCount)
			}
			assertContract(t, recs)
			if tc.wantCount == 1 {
				if recs[0].Kind != KindBuildCache || recs[0].Risk != model.RiskLow {
					t.Errorf("rec = %+v", recs[0])
				}
				if !strings.Contains(recs[0].RiskDetail, "no project files") {
					t.Errorf("risk detail should be explicit about data safety: %q", recs[0].RiskDetail)
				}
			}
		})
	}
}

func TestRecommendUnusedImages(t *testing.T) {
	dockerInfo := model.DockerInfo{
		Status: model.DockerStatus{Available: true},
		DiskUsage: []model.DockerDiskUsage{{
			Type: "Images", SizeText: "15.64GB", Size: ptrU64(15 << 30),
			ReclaimableText: "12.41GB (79%)", Reclaimable: ptrU64(12 << 30),
		}},
		Images: []model.DockerImage{
			{Repository: "public.ecr.aws/supabase/studio", Tag: "2026.10.19", SizeText: "1.75GB", Size: ptrU64(1875 << 20)},
			{Repository: "nginx", Tag: "latest", SizeText: "187MB", Size: ptrU64(187 << 20), InUse: true},
		},
	}
	recs := Recommend(nil, dockerInfo, model.WSLInfo{}, nil, testNow, DefaultOptions())
	if len(recs) != 1 {
		t.Fatalf("recommendations = %+v", recs)
	}
	assertContract(t, recs)

	rec := recs[0]
	if rec.Kind != KindUnusedImages || rec.Risk != model.RiskMedium {
		t.Errorf("rec = %+v", rec)
	}
	evidence := strings.Join(rec.Evidence, "\n")
	if !strings.Contains(evidence, "supabase/studio") {
		t.Errorf("evidence should name the large unused image: %v", rec.Evidence)
	}
	if strings.Contains(evidence, "nginx") {
		t.Errorf("in-use image should not be listed as unused: %v", rec.Evidence)
	}
}
