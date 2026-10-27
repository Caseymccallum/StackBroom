package docker

import (
	"strings"
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		in   string
		want uint64
		ok   bool
	}{
		{"0B", 0, true},
		{"41kB", 41000, true},
		{"4.1kB", 4100, true},
		{"979MB", 979000000, true},
		{"1.43GB", 1430000000, true},
		{"630.1MB", 630100000, true},
		{"12.41GB (79%)", 12410000000, true},
		{"1.5GiB", 1610612736, true},
		{" 2 GB ", 2000000000, true},
		{"", 0, false},
		{"N/A", 0, false},
		{"unknown", 0, false},
	}
	for _, tc := range tests {
		got, ok := ParseSize(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseSize(%q) = (%d,%v), want (%d,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParsePercent(t *testing.T) {
	tests := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"12.41GB (79%)", 79, true},
		{"30.09MB (100%)", 100, true},
		{"42.68MB", 0, false},
		{"(0%)", 0, true},
		{"garbage (%)", 0, false},
	}
	for _, tc := range tests {
		got, ok := ParsePercent(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("ParsePercent(%q) = (%v,%v), want (%v,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestSplitColumnsKeepsMultiWordType(t *testing.T) {
	header := "TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE"
	row := "Local Volumes   18        4         979MB     590.3MB (60%)"
	got := splitColumns(header, row)
	want := []string{"Local Volumes", "18", "4", "979MB", "590.3MB (60%)"}
	if len(got) != len(want) {
		t.Fatalf("splitColumns = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("column %d = %q, want %q (all: %#v)", i, got[i], want[i], got)
		}
	}
}

func TestParseDiskUsageTable(t *testing.T) {
	out := "TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE\n" +
		"Images          25        5         15.64GB   12.41GB (79%)\n" +
		"Containers      5         0         30.09MB   30.09MB (100%)\n" +
		"Local Volumes   18        4         979MB     590.3MB (60%)\n" +
		"Build Cache     21        0         630.1MB   42.68MB\n"

	rows, warnings := ParseDiskUsageTable(out)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(rows) != 4 {
		t.Fatalf("parsed %d rows, want 4", len(rows))
	}
	images := rows[0]
	if images.Type != "Images" || images.Total != 25 || images.Active != 5 {
		t.Errorf("images row = %+v", images)
	}
	if images.Size == nil || *images.Size != 15640000000 {
		t.Errorf("images size = %v", images.Size)
	}
	if images.Reclaimable == nil || *images.Reclaimable != 12410000000 {
		t.Errorf("images reclaimable = %v", images.Reclaimable)
	}
	if images.ReclaimablePercent == nil || *images.ReclaimablePercent != 79 {
		t.Errorf("images percent = %v", images.ReclaimablePercent)
	}
	cache := rows[3]
	if cache.Type != "Build Cache" || cache.ReclaimablePercent != nil {
		t.Errorf("build cache row = %+v", cache)
	}
}

func TestParseDiskUsageTableRejectsUnexpectedHeader(t *testing.T) {
	if rows, warnings := ParseDiskUsageTable("hello\nworld\n"); rows != nil || len(warnings) == 0 {
		t.Fatalf("expected a warning and no rows, got %v %v", rows, warnings)
	}
}

func TestParseDiskUsageJSON(t *testing.T) {
	out := `{"Active":"5","Reclaimable":"12.41GB (79%)","Size":"15.64GB","TotalCount":"25","Type":"Images"}
{"Active":"0","Reclaimable":"30.09MB (100%)","Size":"30.09MB","TotalCount":"5","Type":"Containers"}
{"Active":"0","Reclaimable":"42.68MB","Size":"630.1MB","TotalCount":"21","Type":"Build Cache"}`

	rows, warnings := ParseDiskUsageJSON(out)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(rows) != 3 {
		t.Fatalf("parsed %d rows, want 3", len(rows))
	}
	if rows[0].Total != 25 || rows[0].Active != 5 {
		t.Errorf("first row counts = %+v", rows[0])
	}
	if rows[2].Reclaimable == nil || *rows[2].Reclaimable != 42680000 {
		t.Errorf("build cache reclaimable = %v", rows[2].Reclaimable)
	}
}

func TestParseContainerList(t *testing.T) {
	out := `{"ID":"aaaabbbbccccdddd","Names":"web","Image":"nginx:latest","State":"running","Status":"Up 2 hours","Labels":"com.docker.compose.project=mysite,com.docker.compose.service=web,com.docker.compose.project.working_dir=c:\\Users\\Casey\\Web Apps\\mysite","Ports":"0.0.0.0:8080->80/tcp"}
{"ID":"eeeeffff00001111","Names":"db","Image":"mysql:8.0","State":"exited","Status":"Exited (137) 5 months ago","Labels":"","Ports":""}`

	containers, warnings := ParseContainerList(out)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(containers) != 2 {
		t.Fatalf("parsed %d containers, want 2", len(containers))
	}

	web := containers[0]
	if !web.Running() || web.ComposeProject != "mysite" || web.ComposeService != "web" {
		t.Errorf("web container = %+v", web)
	}
	if web.ComposeWorkingDir != `c:\Users\Casey\Web Apps\mysite` {
		t.Errorf("compose working dir = %q", web.ComposeWorkingDir)
	}
	if web.StoppedForText != "" {
		t.Errorf("running container reported an age: %+v", web)
	}

	db := containers[1]
	if db.Running() {
		t.Error("db container should not be running")
	}
	if db.StoppedForText != "5 months" || db.StoppedForDays != 150 {
		t.Errorf("stopped age = %q/%d, want 5 months/150", db.StoppedForText, db.StoppedForDays)
	}
}

func TestParseContainerListSkipsMalformedLines(t *testing.T) {
	out := `{"ID":"abc","Names":"web","State":"running"}
this is not json
{"ID":"def","Names":"db","State":"exited","Status":"Exited (0) 23 hours ago"}`

	containers, warnings := ParseContainerList(out)
	if len(containers) != 2 {
		t.Fatalf("parsed %d containers, want 2", len(containers))
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "line 2") {
		t.Fatalf("warnings = %v, want one about line 2", warnings)
	}
	if containers[1].StoppedForText != "23 hours" || containers[1].StoppedForDays != 0 {
		t.Errorf("23 hour age = %q/%d", containers[1].StoppedForText, containers[1].StoppedForDays)
	}
}

func TestParseImageListAndInUseMatching(t *testing.T) {
	out := `{"ID":"sha256:1111","Repository":"nginx","Tag":"latest","Size":"187MB","CreatedSince":"2 weeks ago"}
{"ID":"sha256:2222","Repository":"mysql","Tag":"8.0","Size":"600MB","CreatedSince":"3 months ago"}`

	images, warnings := ParseImageList(out)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(images) != 2 {
		t.Fatalf("parsed %d images, want 2", len(images))
	}
	if images[0].Repository != "nginx" || images[0].Tag != "latest" {
		t.Errorf("first image = %+v", images[0])
	}
	if images[0].Size == nil || *images[0].Size != 187000000 {
		t.Errorf("first image size = %v", images[0].Size)
	}
}

func TestParseStatsKeysByIDAndName(t *testing.T) {
	out := `{"Container":"aaaabbbbcccc","Name":"web","CPUPerc":"0.50%","MemUsage":"12.5MiB / 1GiB","MemPerc":"1.22%"}
{"Container":"dddd","Name":"db","CPUPerc":"0.00%","MemUsage":"0B / 0B","MemPerc":"0.00%"}`

	stats, warnings := ParseStats(out)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if got := stats["web"]; got.MemoryUsage != "12.5MiB / 1GiB" {
		t.Errorf("stats by name = %+v", got)
	}
	if got := stats["aaaabbbbcccc"]; got.CPUPercent != "0.50%" {
		t.Errorf("stats by id = %+v", got)
	}
	if got := stats["dddd"]; got.MemoryPerc != "0.00%" {
		t.Errorf("stats by short id = %+v", got)
	}
}

func TestShortID(t *testing.T) {
	tests := map[string]string{
		"sha256:0123456789abcdef": "0123456789ab",
		"0123456789abcdef":        "0123456789ab",
		"abc":                     "abc",
		"":                        "",
	}
	for in, want := range tests {
		if got := ShortID(in); got != want {
			t.Errorf("ShortID(%q) = %q, want %q", in, got, want)
		}
	}
}
