package analysis

import (
	"strings"
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
		RequestedPorts:  ports,
		Project:         project,
		WorkingSetBytes: memory,
		MemoryKnown:     true,
	}
}

func listeningProc(pid int32, rt model.Runtime, requested, listening []int, ownership model.Confidence) model.Process {
	p := serverProc(pid, rt, requested, nil, 10<<20)
	p.ListeningPorts = listening
	p.PortOwnership = ownership
	return p
}

func listener(pid int32, port int, address, family string, ownership model.Confidence) model.TCPListener {
	return model.TCPListener{
		Protocol: "tcp", Family: family, Address: address, Port: port,
		PID: pid, Ownership: ownership, Wildcard: address == "0.0.0.0" || address == "::",
	}
}

func availableNetwork(listeners ...model.TCPListener) model.NetworkInfo {
	return model.NetworkInfo{Available: true, IPv4: true, IPv6: true, Listeners: listeners}
}

func TestFindDuplicatesByPortWithoutListeners(t *testing.T) {
	procs := []model.Process{
		serverProc(100, model.RuntimeNode, []int{5173}, nil, 100<<20),
		serverProc(200, model.RuntimeNode, []int{5173}, nil, 50<<20),
	}
	groups := FindDuplicates(procs, availableNetwork())
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
	// Two command-line requests never prove a duplicate or a binding.
	if g.Confidence != model.Inferred {
		t.Errorf("confidence = %q, want inferred", g.Confidence)
	}
	if len(g.Listeners) != 0 {
		t.Errorf("listeners = %+v, want none", g.Listeners)
	}
	if !evidenceContains(g, "No listener was observed on port 5173") {
		t.Errorf("missing no-listener evidence: %v", g.Evidence)
	}
	if len(g.Evidence) < 2 {
		t.Errorf("evidence = %v", g.Evidence)
	}
}

func TestFindDuplicatesOneObservedListener(t *testing.T) {
	procs := []model.Process{
		listeningProc(100, model.RuntimeNode, []int{5173}, []int{5173}, model.Confirmed),
		serverProc(200, model.RuntimeNode, []int{5173}, nil, 50<<20),
	}
	groups := FindDuplicates(procs, availableNetwork(
		listener(100, 5173, "127.0.0.1", "ipv4", model.Confirmed)))
	if len(groups) != 1 {
		t.Fatalf("groups = %+v", groups)
	}
	g := groups[0]
	if g.Confidence != model.Inferred {
		t.Errorf("confidence = %q, want inferred (one binding does not prove redundancy)", g.Confidence)
	}
	if !evidenceContains(g, "observed listening on 127.0.0.1:5173") {
		t.Errorf("missing binding evidence: %v", g.Evidence)
	}
	if !evidenceContains(g, "no listener owned by this PID was observed") {
		t.Errorf("missing other-PID evidence: %v", g.Evidence)
	}
}

func TestFindDuplicatesTwoObservedListeners(t *testing.T) {
	procs := []model.Process{
		listeningProc(100, model.RuntimeNode, []int{5173}, []int{5173}, model.Confirmed),
		listeningProc(200, model.RuntimeNode, []int{5173}, []int{5173}, model.Confirmed),
	}
	groups := FindDuplicates(procs, availableNetwork(
		listener(100, 5173, "127.0.0.1", "ipv4", model.Confirmed),
		listener(200, 5173, "::1", "ipv6", model.Confirmed)))
	if len(groups) != 1 {
		t.Fatalf("groups = %+v", groups)
	}
	g := groups[0]
	if g.Confidence != model.Confirmed {
		t.Errorf("confidence = %q, want confirmed", g.Confidence)
	}
	if len(g.Listeners) != 2 {
		t.Fatalf("listeners = %+v", g.Listeners)
	}
	if !evidenceContains(g, "127.0.0.1:5173 (ipv4)") || !evidenceContains(g, "[::1]:5173 (ipv6)") {
		t.Errorf("missing address family evidence: %v", g.Evidence)
	}
}

func TestFindDuplicatesSamePortDifferentAddresses(t *testing.T) {
	procs := []model.Process{
		listeningProc(1, model.RuntimeNode, nil, []int{3000}, model.Confirmed),
		listeningProc(2, model.RuntimeNode, nil, []int{3000}, model.Confirmed),
	}
	groups := FindDuplicates(procs, availableNetwork(
		listener(1, 3000, "127.0.0.1", "ipv4", model.Confirmed),
		listener(2, 3000, "0.0.0.0", "ipv4", model.Confirmed)))
	if len(groups) != 1 {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].Confidence != model.Confirmed {
		t.Errorf("confidence = %q, want confirmed", groups[0].Confidence)
	}
	if !evidenceContains(groups[0], "no matching requested-port argument was found") {
		t.Errorf("listener-without-request case not explained: %v", groups[0].Evidence)
	}
}

