package analysis

import (
	"strings"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

var testNow = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func ptrU64(v uint64) *uint64        { return &v }
func ptrTime(t time.Time) *time.Time { return &t }

// assertContract enforces the six-part recommendation contract and the safety
// property that no recommendation promises an automatic action.
func assertContract(t *testing.T, recs []model.Recommendation) {
	t.Helper()
	for _, r := range recs {
		if r.ID == "" || r.Kind == "" || r.Title == "" {
			t.Errorf("recommendation lacks identity: %+v", r)
		}
		if r.Detected == "" {
			t.Errorf("%s: missing 'detected' text", r.ID)
		}
		if r.WhyUnnecessary == "" {
			t.Errorf("%s: missing 'why' text", r.ID)
		}
		if len(r.Evidence) == 0 {
			t.Errorf("%s: missing evidence", r.ID)
		}
		if r.Resource == "" {
			t.Errorf("%s: missing resource description", r.ID)
		}
		if len(r.Review) == 0 {
			t.Errorf("%s: missing review steps", r.ID)
		}
		if r.RiskDetail == "" {
			t.Errorf("%s: missing risk detail", r.ID)
		}
		switch r.Risk {
		case model.RiskLow, model.RiskMedium, model.RiskHigh:
		default:
			t.Errorf("%s: invalid risk %q", r.ID, r.Risk)
		}

		text := strings.ToLower(strings.Join(append([]string{
			r.Title, r.Detected, r.WhyUnnecessary, r.Resource, r.RiskDetail,
		}, r.Evidence...), " "))
		for _, forbidden := range []string{
			"stackbroom will stop", "stackbroom will delete", "stackbroom has stopped",
			"automatically stop", "automatically delete", "will be stopped", "will be removed",
		} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s: recommendation implies an automatic action (%q)", r.ID, forbidden)
			}
		}
	}
}

func TestRecommendDuplicateServer(t *testing.T) {
	dups := []model.DuplicateGroup{{
		Kind:        model.DuplicateByPort,
		Key:         "node on port 5173",
		Runtime:     model.RuntimeNode,
		Port:        5173,
		PIDs:        []int32{100, 200},
		MemoryBytes: 150 << 20,
		MemoryKnown: 2,
		Confidence:  model.Inferred,
		Evidence:    []string{"PID 100 (node, vite) requests port 5173", "PID 200 (node, vite) requests port 5173"},
	}}

	recs := Recommend(nil, model.DockerInfo{}, model.WSLInfo{}, dups, testNow, DefaultOptions())
	if len(recs) != 1 {
		t.Fatalf("recommendations = %d, want 1", len(recs))
	}
	assertContract(t, recs)

	rec := recs[0]
	if rec.Kind != KindDuplicateServer || rec.Risk != model.RiskMedium {
		t.Errorf("rec = %+v", rec)
	}
	if !strings.Contains(rec.Title, "5173") {
		t.Errorf("title does not name the port: %q", rec.Title)
	}
	if len(rec.Review) == 0 || !strings.Contains(strings.Join(rec.Review, " "), "Get-NetTCPConnection") {
		t.Errorf("review should explain how to check the port: %v", rec.Review)
	}
	if len(rec.Subjects) != 2 {
		t.Errorf("subjects = %+v", rec.Subjects)
	}
	if !strings.Contains(strings.Join(rec.Evidence, " "), "separate evidence") {
		t.Errorf("evidence should separate requested ports from observed listeners: %v", rec.Evidence)
	}
	if !strings.Contains(rec.Title, "no listener was observed") {
		t.Errorf("title should not claim a binding when none was observed: %q", rec.Title)
	}
}

