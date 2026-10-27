//go:build windows

package netstat

import (
	"context"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// TestLiveCollector is opt-in and read-only. It never opens a socket or sends a
// packet; it only verifies that the production table source returns a
// well-formed sample on this machine.
func TestLiveCollector(t *testing.T) {
	if os.Getenv("STACKBROOM_LIVE_NETSTAT") != "1" {
		t.Skip("set STACKBROOM_LIVE_NETSTAT=1 to inspect the live Windows TCP table")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := (Collector{}).Collect(ctx)
	if !got.Available {
		t.Fatalf("listener inspection unavailable: %s", got.Reason)
	}
	if !got.IPv4 {
		t.Errorf("IPv4 table was not readable; warnings: %v", got.Warnings)
	}
	if got.SampledAt.IsZero() {
		t.Error("sampledAt is zero")
	}
	if got.Listeners == nil {
		t.Fatal("listeners is nil")
	}

	for _, listener := range got.Listeners {
		if listener.Protocol != Protocol {
			t.Errorf("protocol = %q", listener.Protocol)
		}
		if listener.Family != FamilyIPv4 && listener.Family != FamilyIPv6 {
			t.Errorf("family = %q", listener.Family)
		}
		if listener.Port < 1 || listener.Port > 65535 {
			t.Errorf("invalid port %d", listener.Port)
		}
		if listener.PID < 0 {
			t.Errorf("negative pid %d", listener.PID)
		}
		address := listener.Address
		if zone := strings.LastIndex(address, "%"); zone >= 0 {
			address = address[:zone]
		}
		if _, err := netip.ParseAddr(address); err != nil {
			t.Errorf("address %q is not parseable: %v", listener.Address, err)
		}
		if listener.Ownership != model.Unknown {
			t.Errorf("raw listener ownership = %q, want unknown before association", listener.Ownership)
		}
	}
	t.Logf("observed %d listeners (IPv4=%v IPv6=%v sampled=%s)",
		len(got.Listeners), got.IPv4, got.IPv6, got.SampledAt.Format(time.RFC3339))
}
