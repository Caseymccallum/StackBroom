package main

import (
	"strings"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/processcontrol"
)

func mustTokenIssuer(t *testing.T) *processcontrol.TokenIssuer {
	t.Helper()
	tokens, err := processcontrol.NewTokenIssuer()
	if err != nil {
		t.Fatalf("token issuer: %v", err)
	}
	return tokens
}

func newTestProcessService(t *testing.T, identity processcontrol.ProcessIdentity) (*ProcessControlService, string) {
	t.Helper()
	service := NewProcessControlServiceWithTokens(mustTokenIssuer(t))
	service.verifyFn = func(pid int32, created time.Time) (processcontrol.ProcessIdentity, error) {
		if pid != identity.PID || !created.Equal(identity.CreateTime) {
			return processcontrol.ProcessIdentity{}, errTestIdentity
		}
		return identity, nil
	}
	token, err := service.tokens.Issue(identity)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return service, token
}

var errTestIdentity = &identityError{}

type identityError struct{}

func (*identityError) Error() string { return "identity changed" }

func fakeActions(terminateSucceeds bool) (processcontrol.NativeActions, *int) {
	opened := 0
	priority := processcontrol.PriorityNormal
	actions := processcontrol.NativeActions{
		OpenVerified: func(pid int32, access uint32, expected time.Time) (processcontrol.Handle, processcontrol.ProcessIdentity, error) {
			opened++
			return 1, processcontrol.ProcessIdentity{PID: pid, CreateTime: expected}, nil
		},
		Close:             func(processcontrol.Handle) error { return nil },
		Terminate:         func(processcontrol.Handle, uint32) error { return nil },
		Wait:              func(processcontrol.Handle, time.Duration) (bool, error) { return terminateSucceeds, nil },
		SendGracefulClose: func(int32, time.Time) (int, error) { return 1, nil },
		GetPriority:       func(processcontrol.Handle) (processcontrol.PriorityClass, error) { return priority, nil },
		SetPriority:       func(_ processcontrol.Handle, value processcontrol.PriorityClass) error { priority = value; return nil },
	}
	return actions, &opened
}

func TestApplicationServicesRefuseInvalidSecurityStateAndShareTokens(t *testing.T) {
	inspection, control, err := NewApplicationServices()
	if err != nil {
		t.Fatalf("application services: %v", err)
	}
	if inspection == nil || control == nil || control.tokens == nil {
		t.Fatal("application services must be fully initialised")
	}
	started := time.Now()
	token := inspection.issueTargetToken(model.Process{PID: 43, Name: "test.exe", StartedAt: &started})
	if _, err := control.tokens.Verify(token, 43); err != nil {
		t.Fatalf("services did not share a valid token key: %v", err)
	}
}

func TestScanAndProcessControlShareTokenIssuer(t *testing.T) {
	desktop := NewDesktopServiceWithTokens(mustTokenIssuer(t))
	control := NewProcessControlServiceWithTokens(desktop.tokens)
	started := time.Now()
	token := desktop.issueTargetToken(model.Process{PID: 42, Name: "test.exe", StartedAt: &started})
	if token == "" {
		t.Fatal("scan did not issue a target token")
	}
	created, err := control.tokens.Verify(token, 42)
	if err != nil {
		t.Fatalf("process control rejected a scan-issued token: %v", err)
	}
	if !created.Equal(started) {
		t.Errorf("creation time mismatch: %v vs %v", created, started)
	}
}

func TestPreviewConfirmExecuteOnce(t *testing.T) {
	identity := processcontrol.ProcessIdentity{PID: 4321, Name: "disposable.exe", CreateTime: time.Now()}
	service, token := newTestProcessService(t, identity)
	actions, opened := fakeActions(true)
	service.actions = actions

	preview, err := service.PreviewProcessAction(4321, "terminate", 0, token)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	operationID, _ := preview["operationId"].(string)
	if operationID == "" {
		t.Fatal("empty operation id")
	}
	if preview["risk"] != "irreversible" || preview["permission"] == "" {
		t.Errorf("preview risk/permission missing: %+v", preview)
	}

	result, err := service.ConfirmProcessAction(operationID)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if result["success"] != true || result["outcome"] != "success" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if *opened != 1 {
		t.Errorf("expected one verified handle open, got %d", *opened)
	}

	if _, err := service.ConfirmProcessAction(operationID); err == nil {
		t.Error("duplicate confirmation should be refused")
	}
}

func TestPreviewRejectsProtectedAndStaleTargets(t *testing.T) {
	identity := processcontrol.ProcessIdentity{PID: 4321, Name: "lsass.exe", CreateTime: time.Now(), IsProtected: true}
	service, token := newTestProcessService(t, identity)
	if _, err := service.PreviewProcessAction(4321, "terminate", 0, token); err == nil {
		t.Error("protected target should be refused")
	}

	clean := processcontrol.ProcessIdentity{PID: 4321, Name: "disposable.exe", CreateTime: time.Now()}
	service, token = newTestProcessService(t, clean)
	service.verifyFn = func(int32, time.Time) (processcontrol.ProcessIdentity, error) {
		return processcontrol.ProcessIdentity{}, errTestIdentity
	}
	if _, err := service.PreviewProcessAction(4321, "terminate", 0, token); err == nil {
		t.Error("stale target should be refused")
	}
}

func TestPreviewRejectsInvalidTokenAndPriority(t *testing.T) {
	identity := processcontrol.ProcessIdentity{PID: 4321, Name: "disposable.exe", CreateTime: time.Now()}
	service, _ := newTestProcessService(t, identity)

	if _, err := service.PreviewProcessAction(4321, "terminate", 0, "not-a-token"); err == nil {
		t.Error("invalid token should be refused")
	}
	if _, err := service.PreviewProcessAction(4321, "invalid_action", 0, "x"); err == nil {
		t.Error("invalid action should be refused")
	}
	if _, err := service.PreviewProcessAction(4321, "set_priority", int(processcontrol.PriorityRealTime), "x"); err == nil {
		t.Error("Real-time priority should be refused")
	}
}

func TestGetProcessDetailsDoesNotExposeExePath(t *testing.T) {
	identity := processcontrol.ProcessIdentity{PID: 4321, Name: "disposable.exe", ExePath: `C:\Users\TestUser\private\disposable.exe`, CreateTime: time.Now()}
	service, token := newTestProcessService(t, identity)
	details, err := service.GetProcessDetails(4321, token)
	if err != nil {
		t.Fatalf("details: %v", err)
	}
	if _, exists := details["exePath"]; exists {
		t.Error("exePath must not cross the desktop bridge")
	}
	encoded := strings.ToLower(strings.Join([]string{details["name"].(string)}, " "))
	if strings.Contains(encoded, "private") || strings.Contains(encoded, `c:\`) {
		t.Errorf("sensitive path leaked: %q", encoded)
	}
}

func TestPriorityConfirmationReportsReadback(t *testing.T) {
	identity := processcontrol.ProcessIdentity{PID: 4321, Name: "disposable.exe", CreateTime: time.Now()}
	service, token := newTestProcessService(t, identity)
	actions, _ := fakeActions(true)
	service.actions = actions

	preview, err := service.PreviewProcessAction(4321, "set_priority", int(processcontrol.PriorityHigh), token)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	result, err := service.ConfirmProcessAction(preview["operationId"].(string))
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if result["previousValue"] != "Normal" || result["newValue"] != "High" {
		t.Errorf("readback evidence missing: %+v", result)
	}
}
