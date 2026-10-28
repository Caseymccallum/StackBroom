package processcontrol

import (
	"errors"
	"testing"
	"time"
)

func previewFor(t *testing.T, action ProcessAction, priority PriorityClass) *OperationPreview {
	t.Helper()
	return &OperationPreview{
		OperationID: "op-1",
		PID:         123,
		Action:      action,
		Priority:    priority,
		Target:      ProcessIdentity{PID: 123, Name: "test.exe", CreateTime: time.Now()},
	}
}

func testActions(t *testing.T) (NativeActions, *fakeState) {
	t.Helper()
	state := &fakeState{waitResult: true, priority: PriorityNormal}
	return NativeActions{
		OpenVerified: func(pid int32, access uint32, expected time.Time) (Handle, ProcessIdentity, error) {
			state.opened++
			if state.openErr != nil {
				return 0, ProcessIdentity{}, state.openErr
			}
			return 1, ProcessIdentity{PID: pid, Name: "test.exe", CreateTime: expected}, nil
		},
		Close:             func(Handle) error { state.closed++; return nil },
		Terminate:         func(Handle, uint32) error { state.terminated++; return nil },
		Wait:              func(Handle, time.Duration) (bool, error) { return state.waitResult, state.waitErr },
		SendGracefulClose: func(int32, time.Time) (int, error) { return state.windowCount, state.closeErr },
		GetPriority:       func(Handle) (PriorityClass, error) { return state.priority, nil },
		SetPriority: func(_ Handle, value PriorityClass) error {
			if state.priorityErr != nil {
				return state.priorityErr
			}
			if !state.ignorePriority {
				state.priority = value
			}
			return nil
		},
	}, state
}

type fakeState struct {
	opened         int
	closed         int
	terminated     int
	windowCount    int
	waitResult     bool
	waitErr        error
	openErr        error
	closeErr       error
	priority       PriorityClass
	priorityErr    error
	ignorePriority bool
}

func TestGracefulCloseOutcomes(t *testing.T) {
	actions, state := testActions(t)
	state.windowCount = 1
	result := ExecuteGracefulClose(previewFor(t, ActionGracefulClose, PriorityNormal), actions)
	if !result.Success || result.Outcome != "success" {
		t.Errorf("expected verified success, got %+v", result)
	}
	if state.closed != 1 {
		t.Error("handle not closed")
	}

	state.waitResult = false
	result = ExecuteGracefulClose(previewFor(t, ActionGracefulClose, PriorityNormal), actions)
	if result.Success || result.Outcome != "timeout" {
		t.Errorf("ignored close must be timeout, got %+v", result)
	}

	state.windowCount = 0
	result = ExecuteGracefulClose(previewFor(t, ActionGracefulClose, PriorityNormal), actions)
	if result.Success || result.Outcome != "partial" {
		t.Errorf("no window must be partial, got %+v", result)
	}

	state.windowCount = 1
	state.openErr = errors.New("access denied")
	result = ExecuteGracefulClose(previewFor(t, ActionGracefulClose, PriorityNormal), actions)
	if result.Success || result.Outcome != "refused" {
		t.Errorf("open failure must be refused, got %+v", result)
	}
}

func TestTerminateRequiresVerifiedExit(t *testing.T) {
	actions, state := testActions(t)
	result := ExecuteTerminate(previewFor(t, ActionTerminate, PriorityNormal), actions)
	if !result.Success || state.terminated != 1 {
		t.Errorf("expected success, got %+v", result)
	}

	state.waitResult = false
	result = ExecuteTerminate(previewFor(t, ActionTerminate, PriorityNormal), actions)
	if result.Success || result.Outcome != "partial" {
		t.Errorf("unverified exit must be partial, got %+v", result)
	}
	if state.closed < 2 {
		t.Error("handles not closed on both paths")
	}
}

func TestPriorityReadbackAndRealTimeRefusal(t *testing.T) {
	actions, state := testActions(t)
	result := ExecuteSetPriority(previewFor(t, ActionSetPriority, PriorityHigh), actions)
	if !result.Success || result.PreviousValue != "Normal" || result.NewValue != "High" {
		t.Errorf("unexpected priority result: %+v", result)
	}

	state.ignorePriority = true
	state.priority = PriorityNormal
	result = ExecuteSetPriority(previewFor(t, ActionSetPriority, PriorityHigh), actions)
	if result.Success || result.Outcome != "partial" {
		t.Errorf("readback mismatch must be partial, got %+v", result)
	}

	result = ExecuteSetPriority(previewFor(t, ActionSetPriority, PriorityRealTime), actions)
	if result.Success || result.Outcome != "refused" {
		t.Errorf("Real-time must be refused, got %+v", result)
	}
}

func TestConcurrentPreviewConsumption(t *testing.T) {
	store := NewOperationStore()
	identity := ProcessIdentity{PID: 5, Name: "test.exe", CreateTime: time.Now()}
	preview, err := store.CreatePreview(5, ActionTerminate, PriorityNormal, identity)
	if err != nil {
		t.Fatal(err)
	}
	const workers = 16
	results := make(chan error, workers)
	for index := 0; index < workers; index++ {
		go func() {
			_, err := store.TryConsumePreview(preview.OperationID)
			results <- err
		}()
	}
	successes := 0
	for index := 0; index < workers; index++ {
		if err := <-results; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly one consumer, got %d", successes)
	}
}
