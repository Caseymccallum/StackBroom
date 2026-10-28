package main

import (
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func TestMapProcessesAttachesOpaqueTargetToken(t *testing.T) {
	started := time.Unix(0, 1712345678901234567)
	rows := mapProcesses([]model.Process{{PID: 42, Name: "test.exe", StartedAt: &started}}, func(process model.Process) string {
		if process.PID != 42 || process.StartedAt == nil || !process.StartedAt.Equal(started) {
			t.Errorf("unexpected token input: %+v", process)
		}
		return "opaque"
	})
	if len(rows) != 1 || rows[0].TargetToken != "opaque" {
		t.Fatalf("target token missing: %+v", rows)
	}
}
