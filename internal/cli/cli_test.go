package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/scan"
)

func ptrU64(v uint64) *uint64 { return &v }

func fixtureSnapshot() model.Snapshot {
	generated := time.Date(2026, 10, 27, 12, 0, 0, 0, time.Local)
	started := generated.Add(-9 * time.Hour)
	project := &model.ProjectRef{
		Root: `C:\Web Apps\demo`, Name: "demo", Marker: "package.json",
		Confidence: model.Confirmed, Evidence: []string{"package.json found in C:\\Web Apps\\demo"},
	}
	rec := model.Recommendation{
		ID:             "duplicate-dev-server-1",
		Kind:           "duplicate-dev-server",
		Title:          "Two node development servers target port 5173; only PID 100 is observed listening",
		Detected:       "2 node development server processes are associated with port 5173 (PIDs 100, 200); only PID 100 has an observed listener.",
		WhyUnnecessary: "One process was observed bound; the other requested the port but no listener owned by its PID was observed.",
		Evidence:       []string{"PID 100 (node, vite) requests port 5173; observed listening on 127.0.0.1:5173 (ipv4)"},
		Resource:       "150 MB working set.",
		ResourceBytes:  150 << 20,
		Review:         []string{"Check the listening process with Get-NetTCPConnection."},
		Risk:           model.RiskMedium,
		RiskDetail:     "Stopping the wrong process ends an active session.",
		Confidence:     model.Inferred,
		Subjects:       []model.Subject{{Kind: "process", ID: "100"}},
	}

	return model.Snapshot{
		SchemaVersion: model.SchemaVersion,
		ReadOnly:      true,
		Tool:          model.Tool{Name: "stackbroom", Version: "0.1.0-test"},
		GeneratedAt:   generated,
		DurationMS:    1234,
		Host: model.Host{
			OS: "windows", Arch: "amd64", Hostname: "TEST-PC",
			TotalMemoryBytes: 16 << 30, AvailableMemoryBytes: 8 << 30, MemoryKnown: true,
		},
		Processes: []model.Process{
			{
				PID: 100, Name: "node.exe", Runtime: model.RuntimeNode,
				Category: model.CategoryDevServer, DevServerKind: "vite",
				RequestedPorts: []int{5173}, ListeningPorts: []int{5173}, PortOwnership: model.Confirmed,
				Project: project, WorkingSetBytes: 150 << 20, MemoryKnown: true,
				StartTimeKnown: true, StartedAt: &started, CommandLineKnown: true,
				ParentName: "Code.exe",
			},
			{
				PID: 300, Name: "python.exe", Runtime: model.RuntimePython,
				Category: model.CategoryDevTool, MemoryKnown: false, CommandLineKnown: true,
			},
		},
		Network: model.NetworkInfo{
			Available: true, IPv4: true, IPv6: true,
			SampledAt: generated.Add(-time.Second),
			Listeners: []model.TCPListener{
				{Protocol: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 5173, PID: 100, ProcessName: "node.exe", ProjectName: "demo", Ownership: model.Confirmed},
				{Protocol: "tcp", Family: "ipv6", Address: "::1", Port: 3000, PID: 200, ProcessName: "node.exe", Ownership: model.Confirmed},
				{Protocol: "tcp", Family: "ipv4", Address: "0.0.0.0", Port: 135, PID: 4, ProcessName: "svchost.exe", Wildcard: true, Ownership: model.Unknown},
			},
		},
		Docker: model.DockerInfo{
			Status: model.DockerStatus{Available: true, ServerVersion: "29.4.0", Context: "desktop-linux"},
			Containers: []model.DockerContainer{
				{Name: "old-db", Image: "mysql:8.0", State: "exited", Status: "Exited (0) 5 months ago", StoppedForDays: 150, StoppedForText: "5 months", ComposeProject: "demo"},
			},
			DiskUsage: []model.DockerDiskUsage{
				{Type: "Images", Total: 25, Active: 5, SizeText: "15.64GB", Size: ptrU64(15640000000), ReclaimableText: "12.41GB (79%)", Reclaimable: ptrU64(12410000000)},
				{Type: "Build Cache", Total: 21, Active: 0, SizeText: "630.1MB", Size: ptrU64(630100000), ReclaimableText: "42.68MB", Reclaimable: ptrU64(42680000)},
			},
			Images: []model.DockerImage{
				{Repository: "public.ecr.aws/supabase/studio", Tag: "2026.10.19", SizeText: "1.75GB", Size: ptrU64(1875 << 20)},
			},
		},
		WSL: model.WSLInfo{
			Available:     true,
			Distributions: []model.WSLDistribution{{Name: "Ubuntu", State: "Running", Version: "2", IsDefault: true}},
			VMMemoryBytes: 473 << 20, VMMemoryKnown: true, VMProcessName: "vmmem",
			VMNote: "vmmem covers all running distributions",
		},
		Summary: model.Summary{
			TotalProcesses: 2, DevProcessCount: 2, DevServerCount: 1,
			DevMemoryBytes: 150 << 20, DevMemoryKnown: 1,
			TopProcesses:   []model.TopProcess{{PID: 100, Name: "node.exe", MemoryBytes: 150 << 20, MemoryKnown: true, Category: model.CategoryDevServer, ProjectName: "demo"}},
			TopMemoryBytes: 150 << 20,
			RuntimeTotals:  []model.RuntimeTotal{{Runtime: model.RuntimeNode, ProcessCount: 1, MemoryBytes: 150 << 20, MemoryKnownCount: 1}},
			DockerRunning:  0, DockerStopped: 1,
			DockerTotalBytes: 16270 << 20, DockerTotalKnown: true,
			DockerReclaimableBytes: 12410 << 20, DockerReclaimKnown: true,
			DockerSizeRowsKnown: 2, DockerSizeRowsTotal: 2,
			WSLRunningDistributions: 1, WSLVMMemoryBytes: 473 << 20, WSLVMMemoryKnown: true,
			TCPListenerCount: 3,
		},
		Duplicates: []model.DuplicateGroup{{
			Kind: model.DuplicateByPort, Key: "node on port 5173", Runtime: model.RuntimeNode,
			Port: 5173, PIDs: []int32{100, 200}, MemoryBytes: 150 << 20, MemoryKnown: 1,
			Confidence: model.Inferred, Evidence: []string{"PID 100 (node, vite) requests port 5173"},
			Listeners: []model.ListenerRef{
				{Family: "ipv4", Address: "127.0.0.1", Port: 5173, PID: 100, Ownership: model.Confirmed},
			},
		}},
		Recommendations: []model.Recommendation{rec},
		Warnings:        []string{"Docker: example note."},
	}
}

