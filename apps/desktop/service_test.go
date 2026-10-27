package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/sysinfo"
)

func fixedMetrics() sysinfo.Metrics {
	return sysinfo.Metrics{
		Hostname:             "TEST-PC",
		WindowsVersion:       "Windows 11 (build 26100)",
		BuildNumber:          26100,
		LogicalProcessors:    16,
		TotalMemoryBytes:     32 << 30,
		AvailableMemoryBytes: 16 << 30,
		MemoryKnown:          true,
		CPUPercent:           12.5,
		CPUKnown:             true,
		Volumes: []sysinfo.Volume{{
			Root: `C:\`, Label: "System", FileSystem: "NTFS",
			TotalBytes: 1000, FreeBytes: 400, UsedBytes: 600, UsedPercent: 60,
		}},
		SampledAt: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC),
	}
}

func testSnapshot() model.Snapshot {
	project := &model.ProjectRef{
		Root: `C:\Users\TestUser\secret-project`, Name: "demo", Confidence: model.Confirmed,
	}
	started := time.Unix(100, 0)
	return model.Snapshot{
		Host: model.Host{Hostname: "FALLBACK"},
		Processes: []model.Process{{
			PID: 100, Name: "node.exe", Runtime: model.RuntimeNode, Category: model.CategoryDevServer,
			CommandLine:      `node C:\Users\TestUser\secret-project\server.js --token hunter2`,
			CommandLineKnown: true, ExePath: `C:\Users\TestUser\node.exe`,
			MemoryKnown: true, WorkingSetBytes: 100 << 20, Project: project,
			RequestedPorts: []int{5173}, ListeningPorts: []int{5173}, PortOwnership: model.Confirmed,
			StartTimeKnown: true, StartedAt: &started,
		}},
		Summary: model.Summary{
			TotalProcesses: 1, DevProcessCount: 1, DevServerCount: 1,
			TopProcesses: []model.TopProcess{{PID: 100, Name: "node.exe", Category: model.CategoryDevServer, MemoryBytes: 100 << 20, MemoryKnown: true, ProjectName: "demo"}},
		},
		Network: model.NetworkInfo{
			Available: true, IPv4: true, IPv6: true,
			Listeners: []model.TCPListener{{Protocol: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 5173, PID: 100, ProcessName: "node.exe", Ownership: model.Confirmed}},
		},
		Docker: model.DockerInfo{
			Status:     model.DockerStatus{Available: false, Reason: "daemon unavailable"},
			Containers: []model.DockerContainer{},
			DiskUsage:  []model.DockerDiskUsage{},
			Images:     []model.DockerImage{},
		},
		WSL: model.WSLInfo{Available: false, Reason: "wsl.exe not found", Distributions: []model.WSLDistribution{}},
		Duplicates: []model.DuplicateGroup{{
			Kind: model.DuplicateByPort, Key: "node on port 5173", Runtime: model.RuntimeNode,
			Port: 5173, PIDs: []int32{100}, Confidence: model.Inferred,
			Evidence:  []string{`PID 100 on C:\Users\TestUser\secret-project`},
			Listeners: []model.ListenerRef{{Family: "ipv4", Address: "127.0.0.1", Port: 5173, PID: 100, Ownership: model.Confirmed}},
		}},
		Recommendations: []model.Recommendation{{
			ID: "x", Kind: "duplicate-dev-server", Title: "Review node", Detected: "one",
			WhyUnnecessary: "example", Evidence: []string{`API_TOKEN=hunter2 at C:\Users\TestUser\secret-project`},
			Resource: "100 MB", Review: []string{"check"}, Risk: model.RiskMedium,
			RiskDetail: "example", Confidence: model.Inferred,
		}},
		Warnings: []string{`Docker warning at C:\Users\TestUser\secret-project`},
	}
}

func testService() *DesktopService {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	return &DesktopService{
		scanFn:    func(context.Context, string) model.Snapshot { return testSnapshot() },
		metricsFn: func(context.Context) sysinfo.Metrics { return fixedMetrics() },
		now:       func() time.Time { return now },
	}
}

func TestFullScanDTOIsNarrowAndRedacted(t *testing.T) {
	result, err := testService().FullScan()
	if err != nil {
		t.Fatalf("FullScan: %v", err)
	}
	if len(result.Processes) != 1 || result.Processes[0].PID != 100 {
		t.Fatalf("processes = %+v", result.Processes)
	}
	if result.Overview.Hostname != "TEST-PC" || !result.Overview.MemoryKnown || !result.Overview.CPUKnown {
		t.Errorf("overview = %+v", result.Overview)
	}
	if len(result.Developer.Listeners) != 1 || len(result.Developer.Duplicates) != 1 {
		t.Errorf("developer = %+v", result.Developer)
	}
	if result.Developer.Docker.Available || result.Developer.WSL.Available {
		t.Errorf("unavailable services were reported available: %+v", result.Developer)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"hunter2", "TestUser", "CommandLine"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("desktop DTO leaked %q:\n%s", forbidden, text)
		}
	}
}

func TestQuickAndFullModes(t *testing.T) {
	var mu sync.Mutex
	var modes []string
	service := testService()
	service.scanFn = func(_ context.Context, mode string) model.Snapshot {
		mu.Lock()
		modes = append(modes, mode)
		mu.Unlock()
		return testSnapshot()
	}
	if _, err := service.QuickScan(); err != nil {
		t.Fatalf("QuickScan: %v", err)
	}
	if _, err := service.FullScan(); err != nil {
		t.Fatalf("FullScan: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(modes) != 2 || modes[0] != "quick" || modes[1] != "full" {
		t.Errorf("modes = %v", modes)
	}
}

func TestAppInfoIsReadOnly(t *testing.T) {
	info := testService().GetAppInfo()
	if !info.ReadOnly || info.SchemaVersion != "3" || info.Name != "stackbroom" {
		t.Errorf("info = %+v", info)
	}
	if len(info.Limitations) == 0 || !strings.Contains(strings.Join(info.Limitations, " "), "read-only") {
		t.Errorf("limitations = %v", info.Limitations)
	}
}
