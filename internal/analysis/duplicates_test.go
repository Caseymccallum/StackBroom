package analysis

import (
	"testing"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func serverProc(pid int32, rt model.Runtime, ports []int, project *model.ProjectRef, memory uint64) model.Process {
	return model.Process{
		PID:             pid,
		Name:            string(rt) + ".exe",
		Runtime:         rt,
		Category:        model.CategoryDevServer,
		DevServerKind:   "test server",
		Ports:           ports,
		Project:         project,
		WorkingSetBytes: memory,
		MemoryKnown:     true,
	}
}

func TestFindDuplicatesByPort(t *testing.T) {
	procs := []model.Process{
		serverProc(100, model.RuntimeNode, []int{5173}, nil, 100<<20),
		serverProc(200, model.RuntimeNode, []int{5173}, nil, 50<<20),
	}
	groups := FindDuplicates(procs)
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1: %+v", len(groups), groups)
	}
	g := groups[0]
	if g.Kind != model.DuplicateByPort || g.Port != 5173 || g.Runtime != model.RuntimeNode {
		t.Errorf("group = %+v", g)
	}
	if len(g.PIDs) != 2 || g.PIDs[0] != 100 || g.PIDs[1] != 200 {
		t.Errorf("pids = %v", g.PIDs)
	}
	if g.MemoryBytes != 150<<20 || g.MemoryKnown != 2 {
		t.Errorf("memory = %d known %d", g.MemoryBytes, g.MemoryKnown)
	}
	if g.Confidence != model.Inferred {
		t.Errorf("confidence = %q, want inferred", g.Confidence)
	}
	if len(g.Evidence) != 2 {
		t.Errorf("evidence = %v", g.Evidence)
	}
}

func TestFindDuplicatesByProject(t *testing.T) {
	ref := &model.ProjectRef{Root: `C:\proj`, Name: "proj", Marker: "package.json", Confidence: model.Confirmed}
	procs := []model.Process{
		serverProc(1, model.RuntimeNode, []int{5173}, ref, 10),
		serverProc(2, model.RuntimeNode, []int{5174}, ref, 20),
	}
	groups := FindDuplicates(procs)
	if len(groups) != 1 {
		t.Fatalf("groups = %+v, want one project group", groups)
	}
	if groups[0].Kind != model.DuplicateByProject || groups[0].ProjectRoot != `C:\proj` {
		t.Errorf("group = %+v", groups[0])
	}
}

func TestFindDuplicatesRequiresRealEvidence(t *testing.T) {
	confirmed := &model.ProjectRef{Root: `C:\proj`, Name: "proj", Confidence: model.Confirmed}
	inferred := &model.ProjectRef{Root: `C:\other`, Name: "other", Confidence: model.Inferred}

	tests := []struct {
		name  string
		procs []model.Process
	}{
		{
			name: "single server is not a duplicate",
			procs: []model.Process{
				serverProc(1, model.RuntimeNode, []int{5173}, nil, 10),
			},
		},
		{
			name: "different runtimes on the same port",
			procs: []model.Process{
				serverProc(1, model.RuntimeNode, []int{8000}, nil, 10),
				serverProc(2, model.RuntimePython, []int{8000}, nil, 10),
			},
		},
		{
			name: "dev tools are not servers",
			procs: []model.Process{
				serverProc(1, model.RuntimeNode, []int{5173}, nil, 10),
				{PID: 2, Runtime: model.RuntimeNode, Category: model.CategoryDevTool, Ports: []int{5173}},
			},
		},
		{
			name: "inferred project is not enough",
			procs: []model.Process{
				serverProc(1, model.RuntimeNode, []int{5173}, inferred, 10),
				serverProc(2, model.RuntimeNode, []int{5174}, inferred, 10),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if groups := FindDuplicates(tc.procs); len(groups) != 0 {
				t.Fatalf("unexpected duplicate groups: %+v", groups)
			}
		})
	}

	// Two servers on the same port and the same confirmed project are reported
	// once, by port, rather than twice.
	procs := []model.Process{
		serverProc(1, model.RuntimeNode, []int{5173}, confirmed, 10),
		serverProc(2, model.RuntimeNode, []int{5173}, confirmed, 10),
	}
	groups := FindDuplicates(procs)
	if len(groups) != 1 || groups[0].Kind != model.DuplicateByPort {
		t.Fatalf("groups = %+v, want a single port group", groups)
	}
}
