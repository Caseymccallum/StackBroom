package sysinfo

import (
	"reflect"
	"testing"
)

func TestCPUUsage(t *testing.T) {
	previous := cpuTimes{Idle: 100, Kernel: 200, User: 100}
	current := cpuTimes{Idle: 130, Kernel: 270, User: 130}
	percent, ok := cpuUsage(previous, current)
	if !ok {
		t.Fatal("cpuUsage reported unavailable")
	}
	if percent < 69.9 || percent > 70.1 {
		t.Errorf("percent = %.2f, want 70", percent)
	}

	if _, ok := cpuUsage(current, current); ok {
		t.Error("zero-delta sample should be unavailable")
	}
	if _, ok := cpuUsage(cpuTimes{}, cpuTimes{Idle: 10, Kernel: 5, User: 1}); ok {
		t.Error("idle greater than total should be rejected")
	}
}

func TestDriveRoots(t *testing.T) {
	mask := uint32(1<<2 | 1<<3 | 1<<25)
	want := []string{`C:\`, `D:\`, `Z:\`}
	if got := driveRoots(mask); !reflect.DeepEqual(got, want) {
		t.Errorf("driveRoots = %v, want %v", got, want)
	}
	if got := driveRoots(0); len(got) != 0 {
		t.Errorf("empty mask = %v", got)
	}
}

func TestVolumeUsage(t *testing.T) {
	used, percent := volumeUsage(1000, 250, 200)
	if used != 800 {
		t.Errorf("used = %d, want 800", used)
	}
	if percent != 80 {
		t.Errorf("percent = %f, want 80", percent)
	}
	used, percent = volumeUsage(0, 0, 0)
	if used != 0 || percent != 0 {
		t.Errorf("zero total = %d, %f", used, percent)
	}
	used, percent = volumeUsage(100, 200, 200)
	if used != 0 || percent != 0 {
		t.Errorf("free above total = %d, %f", used, percent)
	}
}

func TestWindowsVersionLabel(t *testing.T) {
	tests := []struct {
		major, minor, build uint32
		want                string
	}{
		{10, 0, 22631, "Windows 11 (build 22631)"},
		{10, 0, 19045, "Windows 10 (build 19045)"},
		{6, 3, 9600, "Windows 8.1 (build 9600)"},
		{6, 1, 7601, "Windows 7 (build 7601)"},
		{11, 0, 1, "Windows 11.0 (build 1)"},
	}
	for _, tc := range tests {
		if got := windowsVersionLabel(tc.major, tc.minor, tc.build); got != tc.want {
			t.Errorf("label(%d,%d,%d) = %q, want %q", tc.major, tc.minor, tc.build, got, tc.want)
		}
	}
}
