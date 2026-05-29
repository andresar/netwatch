package scanner

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockARPReader implements ARPReader for testing.
type mockARPReader struct {
	readFunc func() (map[string]string, error)
}

func (m *mockARPReader) Read() (map[string]string, error) {
	return m.readFunc()
}

func TestOrchestratorInterface(t *testing.T) {
	// Verify Orchestrator satisfies Scanner interface
	var _ Scanner = (*Orchestrator)(nil)
}

func TestOrchestrator_FullPipeline(t *testing.T) {
	// NS-4: Full pipeline enriches devices with MAC, hostname, vendor
	pinger := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			return []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}, nil
		},
	}
	arp := &mockARPReader{
		readFunc: func() (map[string]string, error) {
			return map[string]string{
				"192.168.1.1": "00:11:22:aa:bb:cc",
				"192.168.1.2": "aa:bb:cc:dd:ee:ff",
			}, nil
		},
	}
	dns := &mockDNSResolver{
		resolveFunc: func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
			return map[string]string{
				"192.168.1.1": "router.home",
			}, nil
		},
	}
	oui := &mockOUILookup{
		lookupFunc: func(mac string) string {
			switch mac {
			case "00:11:22:aa:bb:cc":
				return "Cisco Systems, Inc."
			case "aa:bb:cc:dd:ee:ff":
				return "Apple Inc."
			default:
				return "Unknown"
			}
		},
	}

	orch := NewOrchestrator(pinger, arp, dns, oui, DefaultScanConfig())
	result, err := orch.Scan(context.Background(), "192.168.1.0/24")
	if err != nil {
		t.Fatalf("Scan() returned error: %v", err)
	}

	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
	if len(result.Devices) != 3 {
		t.Fatalf("len(Devices) = %d, want 3", len(result.Devices))
	}

	// Verify each device is enriched correctly
	for _, d := range result.Devices {
		switch d.IP {
		case "192.168.1.1":
			if d.MAC != "00:11:22:aa:bb:cc" {
				t.Errorf("Device 192.168.1.1 MAC = %q, want 00:11:22:aa:bb:cc", d.MAC)
			}
			if d.Hostname != "router.home" {
				t.Errorf("Device 192.168.1.1 Hostname = %q, want router.home", d.Hostname)
			}
			if d.Vendor != "Cisco Systems, Inc." {
				t.Errorf("Device 192.168.1.1 Vendor = %q, want Cisco Systems, Inc.", d.Vendor)
			}
		case "192.168.1.2":
			if d.MAC != "aa:bb:cc:dd:ee:ff" {
				t.Errorf("Device 192.168.1.2 MAC = %q, want aa:bb:cc:dd:ee:ff", d.MAC)
			}
			if d.Hostname != "" {
				t.Errorf("Device 192.168.1.2 Hostname = %q, want empty", d.Hostname)
			}
			if d.Vendor != "Apple Inc." {
				t.Errorf("Device 192.168.1.2 Vendor = %q, want Apple Inc.", d.Vendor)
			}
		case "192.168.1.3":
			if d.MAC != "" {
				t.Errorf("Device 192.168.1.3 MAC = %q, want empty (not in ARP)", d.MAC)
			}
			if d.Hostname != "" {
				t.Errorf("Device 192.168.1.3 Hostname = %q, want empty", d.Hostname)
			}
			if d.Vendor != "Unknown" {
				t.Errorf("Device 192.168.1.3 Vendor = %q, want Unknown", d.Vendor)
			}
		default:
			t.Errorf("Unexpected device IP: %s", d.IP)
		}
	}
}

func TestOrchestrator_Timeout(t *testing.T) {
	// Scanner respects timeout — context cancelled after deadline
	pinger := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	arp := &mockARPReader{
		readFunc: func() (map[string]string, error) {
			return map[string]string{}, nil
		},
	}
	dns := &mockDNSResolver{
		resolveFunc: func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}
	oui := &mockOUILookup{
		lookupFunc: func(mac string) string {
			return "Unknown"
		},
	}

	cfg := DefaultScanConfig()
	cfg.ScanTimeout = 5 * time.Millisecond
	orch := NewOrchestrator(pinger, arp, dns, oui, cfg)

	_, err := orch.Scan(context.Background(), "192.168.1.0/24")
	if err == nil {
		t.Fatal("Scan() expected timeout error, got nil")
	}
}

