package scan

import (
	"context"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/winproc"
)

func runNetworkScan(t *testing.T, procs []model.Process, creation map[int32]time.Time,
	identity IdentityVerifier, listeners []model.TCPListener) model.Snapshot {
	t.Helper()
	fixed := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	scanner := New(Options{
		Now:       func() time.Time { return fixed },
		Processes: fakeProcesses{result: winproc.Result{Processes: procs, CreationTimes: creation}},
		Network: fakeNetwork{info: model.NetworkInfo{
			Available: true, IPv4: true, IPv6: true,
			SampledAt: fixed.Add(time.Second),
			Listeners: listeners,
		}},
		Docker:   fakeDocker{},
		WSL:      fakeWSL{},
		Identity: identity,
	})
	return scanner.Run(context.Background())
}

func nodeServer(pid int32, project *model.ProjectRef) model.Process {
	return model.Process{
		PID: pid, Name: "node.exe", Runtime: model.RuntimeNode,
		Category: model.CategoryDevServer, DevServerKind: "vite",
		Project: project,
	}
}

func TestNetworkAssociationConfirmed(t *testing.T) {
	created := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	project := &model.ProjectRef{Root: `C:\proj`, Name: "proj", Confidence: model.Confirmed}
	snapshot := runNetworkScan(t,
		[]model.Process{nodeServer(100, project)},
		map[int32]time.Time{100: created},
		func(pid int32) (time.Time, bool) { return created, pid == 100 },
		[]model.TCPListener{{
			Protocol: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 5173,
			PID: 100, Ownership: model.Unknown,
		}})

	if len(snapshot.Network.Listeners) != 1 {
		t.Fatalf("listeners = %+v", snapshot.Network.Listeners)
	}
	listener := snapshot.Network.Listeners[0]
	if listener.Ownership != model.Confirmed {
		t.Errorf("ownership = %q, want confirmed", listener.Ownership)
	}
	if listener.ProcessName != "node.exe" || listener.ProjectName != "proj" {
		t.Errorf("association = %+v", listener)
	}
	if snapshot.Processes[0].PortOwnership != model.Confirmed {
		t.Errorf("process ownership = %q", snapshot.Processes[0].PortOwnership)
	}
	if len(snapshot.Processes[0].ListeningPorts) != 1 || snapshot.Processes[0].ListeningPorts[0] != 5173 {
		t.Errorf("listening ports = %v", snapshot.Processes[0].ListeningPorts)
	}
	if snapshot.Summary.TCPListenerCount != 1 {
		t.Errorf("summary listener count = %d", snapshot.Summary.TCPListenerCount)
	}
}

func TestNetworkAssociationInferredWithoutCreationTime(t *testing.T) {
	called := false
	snapshot := runNetworkScan(t,
		[]model.Process{nodeServer(100, nil)},
		nil,
		func(pid int32) (time.Time, bool) { called = true; return time.Time{}, false },
		[]model.TCPListener{{
			Protocol: "tcp", Family: "ipv4", Address: "0.0.0.0", Port: 3000, PID: 100,
		}})

	if called {
		t.Error("identity verifier should not run without a snapshot creation time")
	}
	listener := snapshot.Network.Listeners[0]
	if listener.Ownership != model.Inferred {
		t.Errorf("ownership = %q, want inferred", listener.Ownership)
	}
	if snapshot.Processes[0].PortOwnership != model.Inferred {
		t.Errorf("process ownership = %q", snapshot.Processes[0].PortOwnership)
	}
	if !hasWarning(snapshot.Warnings, "Ownership is inferred") {
		t.Errorf("warnings = %v", snapshot.Warnings)
	}
}