func newTestApp(snapshot model.Snapshot) (*App, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := New(out, errOut)
	app.Scan = func(ctx context.Context, opts scan.Options) model.Snapshot {
		_ = ctx
		_ = opts
		return snapshot
	}
	return app, out, errOut
}

func TestVersionCommand(t *testing.T) {
	app, out, _ := newTestApp(fixtureSnapshot())
	if code := app.Run(context.Background(), []string{"version"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "stackbroom") {
		t.Errorf("version output = %q", out.String())
	}
}

func TestUsageAndUnknownCommand(t *testing.T) {
	app, out, errOut := newTestApp(fixtureSnapshot())
	if code := app.Run(context.Background(), nil); code != ExitUsage {
		t.Errorf("no-args exit code = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("usage output = %q", out.String())
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"destroy"}); code != ExitUsage {
		t.Errorf("unknown command exit code = %d", code)
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Errorf("error output = %q", errOut.String())
	}
}

func TestScanFlagError(t *testing.T) {
	app, _, errOut := newTestApp(fixtureSnapshot())
	if code := app.Run(context.Background(), []string{"scan", "--not-a-flag"}); code != ExitUsage {
		t.Fatalf("exit code = %d", code)
	}
	if errOut.Len() == 0 {
		t.Error("expected an error message")
	}
}

func TestScanTextOutput(t *testing.T) {
	app, out, _ := newTestApp(fixtureSnapshot())
	if code := app.Run(context.Background(), []string{"scan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	text := out.String()

	for _, want := range []string{
		"read-only scan",
		"RESOURCES",
		"DEVELOPMENT PROCESSES (2)",
		"demo (confirmed)",
		"vite | requests 5173 | listening 5173",
		"TCP LISTENERS (3 observed, 1 development-related)",
		"127.0.0.1",
		"confirmed",
		"Unknown",
		"DOCKER",
		"Storage (Docker's own report",
		"WSL",
		"Ubuntu",
		"POTENTIAL DUPLICATES",
		"REVIEW OPPORTUNITIES",
		"NOTES AND LIMITATIONS",
		"StackBroom is read-only",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("scan output missing %q\n---\n%s", want, text)
		}
	}
}

func TestNetworkSectionOmitsUnrelatedSystemListeners(t *testing.T) {
	app, out, _ := newTestApp(fixtureSnapshot())
	if code := app.Run(context.Background(), []string{"scan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	text := out.String()
	if strings.Contains(text, "svchost.exe") || strings.Contains(text, "0.0.0.0") {
		t.Errorf("system listener leaked into the text table:\n%s", text)
	}
	if !strings.Contains(text, "1 development-related") {
		t.Errorf("development-related count missing:\n%s", text)
	}
}

func TestNetworkNoDevelopmentRelatedListeners(t *testing.T) {
	snapshot := fixtureSnapshot()
	snapshot.Processes = []model.Process{}
	snapshot.Summary.DevProcessCount = 0
	snapshot.Summary.TCPListenerCount = 1
	snapshot.Network.Listeners = []model.TCPListener{{
		Protocol: "tcp", Family: "ipv4", Address: "0.0.0.0", Port: 135, PID: 4,
		ProcessName: "svchost.exe", Wildcard: true, Ownership: model.Inferred,
	}}
	app, out, _ := newTestApp(snapshot)
	if code := app.Run(context.Background(), []string{"scan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	text := out.String()
	if !strings.Contains(text, "None are owned by recognised development processes") {
		t.Errorf("empty development-related set not explained:\n%s", text)
	}
	if !strings.Contains(text, "1 other TCP listener was observed") {
		t.Errorf("full-table count missing:\n%s", text)
	}
}

func TestNetworkUnavailableIsExplicit(t *testing.T) {
	snapshot := fixtureSnapshot()
	snapshot.Network = model.NetworkInfo{
		Available: false, Reason: "access denied while reading the IPV4 table",
		Listeners: []model.TCPListener{},
	}
	snapshot.Summary.TCPListenerCount = 0
	app, out, _ := newTestApp(snapshot)
	if code := app.Run(context.Background(), []string{"scan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	text := out.String()
	if !strings.Contains(text, "Unavailable: access denied") {
		t.Errorf("network availability not stated:\n%s", text)
	}
	if strings.Contains(text, "No TCP listeners were observed") {
		t.Error("unavailable network was reported as an observed empty table")
	}
}

func TestNetworkNoListenersObserved(t *testing.T) {
	snapshot := fixtureSnapshot()
	snapshot.Network.Listeners = []model.TCPListener{}
	snapshot.Summary.TCPListenerCount = 0
	app, out, _ := newTestApp(snapshot)
	if code := app.Run(context.Background(), []string{"scan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "No TCP listeners were observed") {
		t.Errorf("empty listener table not explained:\n%s", out.String())
	}
}

func TestScanJSONOutput(t *testing.T) {
	app, out, _ := newTestApp(fixtureSnapshot())
	if code := app.Run(context.Background(), []string{"scan", "--json"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}

	var got model.Snapshot
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if !got.ReadOnly {
		t.Error("JSON snapshot must be marked read-only")
	}
	if got.SchemaVersion != model.SchemaVersion {
		t.Errorf("schema version = %q", got.SchemaVersion)
	}
	if len(got.Processes) != 2 || len(got.Recommendations) != 1 {
		t.Errorf("decoded snapshot = %d processes, %d recommendations", len(got.Processes), len(got.Recommendations))
	}
	if len(got.Network.Listeners) != 3 || got.Summary.TCPListenerCount != 3 {
		t.Errorf("network = %+v summary count = %d", got.Network.Listeners, got.Summary.TCPListenerCount)
	}
	if len(got.Processes[0].RequestedPorts) != 1 || len(got.Processes[0].ListeningPorts) != 1 {
		t.Errorf("port evidence = requested %v listening %v",
			got.Processes[0].RequestedPorts, got.Processes[0].ListeningPorts)
	}
	body := out.String()
	if !strings.Contains(body, `"requestedPorts"`) || !strings.Contains(body, `"listeningPorts"`) {
		t.Error("schema v3 port field names are missing")
	}
	if strings.Contains(body, `"ports":`) {
		t.Error("deprecated ports field is still emitted")
	}
}

func TestPlanTextOutput(t *testing.T) {
	app, out, _ := newTestApp(fixtureSnapshot())
	if code := app.Run(context.Background(), []string{"plan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	text := out.String()

	for _, want := range []string{
		"review suggestions",
		"1. Two node development servers target port 5173; only PID 100 is observed listening",
		"Risk: medium | Confidence: inferred",
		"Detected:",
		"Evidence:",
		"Resource use:",
		"Review before acting:",
		"Risk of clean-up:",
		"Get-NetTCPConnection",
		"NO CLEANUP WAS PERFORMED",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("plan output missing %q\n---\n%s", want, text)
		}
	}
}

func TestPlanJSONOutput(t *testing.T) {
	app, out, _ := newTestApp(fixtureSnapshot())
	if code := app.Run(context.Background(), []string{"plan", "--json"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}

	var doc model.PlanDocument
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if !doc.ReadOnly {
		t.Error("plan must be read-only")
	}
	if doc.Executes {
		t.Error("plan must never report that it executes cleanup")
	}
	if len(doc.Recommendations) != 1 {
		t.Errorf("recommendations = %d", len(doc.Recommendations))
	}
}

func TestPlanWithNoRecommendations(t *testing.T) {
	snapshot := fixtureSnapshot()
	snapshot.Recommendations = nil
	snapshot.Duplicates = nil

	app, out, _ := newTestApp(snapshot)
	if code := app.Run(context.Background(), []string{"plan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "No suggestions met the evidence thresholds") {
		t.Errorf("plan output = %q", out.String())
	}
}

func TestScanFlagsArePropagated(t *testing.T) {
	out := &bytes.Buffer{}
	app := New(out, &bytes.Buffer{})
	var captured scan.Options
	app.Scan = func(ctx context.Context, opts scan.Options) model.Snapshot {
		_ = ctx
		captured = opts
		return fixtureSnapshot()
	}

	code := app.Run(context.Background(), []string{
		"scan", "--top", "3", "--timeout", "5s", "--skip-docker", "--skip-wsl",
		"--no-command-lines", "--allow-remote-docker",
	})
	if code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	if captured.TopN != 3 {
		t.Errorf("TopN = %d", captured.TopN)
	}
	if captured.Timeout != 5*time.Second {
		t.Errorf("Timeout = %s", captured.Timeout)
	}
	if !captured.SkipDocker || !captured.SkipWSL {
		t.Errorf("skip flags not propagated: %+v", captured)
	}
	if !captured.AllowRemoteDocker {
		t.Error("--allow-remote-docker was not propagated")
	}
	if captured.EnrichCommandLines {
		t.Error("--no-command-lines did not disable enrichment")
	}
}

func TestTextOutputSanitisesHostileNames(t *testing.T) {
	snapshot := fixtureSnapshot()
	snapshot.Processes[0].Name = "node\x1b[31m.exe"
	snapshot.Processes[0].Project.Name = "evil\x07project"
	snapshot.Summary.TopProcesses[0].Name = "node\x1b[2J.exe"
	snapshot.Warnings = []string{"warning\x1b[0m"}
	snapshot.Recommendations[0].Title = "look\x1b[31m here"

	app, out, _ := newTestApp(snapshot)
	if code := app.Run(context.Background(), []string{"scan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	text := out.String()
	if strings.ContainsAny(text, "\x1b\x07") {
		t.Fatalf("terminal control characters reached the output:\n%q", text)
	}
	if !strings.Contains(text, "evil?project") {
		t.Errorf("project name escape sequence was not neutralised:\n%s", text)
	}
	if !strings.Contains(text, "node?[2J.exe") {
		t.Errorf("process name escape sequence was not neutralised:\n%s", text)
	}

	planApp, planOut, _ := newTestApp(snapshot)
	if code := planApp.Run(context.Background(), []string{"plan"}); code != ExitOK {
		t.Fatalf("plan exit code = %d", code)
	}
	if strings.ContainsAny(planOut.String(), "\x1b\x07") {
		t.Fatalf("terminal control characters reached the plan output")
	}
}

func TestSanitizeTruncatesLongValues(t *testing.T) {
	long := strings.Repeat("a", 500)
	got := sanitize(long)
	if len(got) > 210 {
		t.Fatalf("sanitize did not truncate: %d characters", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("truncated value has no marker: %q", got[len(got)-10:])
	}
}

// sensitiveSnapshot adds command lines, paths and credentials to the standard
// fixture so redaction can be tested end to end.
func sensitiveSnapshot() model.Snapshot {
	snapshot := fixtureSnapshot()
	command := `node C:\Users\Casey\Web Apps\demo\server.js --password hunter2 --token=abc123`
	snapshot.Processes[0].CommandLine = command
	snapshot.Processes[0].ExePath = `C:\Users\Casey\AppData\Local\Programs\nodejs\node.exe`
	snapshot.Processes[0].Project.Root = `C:\Users\Casey\Web Apps\demo`
	snapshot.Processes[0].Project.Evidence = []string{`package.json found in C:\Users\Casey\Web Apps\demo`}
	snapshot.Recommendations[0].Subjects[0].Detail = command
	snapshot.Recommendations[0].Detected = `project at C:\Users\Casey\Web Apps\demo`
	snapshot.Network.Listeners[0].ProcessName = `C:\Users\Casey\svc.exe`
	snapshot.Network.Listeners[0].ProjectName = `C:\Users\Casey\Web Apps\demo`
	snapshot.Network.Warnings = []string{`listener token API_TOKEN=xyz789 in C:\Users\Casey`}
	snapshot.Warnings = append(snapshot.Warnings, `token used: API_TOKEN=xyz789 in C:\Users\Casey\Web Apps\demo`)
	return snapshot
}

func assertNoSensitiveText(t *testing.T, text string) {
	t.Helper()
	for _, forbidden := range []string{"hunter2", "abc123", "xyz789", "Casey"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("output contains %q:\n%s", forbidden, text)
		}
	}
}

func TestScanJSONOmitsCommandLinesByDefault(t *testing.T) {
	app, out, _ := newTestApp(sensitiveSnapshot())
	app.HomeDir = `C:\Users\Casey`
	if code := app.Run(context.Background(), []string{"scan", "--json"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}

	body := out.String()
	assertNoSensitiveText(t, body)
	if !strings.Contains(body, `"commandLineOmitted": true`) {
		t.Errorf("commandLineOmitted marker missing:\n%s", body)
	}

	var got model.Snapshot
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Redaction == nil || got.Redaction.Mode != "default" || got.Redaction.CommandLines != "omitted" {
		t.Errorf("redaction = %+v", got.Redaction)
	}
	if got.Processes[0].CommandLine != "" {
		t.Errorf("command line present: %q", got.Processes[0].CommandLine)
	}
	if got.Recommendations[0].Subjects[0].Detail != "" {
		t.Errorf("subject detail present: %q", got.Recommendations[0].Subjects[0].Detail)
	}
}

func TestScanDiagnosticsRedactsSecretsAndWarns(t *testing.T) {
	app, out, errOut := newTestApp(sensitiveSnapshot())
	app.HomeDir = `C:\Users\Casey`
	if code := app.Run(context.Background(), []string{"scan", "--json", "--diagnostics"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}

	body := out.String()
	assertNoSensitiveText(t, body)
	if !strings.Contains(body, "[REDACTED]") {
		t.Error("diagnostics output has no redaction markers")
	}
	if !strings.Contains(body, "%USERPROFILE%") {
		t.Error("diagnostics output did not tokenise the home directory")
	}

	var got model.Snapshot
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Redaction == nil || got.Redaction.CommandLines != "redacted" {
		t.Errorf("redaction = %+v", got.Redaction)
	}
	if got.Processes[0].CommandLine == "" {
		t.Error("diagnostics omitted the command line entirely")
	}
	if !strings.Contains(errOut.String(), "diagnostics mode") {
		t.Errorf("missing stderr warning: %q", errOut.String())
	}
}

func TestPlanTextNeverPrintsRawCommandLines(t *testing.T) {
	app, out, _ := newTestApp(sensitiveSnapshot())
	app.HomeDir = `C:\Users\Casey`
	if code := app.Run(context.Background(), []string{"plan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	text := out.String()
	assertNoSensitiveText(t, text)
	if !strings.Contains(text, "OUTPUT POLICY") {
		t.Error("plan output does not state its output policy")
	}
	if !strings.Contains(text, "omitted") {
		t.Errorf("plan output should say command lines are omitted:\n%s", text)
	}
}

func TestScanTextStatesOutputPolicy(t *testing.T) {
	app, out, _ := newTestApp(sensitiveSnapshot())
	app.HomeDir = `C:\Users\Casey`
	if code := app.Run(context.Background(), []string{"scan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	text := out.String()
	assertNoSensitiveText(t, text)
	if !strings.Contains(text, "Output policy") {
		t.Error("scan notes do not state the output policy")
	}
}
