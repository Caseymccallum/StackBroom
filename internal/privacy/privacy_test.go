package privacy

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func fixtureSnapshot() model.Snapshot {
	started := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	return model.Snapshot{
		SchemaVersion: model.SchemaVersion,
		ReadOnly:      true,
		GeneratedAt:   started,
		Processes: []model.Process{{
			PID: 100, Name: "node.exe",
			ExePath:          `C:\Users\Casey\AppData\Local\Programs\nodejs\node.exe`,
			CommandLine:      `node C:\Users\Casey\Web Apps\demo\server.js --password hunter2`,
			CommandLineKnown: true,
			MemoryKnown:      true,
			WorkingSetBytes:  10 << 20,
			Project: &model.ProjectRef{
				Root:       `C:\Users\Casey\Web Apps\demo`,
				Name:       "demo",
				Marker:     "package.json",
				Confidence: model.Confirmed,
				Evidence:   []string{`package.json found in C:\Users\Casey\Web Apps\demo`},
			},
		}},
		Docker: model.DockerInfo{
			Status: model.DockerStatus{Available: true, Endpoint: "npipe:////./pipe/docker_engine", EndpointLocal: true},
			Containers: []model.DockerContainer{{
				Name: "web", Image: "nginx:latest", State: "exited",
				ComposeWorkingDir: `C:\Users\Casey\Web Apps\demo`,
			}},
			Warnings: []string{`could not read C:\Users\Casey\Web Apps\demo\compose.yml`},
		},
		Recommendations: []model.Recommendation{{
			ID: "x", Kind: "duplicate-dev-server", Title: "demo issue",
			Detected:       `project at C:\Users\Casey\Web Apps\demo`,
			WhyUnnecessary: "example",
			Evidence:       []string{`token=abc123 in command line`},
			Resource:       "10 MB",
			Review:         []string{"check it"},
			Risk:           model.RiskMedium,
			RiskDetail:     "example risk",
			Confidence:     model.Inferred,
			Subjects: []model.Subject{{
				Kind:   "process",
				ID:     "100",
				Detail: `node C:\Users\Casey\Web Apps\demo\server.js --password hunter2`,
			}},
		}},
		Warnings: []string{`command line C:\Users\Casey\Web Apps\demo\server.js had API_TOKEN=xyz789`},
	}
}

func assertNoSecretsOrHome(t *testing.T, snapshot model.Snapshot, secrets ...string) {
	t.Helper()
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	text := string(encoded)
	for _, secret := range append(secrets, "Casey") {
		if strings.Contains(text, secret) {
			t.Errorf("output still contains %q:\n%s", secret, text)
		}
	}
}

func TestApplyDefaultOmitsCommandLinesAndRedacts(t *testing.T) {
	input := fixtureSnapshot()
	out := Apply(input, Policy{Mode: ModeDefault, UserProfile: `C:\Users\Casey`})

	if out.Redaction == nil || out.Redaction.Mode != "default" || out.Redaction.CommandLines != "omitted" {
		t.Fatalf("redaction = %+v", out.Redaction)
	}
	if out.Processes[0].CommandLine != "" || !out.Processes[0].CommandLineOmitted {
		t.Errorf("command line was not omitted: %+v", out.Processes[0])
	}
	if out.Recommendations[0].Subjects[0].Detail != "" {
		t.Errorf("subject detail leaked a command line: %q", out.Recommendations[0].Subjects[0].Detail)
	}
	assertNoSecretsOrHome(t, out, "hunter2", "abc123", "xyz789")
}

func TestApplyDiagnosticsRedactsButKeepsDetail(t *testing.T) {
	input := fixtureSnapshot()
	out := Apply(input, Policy{Mode: ModeDiagnostics, UserProfile: `C:\Users\Casey`})

	if out.Redaction == nil || out.Redaction.CommandLines != "redacted" {
		t.Fatalf("redaction = %+v", out.Redaction)
	}
	cmd := out.Processes[0].CommandLine
	if cmd == "" {
		t.Fatal("diagnostics dropped the command line")
	}
	if strings.Contains(cmd, "hunter2") {
		t.Errorf("secret survived: %q", cmd)
	}
	if !strings.Contains(cmd, "[REDACTED]") {
		t.Errorf("no redaction marker: %q", cmd)
	}
	if !strings.Contains(cmd, "%USERPROFILE%") {
		t.Errorf("home path was not tokenised: %q", cmd)
	}
	assertNoSecretsOrHome(t, out, "hunter2", "abc123", "xyz789")
}

func TestApplyDoesNotMutateInput(t *testing.T) {
	input := fixtureSnapshot()
	_ = Apply(input, Policy{Mode: ModeDefault, UserProfile: `C:\Users\Casey`})

	if input.Processes[0].CommandLine == "" {
		t.Error("Apply mutated the input snapshot")
	}
	if input.Processes[0].Project.Root == "" {
		t.Error("Apply mutated the input project")
	}
}

func TestApplyRedactsEvidenceAndWarnings(t *testing.T) {
	out := Apply(fixtureSnapshot(), Policy{Mode: ModeDefault, UserProfile: `C:\Users\Casey`})
	evidence := strings.Join(out.Recommendations[0].Evidence, " ")
	if strings.Contains(evidence, "abc123") {
		t.Errorf("evidence leaked a secret: %q", evidence)
	}
	warnings := strings.Join(out.Warnings, " ")
	if strings.Contains(warnings, "xyz789") {
		t.Errorf("warnings leaked a secret: %q", warnings)
	}
	if strings.Contains(out.Docker.Containers[0].ComposeWorkingDir, "Casey") {
		t.Errorf("compose path was not tokenised: %q", out.Docker.Containers[0].ComposeWorkingDir)
	}
}

func TestApplyEmptySnapshot(t *testing.T) {
	out := Apply(model.Snapshot{}, Policy{Mode: ModeDefault})
	if out.Redaction == nil || out.Redaction.Mode != "default" {
		t.Fatalf("redaction = %+v", out.Redaction)
	}
}
