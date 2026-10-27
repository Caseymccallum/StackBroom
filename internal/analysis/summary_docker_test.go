package analysis

import (
	"testing"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func size(v uint64) *uint64 { return &v }

func TestDockerAccountingCompleteReport(t *testing.T) {
	docker := model.DockerInfo{
		Status: model.DockerStatus{Available: true},
		DiskUsage: []model.DockerDiskUsage{
			{Type: "Images", Size: size(1000), Reclaimable: size(500)},
			{Type: "Containers", Size: size(100), Reclaimable: size(100)},
			{Type: "Local Volumes", Size: size(200), Reclaimable: size(50)},
			{Type: "Build Cache", Size: size(300), Reclaimable: size(75)},
		},
	}
	got := Summarize(nil, docker, model.WSLInfo{}, 8)

	if !got.DockerTotalKnown || !got.DockerReclaimKnown {
		t.Fatalf("complete report should be fully known: %+v", got)
	}
	if got.DockerTotalBytes != 1600 || got.DockerReclaimableBytes != 725 {
		t.Errorf("totals = %d / %d", got.DockerTotalBytes, got.DockerReclaimableBytes)
	}
	if got.DockerSizeRowsKnown != 4 || got.DockerSizeRowsTotal != 4 {
		t.Errorf("size rows = %d/%d", got.DockerSizeRowsKnown, got.DockerSizeRowsTotal)
	}
	if got.DockerReclaimRowsKnown != 4 || got.DockerReclaimRowsTotal != 4 {
		t.Errorf("reclaim rows = %d/%d", got.DockerReclaimRowsKnown, got.DockerReclaimRowsTotal)
	}
}

func TestDockerAccountingMissingReclaimableField(t *testing.T) {
	docker := model.DockerInfo{
		Status: model.DockerStatus{Available: true},
		DiskUsage: []model.DockerDiskUsage{
			{Type: "Images", Size: size(1000), Reclaimable: size(500)},
			{Type: "Build Cache", Size: size(300)},
		},
	}
	got := Summarize(nil, docker, model.WSLInfo{}, 8)

	if !got.DockerTotalKnown {
		t.Error("size total should still be complete")
	}
	if got.DockerReclaimKnown {
		t.Error("reclaim total must not be claimed complete when a row lacks it")
	}
	if got.DockerReclaimRowsKnown != 1 || got.DockerReclaimRowsTotal != 2 {
		t.Errorf("reclaim rows = %d/%d", got.DockerReclaimRowsKnown, got.DockerReclaimRowsTotal)
	}
	if got.DockerReclaimableBytes != 500 {
		t.Errorf("reclaimable = %d, want the known rows only", got.DockerReclaimableBytes)
	}
}

func TestDockerAccountingUnknownSizesButCompleteReclaim(t *testing.T) {
	// Regression: reclaim completeness used to follow size completeness. A row
	// with a reclaimable figure but no size must not make reclaim unknown.
	docker := model.DockerInfo{
		Status: model.DockerStatus{Available: true},
		DiskUsage: []model.DockerDiskUsage{
			{Type: "Images", Reclaimable: size(500)},
			{Type: "Build Cache", Size: size(300), Reclaimable: size(75)},
		},
	}
	got := Summarize(nil, docker, model.WSLInfo{}, 8)

	if got.DockerTotalKnown {
		t.Error("size total should be incomplete")
	}
	if !got.DockerReclaimKnown {
		t.Error("reclaim total is complete even though sizes are not")
	}
	if got.DockerReclaimableBytes != 575 {
		t.Errorf("reclaimable = %d, want 575", got.DockerReclaimableBytes)
	}
	if got.DockerTotalBytes != 300 {
		t.Errorf("total = %d, want the known size only", got.DockerTotalBytes)
	}
}

func TestDockerAccountingPartiallyParsed(t *testing.T) {
	docker := model.DockerInfo{
		Status: model.DockerStatus{Available: true},
		DiskUsage: []model.DockerDiskUsage{
			{Type: "Images", Size: size(1000), Reclaimable: size(500)},
			{Type: "Build Cache"},
		},
	}
	got := Summarize(nil, docker, model.WSLInfo{}, 8)

	if got.DockerTotalKnown || got.DockerReclaimKnown {
		t.Errorf("partially parsed report must not be fully known: %+v", got)
	}
	if got.DockerSizeRowsKnown != 1 || got.DockerSizeRowsTotal != 2 {
		t.Errorf("size rows = %d/%d", got.DockerSizeRowsKnown, got.DockerSizeRowsTotal)
	}
	if got.DockerReclaimRowsKnown != 1 || got.DockerReclaimRowsTotal != 2 {
		t.Errorf("reclaim rows = %d/%d", got.DockerReclaimRowsKnown, got.DockerReclaimRowsTotal)
	}
}

func TestDockerAccountingEmptyInventory(t *testing.T) {
	got := Summarize(nil, model.DockerInfo{Status: model.DockerStatus{Available: true}}, model.WSLInfo{}, 8)
	if got.DockerTotalKnown || got.DockerReclaimKnown {
		t.Error("empty inventory must not report known totals")
	}
	if got.DockerTotalBytes != 0 || got.DockerReclaimableBytes != 0 {
		t.Errorf("empty totals = %d / %d", got.DockerTotalBytes, got.DockerReclaimableBytes)
	}
}