func TestNetworkAssociationDetectsPIDReuse(t *testing.T) {
	created := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	snapshot := runNetworkScan(t,
		[]model.Process{nodeServer(100, nil)},
		map[int32]time.Time{100: created},
		func(pid int32) (time.Time, bool) { return created.Add(time.Hour), true },
		[]model.TCPListener{{
			Protocol: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 5173, PID: 100,
		}})

	listener := snapshot.Network.Listeners[0]
	if listener.Ownership != model.Unknown {
		t.Errorf("ownership = %q, want unknown", listener.Ownership)
	}
	if listener.ProcessName != "" || listener.ProjectName != "" {
		t.Errorf("reused PID kept an association: %+v", listener)
	}
	if len(snapshot.Processes[0].ListeningPorts) != 0 {
		t.Errorf("listening ports = %v, want none", snapshot.Processes[0].ListeningPorts)
	}
	if !hasWarning(snapshot.Warnings, "changed identity") {
		t.Errorf("warnings = %v", snapshot.Warnings)
	}
}

func TestNetworkAssociationMissingProcess(t *testing.T) {
	snapshot := runNetworkScan(t, nil, nil, nil,
		[]model.TCPListener{{
			Protocol: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 5173, PID: 999,
		}})
	listener := snapshot.Network.Listeners[0]
	if listener.Ownership != model.Unknown || listener.ProcessName != "" {
		t.Errorf("listener = %+v", listener)
	}
	if !hasWarning(snapshot.Warnings, "not present in the process snapshot") {
		t.Errorf("warnings = %v", snapshot.Warnings)
	}
}

func TestNetworkAssociationReverificationFailureIsInferred(t *testing.T) {
	created := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	snapshot := runNetworkScan(t,
		[]model.Process{nodeServer(100, nil)},
		map[int32]time.Time{100: created},
		func(pid int32) (time.Time, bool) { return time.Time{}, false },
		[]model.TCPListener{{
			Protocol: "tcp", Family: "ipv6", Address: "::1", Port: 5173, PID: 100,
		}})
	listener := snapshot.Network.Listeners[0]
	if listener.Ownership != model.Inferred {
		t.Errorf("ownership = %q, want inferred", listener.Ownership)
	}
	if listener.ProcessName != "node.exe" {
		t.Errorf("process name = %q", listener.ProcessName)
	}
	if !hasWarning(snapshot.Warnings, "could not be re-read") {
		t.Errorf("warnings = %v", snapshot.Warnings)
	}
}

func TestNetworkAssociationMultipleListeners(t *testing.T) {
	created := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	snapshot := runNetworkScan(t,
		[]model.Process{nodeServer(100, nil)},
		map[int32]time.Time{100: created},
		func(pid int32) (time.Time, bool) { return created, true },
		[]model.TCPListener{
			{Protocol: "tcp", Family: "ipv6", Address: "::", Port: 443, PID: 100, Wildcard: true},
			{Protocol: "tcp", Family: "ipv4", Address: "0.0.0.0", Port: 80, PID: 100, Wildcard: true},
			{Protocol: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 80, PID: 100},
		})
	ports := snapshot.Processes[0].ListeningPorts
	if len(ports) != 2 || ports[0] != 80 || ports[1] != 443 {
		t.Errorf("listening ports = %v", ports)
	}
	if snapshot.Processes[0].PortOwnership != model.Confirmed {
		t.Errorf("ownership = %q", snapshot.Processes[0].PortOwnership)
	}
}

func TestNetworkAssociationWarningCap(t *testing.T) {
	var listeners []model.TCPListener
	for i := 0; i < 20; i++ {
		listeners = append(listeners, model.TCPListener{
			Protocol: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 1000 + i, PID: int32(9000 + i),
		})
	}
	snapshot := runNetworkScan(t, nil, nil, nil, listeners)
	if len(snapshot.Network.Warnings) > maxAssociationWarnings+1 {
		t.Errorf("warnings = %d, want at most %d", len(snapshot.Network.Warnings), maxAssociationWarnings+1)
	}
	if !hasWarning(snapshot.Network.Warnings, "additional listener ownership notes") {
		t.Errorf("missing truncation note: %v", snapshot.Network.Warnings)
	}
}
