//go:build windows

package sysinfo

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveSystemMetrics(t *testing.T) {
	if os.Getenv("STACKBROOM_LIVE_SYSINFO") != "1" {
		t.Skip("set STACKBROOM_LIVE_SYSINFO=1 to read live host metrics")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := Collect(ctx)
	if !got.MemoryKnown || got.TotalMemoryBytes == 0 || got.AvailableMemoryBytes > got.TotalMemoryBytes {
		t.Errorf("memory = %+v", got)
	}
	if !got.CPUKnown || got.CPUPercent < 0 || got.CPUPercent > 100 {
		t.Errorf("cpu = %v known=%v", got.CPUPercent, got.CPUKnown)
	}
	if len(got.Volumes) == 0 {
		t.Errorf("no fixed volumes: %v", got.Warnings)
	}
	for _, volume := range got.Volumes {
		if volume.TotalBytes == 0 || volume.FreeBytes > volume.TotalBytes {
			t.Errorf("volume = %+v", volume)
		}
	}
	t.Logf("host=%s os=%s cpu=%.1f%% volumes=%d",
		got.Hostname, got.WindowsVersion, got.CPUPercent, len(got.Volumes))
}