func TestRecommendDuplicateServerListenerEvidence(t *testing.T) {
	tests := []struct {
		name      string
		group     model.DuplicateGroup
		wantTitle string
	}{
		{
			name: "one observed listener",
			group: model.DuplicateGroup{
				Kind: model.DuplicateByPort, Runtime: model.RuntimeNode, Port: 5173,
				PIDs: []int32{100, 200}, Confidence: model.Inferred,
				Evidence: []string{"PID 100 is observed listening on 127.0.0.1:5173"},
				Listeners: []model.ListenerRef{
					{Family: "ipv4", Address: "127.0.0.1", Port: 5173, PID: 100, Ownership: model.Confirmed},
				},
			},
			wantTitle: "only PID 100 is observed listening",
		},
		{
			name: "two observed listeners",
			group: model.DuplicateGroup{
				Kind: model.DuplicateByPort, Runtime: model.RuntimeNode, Port: 5173,
				PIDs: []int32{100, 200}, Confidence: model.Confirmed,
				Listeners: []model.ListenerRef{
					{Family: "ipv4", Address: "127.0.0.1", Port: 5173, PID: 100, Ownership: model.Confirmed},
					{Family: "ipv6", Address: "::1", Port: 5173, PID: 200, Ownership: model.Confirmed},
				},
			},
			wantTitle: "are observed listening",
		},
		{
			name: "another process holds the port",
			group: model.DuplicateGroup{
				Kind: model.DuplicateByPort, Runtime: model.RuntimeNode, Port: 5173,
				PIDs: []int32{100, 200}, Confidence: model.Inferred,
				Listeners: []model.ListenerRef{
					{Family: "ipv4", Address: "0.0.0.0", Port: 5173, PID: 999, Ownership: model.Confirmed},
				},
			},
			wantTitle: "held by another process",
		},
		{
			name: "ownership uncertain",
			group: model.DuplicateGroup{
				Kind: model.DuplicateByPort, Runtime: model.RuntimeNode, Port: 5173,
				PIDs: []int32{100, 200}, Confidence: model.Inferred,
				Listeners: []model.ListenerRef{
					{Family: "ipv4", Address: "127.0.0.1", Port: 5173, PID: 200, Ownership: model.Unknown},
				},
			},
			wantTitle: "ownership is uncertain",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recs := Recommend(nil, model.DockerInfo{}, model.WSLInfo{}, []model.DuplicateGroup{tc.group}, testNow, DefaultOptions())
			if len(recs) != 1 {
				t.Fatalf("recommendations = %+v", recs)
			}
			rec := recs[0]
			assertContract(t, recs)
			if !strings.Contains(rec.Title, tc.wantTitle) {
				t.Errorf("title = %q, want %q", rec.Title, tc.wantTitle)
			}
			listenerSubjects := 0
			for _, subject := range rec.Subjects {
				if subject.Kind == "listener" {
					listenerSubjects++
				}
			}
			if listenerSubjects != len(tc.group.Listeners) {
				t.Errorf("listener subjects = %d, want %d", listenerSubjects, len(tc.group.Listeners))
			}
			text := strings.ToLower(strings.Join(append([]string{rec.Title, rec.Detected, rec.WhyUnnecessary}, rec.Evidence...), " "))
			for _, forbidden := range []string{"failed to bind", "crashed", "orphaned", "process exited"} {
				if strings.Contains(text, forbidden) {
					t.Errorf("recommendation asserts an unsupported conclusion %q: %s", forbidden, text)
				}
			}
		})
	}
}

func TestRecommendLongRunningServer(t *testing.T) {
	project := &model.ProjectRef{
		Root: `C:\proj`, Name: "proj", Marker: "package.json",
		Confidence: model.Confirmed, Evidence: []string{"package.json found in C:\\proj"},
	}
	base := model.Process{
		PID: 9, Name: "node.exe", Runtime: model.RuntimeNode,
		Category: model.CategoryDevServer, DevServerKind: "vite",
		RequestedPorts: []int{5173}, Project: project,
		WorkingSetBytes: 200 << 20, MemoryKnown: true,
	}

	tests := []struct {
		name      string
		mutate    func(*model.Process)
		wantCount int
	}{
		{"old confirmed project server", func(p *model.Process) {
			p.StartTimeKnown = true
			p.StartedAt = ptrTime(testNow.Add(-10 * time.Hour))
		}, 1},
		{"recent server", func(p *model.Process) {
			p.StartTimeKnown = true
			p.StartedAt = ptrTime(testNow.Add(-1 * time.Hour))
		}, 0},
		{"unknown start time", func(p *model.Process) {}, 0},
		{"inferred project", func(p *model.Process) {
			p.Project = &model.ProjectRef{Root: `C:\proj`, Name: "proj", Confidence: model.Inferred}
			p.StartTimeKnown = true
			p.StartedAt = ptrTime(testNow.Add(-10 * time.Hour))
		}, 0},
		{"no project", func(p *model.Process) {
			p.Project = nil
			p.StartTimeKnown = true
			p.StartedAt = ptrTime(testNow.Add(-10 * time.Hour))
		}, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			tc.mutate(&p)
			recs := Recommend([]model.Process{p}, model.DockerInfo{}, model.WSLInfo{}, nil, testNow, DefaultOptions())
			if len(recs) != tc.wantCount {
				t.Fatalf("recommendations = %d, want %d: %+v", len(recs), tc.wantCount, recs)
			}
			assertContract(t, recs)
			if tc.wantCount == 1 {
				if recs[0].Kind != KindLongRunningServer || recs[0].Risk != model.RiskMedium {
					t.Errorf("rec = %+v", recs[0])
				}
				if !strings.Contains(recs[0].WhyUnnecessary, "not evidence") {
					t.Errorf("age caveat missing: %q", recs[0].WhyUnnecessary)
				}
			}
		})
	}
}

func TestRecommendNothingWhenNothingIsKnown(t *testing.T) {
	recs := Recommend(nil, model.DockerInfo{}, model.WSLInfo{}, nil, testNow, DefaultOptions())
	if len(recs) != 0 {
		t.Fatalf("unexpected recommendations: %+v", recs)
	}
}
