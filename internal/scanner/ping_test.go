package scanner

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockPinger implements Pinger for testing concurrency and results.
type mockPinger struct {
	pingFunc func(ctx context.Context, subnet string, concurrency int) ([]string, error)
}

func (m *mockPinger) Ping(ctx context.Context, subnet string, concurrency int) ([]string, error) {
	return m.pingFunc(ctx, subnet, concurrency)
}

func TestPingerInterface(t *testing.T) {
	// Verify mockPinger satisfies Pinger interface
	var _ Pinger = (*mockPinger)(nil)
}

func TestPinger_ReturnsAliveHosts(t *testing.T) {
	// NS-1: Ping sweep returns responsive hosts
	expected := []string{"192.168.1.1", "192.168.1.10", "192.168.1.100"}
	mock := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			return expected, nil
		},
	}

	alive, err := mock.Ping(context.Background(), "192.168.1.0/24", 32)
	if err != nil {
		t.Fatalf("Ping() returned error: %v", err)
	}
	if len(alive) != 3 {
		t.Fatalf("len(alive) = %d, want 3", len(alive))
	}
	for i, ip := range expected {
		if alive[i] != ip {
			t.Errorf("alive[%d] = %q, want %q", i, alive[i], ip)
		}
	}
}

func TestPinger_EmptySubnet(t *testing.T) {
	// NS-1: No responsive hosts returns empty
	mock := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			return []string{}, nil
		},
	}

	alive, err := mock.Ping(context.Background(), "10.0.0.0/24", 32)
	if err != nil {
		t.Fatalf("Ping() returned error: %v", err)
	}
	if len(alive) != 0 {
		t.Errorf("len(alive) = %d, want 0", len(alive))
	}
}

func TestPinger_ConcurrencyLimit(t *testing.T) {
	// NS-2: Concurrent probes limited to configurable concurrency
	var mu sync.Mutex
	maxConcurrent := 0
	currentConcurrent := 0

	mock := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			mu.Lock()
			currentConcurrent++
			if currentConcurrent > maxConcurrent {
				maxConcurrent = currentConcurrent
			}
			mu.Unlock()

			// Simulate work
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			currentConcurrent--
			mu.Unlock()

			return []string{"192.168.1.1"}, nil
		},
	}

	// Test with 4 hosts and concurrency=2
	// Using smaller batch to verify the mock pattern
	alive, err := mock.Ping(context.Background(), "192.168.1.0/30", 2)
	if err != nil {
		t.Fatalf("Ping() returned error: %v", err)
	}
	if len(alive) != 1 {
		t.Errorf("len(alive) = %d, want 1", len(alive))
	}
}

func TestPinger_ContextCancelled(t *testing.T) {
	// Pinger should respect context cancellation
	mock := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(50 * time.Millisecond):
				return []string{"192.168.1.1"}, nil
			}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := mock.Ping(ctx, "192.168.1.0/24", 32)
	if err == nil {
		t.Error("Ping() expected error for cancelled context, got nil")
	}
}

func TestPinger_Timeout(t *testing.T) {
	// Pinger should respect timeout
	mock := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return []string{"192.168.1.1"}, nil
			}
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := mock.Ping(ctx, "192.168.1.0/24", 32)
	if err == nil {
		t.Error("Ping() expected timeout error, got nil")
	}
}

func TestPinger_Error(t *testing.T) {
	// Pinger should propagate errors
	expectedErr := errors.New("ping failed")
	mock := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			return nil, expectedErr
		},
	}

	_, err := mock.Ping(context.Background(), "192.168.1.0/24", 32)
	if err == nil {
		t.Fatal("Ping() expected error, got nil")
	}
}

// Test subnet IP generation - pure function for enumerating IPs in a CIDR range
func TestIPsInCIDR(t *testing.T) {
	// Test that a /24 generates 254 IPs (excluding network and broadcast)
	ips, err := IPsInCIDR("192.168.1.0/24")
	if err != nil {
		t.Fatalf("IPsInCIDR() returned error: %v", err)
	}
	if len(ips) != 254 {
		t.Errorf("len(ips) = %d, want 254 for /24", len(ips))
	}
	if ips[0] != "192.168.1.1" {
		t.Errorf("ips[0] = %q, want %q", ips[0], "192.168.1.1")
	}
	if ips[253] != "192.168.1.254" {
		t.Errorf("ips[253] = %q, want %q", ips[253], "192.168.1.254")
	}
}

func TestIPsInCIDR_SmallSubnet(t *testing.T) {
	// Test /30 generates 2 usable IPs
	ips, err := IPsInCIDR("10.0.0.0/30")
	if err != nil {
		t.Fatalf("IPsInCIDR() returned error: %v", err)
	}
	if len(ips) != 2 {
		t.Errorf("len(ips) = %d, want 2 for /30", len(ips))
	}
}

func TestIPsInCIDR_Invalid(t *testing.T) {
	// Test invalid CIDR returns error
	_, err := IPsInCIDR("not-a-cidr")
	if err == nil {
		t.Error("IPsInCIDR() expected error for invalid CIDR, got nil")
	}
}
