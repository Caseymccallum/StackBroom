package wsl

import (
	"strings"
	"testing"
)

func TestParseDistributionList(t *testing.T) {
	out := "  NAME              STATE           VERSION\r\n" +
		"* Ubuntu-22.04      Running         2\r\n" +
		"  docker-desktop    Stopped         2\r\n" +
		"  My Dev Box        Stopped         2\r\n"

	dists, warnings := ParseDistributionList(out)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(dists) != 3 {
		t.Fatalf("parsed %d distributions, want 3", len(dists))
	}
	if dists[0].Name != "Ubuntu-22.04" || !dists[0].IsDefault || !dists[0].Running() {
		t.Errorf("first distribution = %+v", dists[0])
	}
	if dists[1].Name != "docker-desktop" || dists[1].IsDefault || dists[1].Running() {
		t.Errorf("second distribution = %+v", dists[1])
	}
	if dists[2].Name != "My Dev Box" {
		t.Errorf("name with spaces = %q", dists[2].Name)
	}
	if dists[0].Version != "2" {
		t.Errorf("version = %q", dists[0].Version)
	}
}

func TestParseDistributionListLocalisedHeader(t *testing.T) {
	// Headers are localised by Windows; rows are parsed positionally so the
	// header language does not matter.
	out := "  NOM               ETAT            VERSION\n" +
		"* Debian            En cours        2\n"
	dists, _ := ParseDistributionList(out)
	if len(dists) != 1 || dists[0].Name != "Debian" || dists[0].Version != "2" {
		t.Fatalf("localised output parsed as %+v", dists)
	}
}

func TestParseDistributionListEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantCount   int
		wantWarning bool
	}{
		{"empty", "", 0, false},
		{"header only", "NAME STATE VERSION\n", 0, false},
		{"garbage", "this is not a distribution list\n", 0, false},
		{
			name:        "no distributions message",
			in:          "Windows Subsystem for Linux has no installed distributions.\n",
			wantCount:   0,
			wantWarning: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dists, warnings := ParseDistributionList(tc.in)
			if len(dists) != tc.wantCount {
				t.Errorf("count = %d, want %d", len(dists), tc.wantCount)
			}
			if tc.wantWarning && len(warnings) == 0 {
				t.Error("expected a warning")
			}
		})
	}
}

func TestParseDistributionListKeepsUnknownStates(t *testing.T) {
	dists, _ := ParseDistributionList("* Ubuntu  Running  2\n")
	if len(dists) != 1 {
		t.Fatalf("parsed %d distributions", len(dists))
	}
	if dists[0].State != "Running" {
		t.Errorf("state = %q", dists[0].State)
	}

	dists, _ = ParseDistributionList("* Ubuntu  Arret  2\n")
	if dists[0].State != "Arret" {
		t.Errorf("unknown state should be preserved, got %q", dists[0].State)
	}
	if strings.Contains(strings.ToLower(dists[0].State), "run") {
		t.Error("unknown state incorrectly considered running")
	}
}
