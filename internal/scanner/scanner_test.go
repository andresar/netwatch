package scanner

import (
	"context"
	"testing"
	"time"

	"github.com/andresar/netwatch/internal/models"
)

// mockScanner implements Scanner for testing.
type mockScanner struct {
	scanFunc func(ctx context.Context, subnet string) (*models.ScanResult, error)
}

func (m *mockScanner) Scan(ctx context.Context, subnet string) (*models.ScanResult, error) {
	return m.scanFunc(ctx, subnet)
}

func TestScannerInterfaceContract(t *testing.T) {
	// Verify a mock satisfies the Scanner interface
	var _ Scanner = (*mockScanner)(nil)
	_ = 1 // compile-time check
}

func TestMockScanner_Scan(t *testing.T) {
	// Mock Scanner returns expected results
	expected := &models.ScanResult{
		Devices: []models.Device{
			{IP: "192.168.1.1", MAC: "00:11:22:aa:bb:cc", Hostname: "router", Vendor: "Cisco"},
		},
		ScannedAt: time.Now(),
		Total:     1,
	}

	mock := &mockScanner{
		scanFunc: func(ctx context.Context, subnet string) (*models.ScanResult, error) {
			return expected, nil
		},
	}

	result, err := mock.Scan(context.Background(), "192.168.1.0/24")
	if err != nil {
		t.Fatalf("mock.Scan() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("mock.Scan() returned nil result")
	}
	if result.Total != 1 {
		t.Errorf("result.Total = %d, want 1", result.Total)
	}
	if len(result.Devices) != 1 {
		t.Fatalf("len(result.Devices) = %d, want 1", len(result.Devices))
	}
	if result.Devices[0].IP != "192.168.1.1" {
		t.Errorf("result.Devices[0].IP = %q, want %q", result.Devices[0].IP, "192.168.1.1")
	}
}

func TestMockScanner_Error(t *testing.T) {
	// Mock Scanner propagates errors
	mock := &mockScanner{
		scanFunc: func(ctx context.Context, subnet string) (*models.ScanResult, error) {
			return nil, context.DeadlineExceeded
		},
	}

	_, err := mock.Scan(context.Background(), "10.0.0.0/16")
	if err == nil {
		t.Fatal("mock.Scan() expected error, got nil")
	}
}

func TestMockScanner_CancelledContext(t *testing.T) {
	// Mock Scanner respects context cancellation
	mock := &mockScanner{
		scanFunc: func(ctx context.Context, subnet string) (*models.ScanResult, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return &models.ScanResult{}, nil
			}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := mock.Scan(ctx, "192.168.1.0/24")
	if err == nil {
		t.Error("mock.Scan() expected error for cancelled context, got nil")
	}
	if result != nil {
		t.Error("mock.Scan() expected nil result for cancelled context")
	}
}

func TestScanConfig_Defaults(t *testing.T) {
	// ScanConfig sensible defaults
	cfg := DefaultScanConfig()
	if cfg.PingConcurrency != 32 {
		t.Errorf("cfg.PingConcurrency = %d, want 32", cfg.PingConcurrency)
	}
	if cfg.ScanTimeout != 30*time.Second {
		t.Errorf("cfg.ScanTimeout = %v, want 30s", cfg.ScanTimeout)
	}
	if cfg.DNSTimeout != 3*time.Second {
		t.Errorf("cfg.DNSTimeout = %v, want 3s", cfg.DNSTimeout)
	}
}