func TestFindDuplicatesOtherProcessHoldsPort(t *testing.T) {
	procs := []model.Process{
		serverProc(100, model.RuntimeNode, []int{8000}, nil, 10),
		serverProc(200, model.RuntimeNode, []int{8000}, nil, 10),
		{PID: 999, Name: "python.exe", Runtime: model.RuntimePython, Category: model.CategoryDevTool},
	}
	groups := FindDuplicates(procs, availableNetwork(
		listener(999, 8000, "0.0.0.0", "ipv4", model.Confirmed)))
	if len(groups) != 1 {
		t.Fatalf("groups = %+v", groups)
	}
	g := groups[0]
	if !evidenceContains(g, "also held by PID 999 (python.exe)") {
		t.Errorf("missing other-owner evidence: %v", g.Evidence)
	}
	if g.Confidence != model.Inferred {
		t.Errorf("confidence = %q, want inferred", g.Confidence)
	}
}

func TestFindDuplicatesUnknownOwnership(t *testing.T) {
	procs := []model.Process{
		serverProc(100, model.RuntimeNode, []int{5173}, nil, 10),
		serverProc(200, model.RuntimeNode, []int{5173}, nil, 10),
	}
	groups := FindDuplicates(procs, availableNetwork(
		listener(200, 5173, "0.0.0.0", "ipv4", model.Unknown)))
	if len(groups) != 1 {
		t.Fatalf("groups = %+v", groups)
	}
	g := groups[0]
	if g.Confidence != model.Inferred {
		t.Errorf("confidence = %q, want inferred", g.Confidence)
	}
	if !evidenceContains(g, "is unknown") {
		t.Errorf("unknown ownership not explained: %v", g.Evidence)
	}
	if len(g.Listeners) != 1 || g.Listeners[0].Ownership != model.Unknown {
		t.Errorf("listener refs = %+v", g.Listeners)
	}
}

func TestFindDuplicatesNetworkUnavailable(t *testing.T) {
	procs := []model.Process{
		serverProc(1, model.RuntimeNode, []int{5173}, nil, 10),
		serverProc(2, model.RuntimeNode, []int{5173}, nil, 10),
	}
	groups := FindDuplicates(procs, model.NetworkInfo{
		Available: false, Reason: "access denied", Listeners: []model.TCPListener{},
	})
	if len(groups) != 1 {
		t.Fatalf("groups = %+v", groups)
	}
	if !evidenceContains(groups[0], "TCP listener inspection was unavailable") {
		t.Errorf("availability limitation missing: %v", groups[0].Evidence)
	}
	if groups[0].Confidence != model.Inferred {
		t.Errorf("confidence = %q, want inferred", groups[0].Confidence)
	}
}

func TestFindDuplicatesByProject(t *testing.T) {
	ref := &model.ProjectRef{Root: `C:\proj`, Name: "proj", Marker: "package.json", Confidence: model.Confirmed}
	procs := []model.Process{
		serverProc(1, model.RuntimeNode, []int{5173}, ref, 10),
		serverProc(2, model.RuntimeNode, []int{5174}, ref, 20),
	}
	groups := FindDuplicates(procs, availableNetwork())
	if len(groups) != 1 {
		t.Fatalf("groups = %+v, want one project group", groups)
	}
	if groups[0].Kind != model.DuplicateByProject || groups[0].ProjectRoot != `C:\proj` {
		t.Errorf("group = %+v", groups[0])
	}
}

func evidenceContains(g model.DuplicateGroup, substr string) bool {
	for _, line := range g.Evidence {
		if strings.Contains(line, substr) {
			return true
		}
	}
	return false
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
				{PID: 2, Runtime: model.RuntimeNode, Category: model.CategoryDevTool, RequestedPorts: []int{5173}},
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
			if groups := FindDuplicates(tc.procs, availableNetwork()); len(groups) != 0 {
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
	groups := FindDuplicates(procs, availableNetwork())
	if len(groups) != 1 || groups[0].Kind != model.DuplicateByPort {
		t.Fatalf("groups = %+v, want a single port group", groups)
	}
}
