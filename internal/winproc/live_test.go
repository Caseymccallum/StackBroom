//go:build windows

package winproc

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveProcessCollection is opt-in and read-only. Enable it with:
//
//	$env:STACKBROOM_LIVE = "1"; go test ./internal/winproc -run TestLive
//
// It only reads the process list; it never opens a handle that can terminate a
// process.
func TestLiveProcessCollection(t *testing.T) {
	if os.Getenv("STACKBROOM_LIVE") != "1" {
		t.Skip("set STACKBROOM_LIVE=1 to run the read-only live inspection test")
	}

	res, err := Collect(context.Background(), Options{
		EnrichCommandLines: false,
		Timeout:            15 * time.Second,
	})
	if err != nil {
		t.Fatalf("Collect returned an error: %v", err)
	}
	if len(res.Processes) == 0 {
		t.Fatal("no processes were collected")
	}

	self := int32(os.Getpid())
	foundSelf := false
	for _, p := range res.Processes {
		if p.PID == self {
			foundSelf = true
			if p.Name == "" {
				t.Errorf("process %d has no name", p.PID)
			}
		}
		if p.MemoryKnown && p.WorkingSetBytes == 0 {
			t.Errorf("process %d reports known memory of zero", p.PID)
		}
	}
	if !foundSelf {
		t.Errorf("the test process (pid %d) was not found in the process list", self)
	}
}

func TestLiveCreationTime(t *testing.T) {
	if os.Getenv("STACKBROOM_LIVE") != "1" {
		t.Skip("set STACKBROOM_LIVE=1 to run the read-only live inspection test")
	}
	self := int32(os.Getpid())
	first, ok := CreationTime(self)
	if !ok {
		t.Fatal("CreationTime failed for the test process")
	}
	second, ok := CreationTime(self)
	if !ok {
		t.Fatal("CreationTime failed on the second read")
	}
	if !first.Equal(second) {
		t.Fatalf("creation time changed: %s then %s", first, second)
	}
	if first.After(time.Now().Add(time.Minute)) {
		t.Errorf("implausible creation time in the future: %s", first)
	}
}

func TestLiveHostMemory(t *testing.T) {
	if os.Getenv("STACKBROOM_LIVE") != "1" {
		t.Skip("set STACKBROOM_LIVE=1 to run the read-only live inspection test")
	}
	total, available, ok := HostMemory()
	if !ok {
		t.Fatal("HostMemory reported unavailable")
	}
	if total == 0 || available > total {
		t.Fatalf("implausible memory figures: total=%d available=%d", total, available)
	}
}
