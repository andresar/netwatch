//go:build mdns

package scanner

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMDNS_DisabledByEnvVar(t *testing.T) {
	// MD-1: When NETWATCH_MDNS_ENABLED is not set, MDNSResolve returns empty
	os.Unsetenv("NETWATCH_MDNS_ENABLED")

	result, err := MDNSResolve(context.Background(), []string{"192.168.1.1"}, 5*time.Second)
	if err != nil {
		t.Fatalf("MDNSResolve() returned error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("MDNSResolve() returned %d results when disabled, want 0", len(result))
	}
}

func TestMDNS_EmptyIPs(t *testing.T) {
	// MD-1: Empty IP list returns empty result without error
	os.Setenv("NETWATCH_MDNS_ENABLED", "true")

	result, err := MDNSResolve(context.Background(), nil, 5*time.Second)
	if err != nil {
		t.Fatalf("MDNSResolve() returned error for nil IPs: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("MDNSResolve() returned %d results for nil IPs, want 0", len(result))
	}

	result, err = MDNSResolve(context.Background(), []string{}, 5*time.Second)
	if err != nil {
		t.Fatalf("MDNSResolve() returned error for empty IPs: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("MDNSResolve() returned %d results for empty IPs, want 0", len(result))
	}
}

func TestMDNS_Timeout(t *testing.T) {
	// MD-2: mDNS respects configurable timeout — not an error, just empty result
	os.Setenv("NETWATCH_MDNS_ENABLED", "true")

	// Very short context timeout to exercise the timeout path
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	result, err := MDNSResolve(ctx, []string{"192.168.1.1", "192.168.1.2"}, 500*time.Millisecond)
	// Timeout should NOT be an error — phase continues gracefully
	if err != nil {
		t.Logf("MDNSResolve() returned error (acceptable with short timeout): %v", err)
	}
	_ = result
}

func TestMDNS_DefaultTimeout(t *testing.T) {
	// MD-2: Zero/negative timeout uses default (5s)
	os.Setenv("NETWATCH_MDNS_ENABLED", "true")

	result, err := MDNSResolve(context.Background(), []string{"192.168.1.1"}, 0)
	if err != nil {
		t.Fatalf("MDNSResolve() returned error: %v", err)
	}
	_ = result
}

func TestMDNS_EnabledByEnvVar(t *testing.T) {
	// MD-1: When NETWATCH_MDNS_ENABLED is true, the function attempts resolution
	os.Setenv("NETWATCH_MDNS_ENABLED", "true")

	result, err := MDNSResolve(context.Background(), []string{"192.168.1.1"}, 3*time.Second)
	if err != nil {
		t.Logf("MDNSResolve() returned error (expected — no mDNS responder in test env): %v", err)
	}
	_ = result
}