func TestOrchestrator_CancelledContext(t *testing.T) {
	// Scanner respects context cancellation
	pinger := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	arp := &mockARPReader{
		readFunc: func() (map[string]string, error) {
			return map[string]string{}, nil
		},
	}
	dns := &mockDNSResolver{
		resolveFunc: func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}
	oui := &mockOUILookup{
		lookupFunc: func(mac string) string {
			return "Unknown"
		},
	}

	orch := NewOrchestrator(pinger, arp, dns, oui, DefaultScanConfig())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := orch.Scan(ctx, "192.168.1.0/24")
	if err == nil {
		t.Fatal("Scan() expected error for cancelled context, got nil")
	}
}

func TestOrchestrator_PhaseOrdering(t *testing.T) {
	// NS-4: Verify phases execute in order: ICMP → ARP → DNS → OUI
	var mu sync.Mutex
	var phaseOrder []string

	pinger := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			mu.Lock()
			phaseOrder = append(phaseOrder, "ping")
			mu.Unlock()
			return []string{"192.168.1.1"}, nil
		},
	}
	arp := &mockARPReader{
		readFunc: func() (map[string]string, error) {
			mu.Lock()
			phaseOrder = append(phaseOrder, "arp")
			mu.Unlock()
			return map[string]string{"192.168.1.1": "00:11:22:aa:bb:cc"}, nil
		},
	}
	dns := &mockDNSResolver{
		resolveFunc: func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
			mu.Lock()
			phaseOrder = append(phaseOrder, "dns")
			mu.Unlock()
			return map[string]string{"192.168.1.1": "router.home"}, nil
		},
	}
	oui := &mockOUILookup{
		lookupFunc: func(mac string) string {
			mu.Lock()
			phaseOrder = append(phaseOrder, "oui")
			mu.Unlock()
			return "Cisco"
		},
	}

	orch := NewOrchestrator(pinger, arp, dns, oui, DefaultScanConfig())
	_, err := orch.Scan(context.Background(), "192.168.1.0/24")
	if err != nil {
		t.Fatalf("Scan() returned error: %v", err)
	}

	// Verify ping happened before any other phase
	pingIdx := -1
	arpIdx := -1
	dnsIdx := -1
	ouiIdx := -1
	for i, p := range phaseOrder {
		switch p {
		case "ping":
			pingIdx = i
		case "arp":
			arpIdx = i
		case "dns":
			dnsIdx = i
		case "oui":
			ouiIdx = i
		}
	}

	if pingIdx < 0 {
		t.Fatal("ping phase never executed")
	}
	if arpIdx < 0 {
		t.Fatal("arp phase never executed")
	}
	if dnsIdx < 0 {
		t.Fatal("dns phase never executed")
	}
	if ouiIdx < 0 {
		t.Fatal("oui phase never executed")
	}

	if pingIdx > arpIdx {
		t.Error("ping phase should execute before arp phase")
	}
	if arpIdx > dnsIdx {
		t.Error("arp phase should execute before dns phase")
	}
	if dnsIdx > ouiIdx {
		t.Error("dns phase should execute before oui phase")
	}
}

func TestOrchestrator_EmptySubnet(t *testing.T) {
	// Pipeline handles empty ping results gracefully
	pinger := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			return []string{}, nil
		},
	}
	arp := &mockARPReader{
		readFunc: func() (map[string]string, error) {
			return map[string]string{}, nil
		},
	}
	dns := &mockDNSResolver{
		resolveFunc: func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}
	oui := &mockOUILookup{
		lookupFunc: func(mac string) string {
			return "Unknown"
		},
	}

	orch := NewOrchestrator(pinger, arp, dns, oui, DefaultScanConfig())
	result, err := orch.Scan(context.Background(), "10.0.0.0/24")
	if err != nil {
		t.Fatalf("Scan() returned error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("Total = %d, want 0", result.Total)
	}
	if len(result.Devices) != 0 {
		t.Errorf("len(Devices) = %d, want 0", len(result.Devices))
	}
}
