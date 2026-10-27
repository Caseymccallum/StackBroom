package analysis

import (
	"strings"
	"testing"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func TestRecommendWSLRunningDistribution(t *testing.T) {
	wslInfo := model.WSLInfo{
		Available: true,
		Distributions: []model.WSLDistribution{
			{Name: "Ubuntu-22.04", State: "Running", Version: "2"},
			{Name: "docker-desktop", State: "Running", Version: "2"},
			{Name: "Debian", State: "Stopped", Version: "2"},
		},
	}
	recs := Recommend(nil, model.DockerInfo{}, wslInfo, nil, testNow, DefaultOptions())
	if len(recs) != 1 {
		t.Fatalf("recommendations = %+v, want only Ubuntu", recs)
	}
	assertContract(t, recs)

	rec := recs[0]
	if rec.Kind != KindWSLRunning || rec.Risk != model.RiskHigh {
		t.Errorf("rec = %+v", rec)
	}
	if !strings.Contains(rec.Title, "Ubuntu-22.04") {
		t.Errorf("title = %q", rec.Title)
	}
	if !rec.ResourceUnknown || !strings.Contains(strings.ToLower(rec.Resource), "unknown") {
		t.Errorf("resource should be explicitly unknown: %q", rec.Resource)
	}
	if !strings.Contains(strings.ToLower(rec.RiskDetail), "terminat") &&
		!strings.Contains(strings.ToLower(rec.RiskDetail), "stops") {
		t.Errorf("risk detail should describe termination: %q", rec.RiskDetail)
	}
}

func TestRecommendWSLUsesHostVMMemoryWhenKnown(t *testing.T) {
	wslInfo := model.WSLInfo{
		Available:     true,
		Distributions: []model.WSLDistribution{{Name: "Ubuntu", State: "Running", Version: "2"}},
		VMMemoryBytes: 473 << 20,
		VMMemoryKnown: true,
		VMProcessName: "vmmem",
		VMNote:        "vmmem is the shared WSL2 utility VM",
	}
	recs := Recommend(nil, model.DockerInfo{}, wslInfo, nil, testNow, DefaultOptions())
	if len(recs) != 1 {
		t.Fatalf("recommendations = %+v", recs)
	}
	assertContract(t, recs)
	if recs[0].ResourceUnknown {
		t.Error("resource should be known when vmmem memory is available")
	}
	if !strings.Contains(strings.Join(recs[0].Evidence, " "), "shared") {
		t.Errorf("evidence should explain the VM is shared: %v", recs[0].Evidence)
	}
}

func TestRecommendSortsByKindPriority(t *testing.T) {
	dockerInfo := model.DockerInfo{
		Status: model.DockerStatus{Available: true},
		Containers: []model.DockerContainer{
			{Name: "old", State: "exited", Status: "Exited (0) 5 months ago", StoppedForDays: 150, StoppedForText: "5 months"},
		},
		DiskUsage: []model.DockerDiskUsage{{
			Type: "Build Cache", SizeText: "1GB", Size: ptrU64(1 << 30),
			ReclaimableText: "600MB", Reclaimable: ptrU64(600 << 20),
		}},
	}
	dups := []model.DuplicateGroup{{
		Kind: model.DuplicateByPort, Key: "node on port 5173", Runtime: model.RuntimeNode,
		Port: 5173, PIDs: []int32{1, 2}, MemoryKnown: 0, Confidence: model.Inferred,
		Evidence: []string{"evidence"},
	}}
	recs := Recommend(nil, dockerInfo, model.WSLInfo{}, dups, testNow, DefaultOptions())
	if len(recs) != 3 {
		t.Fatalf("recommendations = %+v", recs)
	}
	want := []string{KindDuplicateServer, KindStoppedContainers, KindBuildCache}
	for i, kind := range want {
		if recs[i].Kind != kind {
			t.Fatalf("order = %v, want %v", recs, want)
		}
	}
}
