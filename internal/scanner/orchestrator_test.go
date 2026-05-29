package scanner

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/andresar/netwatch/internal/models"
)

// noopMDNS is a no-op mDNS resolver for tests that don't test mDNS.
type noopMDNS struct{}

func (n *noopMDNS) ResolveMDNS(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
	return map[string]string{}, nil
}

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

	orch := NewOrchestrator(pinger, arp, dns, &noopMDNS{}, oui, DefaultScanConfig())
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
	orch := NewOrchestrator(pinger, arp, dns, &noopMDNS{}, oui, cfg)

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

	orch := NewOrchestrator(pinger, arp, dns, &noopMDNS{}, oui, DefaultScanConfig())

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

	orch := NewOrchestrator(pinger, arp, dns, &noopMDNS{}, oui, DefaultScanConfig())
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

func TestOrchestrator_ARPBackfill(t *testing.T) {
	// When ICMP returns nothing but ARP has entries, union should
	// include ARP-only IPs (e.g. iOS devices in sleep mode)
	pinger := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			return []string{}, nil
		},
	}
	arp := &mockARPReader{
		readFunc: func() (map[string]string, error) {
			return map[string]string{
				"192.168.1.10": "ca:7a:77:f9:56:05",
				"192.168.1.20": "2c:96:82:75:69:e8",
			}, nil
		},
	}
	dns := &mockDNSResolver{
		resolveFunc: func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
			return map[string]string{"192.168.1.10": "iphone-de-casa"}, nil
		},
	}
	oui := &mockOUILookup{
		lookupFunc: func(mac string) string {
			if mac == "2c:96:82:75:69:e8" {
				return "MitraStar"
			}
			return "Unknown"
		},
	}

	orch := NewOrchestrator(pinger, arp, dns, &noopMDNS{}, oui, DefaultScanConfig())
	result, err := orch.Scan(context.Background(), "192.168.1.0/24")
	if err != nil {
		t.Fatalf("Scan() returned error: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("Total = %d, want 2 (ARP-backfilled)", result.Total)
	}

	// Device 1: ARP-only, local MAC
	for _, d := range result.Devices {
		switch d.IP {
		case "192.168.1.10":
			if d.MAC != "ca:7a:77:f9:56:05" {
				t.Errorf("MAC = %q, want ca:7a:77:f9:56:05", d.MAC)
			}
			if !d.LocalMAC {
				t.Error("LocalMAC = false, want true (randomized MAC)")
			}
			if d.Hostname != "iphone-de-casa" {
				t.Errorf("Hostname = %q, want iphone-de-casa", d.Hostname)
			}
			if d.Vendor != "Unknown" {
				t.Errorf("Vendor = %q, want Unknown (randomized MAC)", d.Vendor)
			}
		case "192.168.1.20":
			if d.MAC != "2c:96:82:75:69:e8" {
				t.Errorf("MAC = %q, want 2c:96:82:75:69:e8", d.MAC)
			}
			if d.LocalMAC {
				t.Error("LocalMAC = true, want false (universal MAC)")
			}
			if d.Vendor != "MitraStar" {
				t.Errorf("Vendor = %q, want MitraStar", d.Vendor)
			}
		default:
			t.Errorf("Unexpected device IP: %s", d.IP)
		}
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

	orch := NewOrchestrator(pinger, arp, dns, &noopMDNS{}, oui, DefaultScanConfig())
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

type mockMDNSResolver struct {
	resolveFunc func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error)
}

func (m *mockMDNSResolver) ResolveMDNS(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
	return m.resolveFunc(ctx, ips, timeout)
}

func TestOrchestrator_MDNSEnrichment(t *testing.T) {
	// When DNS returns empty for an IP but mDNS has it, mDNS hostname is used
	pinger := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			return []string{"192.168.1.100", "192.168.1.200"}, nil
		},
	}
	arp := &mockARPReader{
		readFunc: func() (map[string]string, error) {
			return map[string]string{
				"192.168.1.100": "ca:7a:77:f9:56:05",
				"192.168.1.200": "00:11:22:33:44:55",
			}, nil
		},
	}
	dns := &mockDNSResolver{
		resolveFunc: func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
			return map[string]string{"192.168.1.200": "printer.home"}, nil
		},
	}
	mdns := &mockMDNSResolver{
		resolveFunc: func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
			return map[string]string{"192.168.1.100": "iPhone-de-Andres.local"}, nil
		},
	}
	oui := &mockOUILookup{
		lookupFunc: func(mac string) string { return "Unknown" },
	}

	orch := NewOrchestrator(pinger, arp, dns, mdns, oui, DefaultScanConfig())
	result, err := orch.Scan(context.Background(), "192.168.1.0/24")
	if err != nil {
		t.Fatalf("Scan() returned error: %v", err)
	}

	var ip100, ip200 *models.Device
	for i, d := range result.Devices {
		switch d.IP {
		case "192.168.1.100":
			ip100 = &result.Devices[i]
		case "192.168.1.200":
			ip200 = &result.Devices[i]
		}
	}

	if ip100 == nil {
		t.Fatal("192.168.1.100 not in results")
	}
	if ip200 == nil {
		t.Fatal("192.168.1.200 not in results")
	}
	if ip100.Hostname != "iPhone-de-Andres.local" {
		t.Errorf("ip100 hostname = %q, want %q", ip100.Hostname, "iPhone-de-Andres.local")
	}
	if ip200.Hostname != "printer.home" {
		t.Errorf("ip200 hostname = %q, want %q", ip200.Hostname, "printer.home")
	}
}

func TestOrchestrator_MDNSFailsGracefully(t *testing.T) {
	// When mDNS returns error, devices still appear without hostname
	pinger := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			return []string{"192.168.1.100"}, nil
		},
	}
	arp := &mockARPReader{
		readFunc: func() (map[string]string, error) {
			return map[string]string{"192.168.1.100": "ca:7a:77:f9:56:05"}, nil
		},
	}
	dns := &mockDNSResolver{
		resolveFunc: func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}
	mdns := &mockMDNSResolver{
		resolveFunc: func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
			return nil, fmt.Errorf("mDNS timeout")
		},
	}
	oui := &mockOUILookup{
		lookupFunc: func(mac string) string { return "Unknown" },
	}

	orch := NewOrchestrator(pinger, arp, dns, mdns, oui, DefaultScanConfig())
	result, err := orch.Scan(context.Background(), "192.168.1.0/24")
	if err != nil {
		t.Fatalf("Scan() returned error: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Total = %d, want 1", result.Total)
	}
	if result.Devices[0].Hostname != "" {
		t.Errorf("expected empty hostname on mDNS failure, got %q", result.Devices[0].Hostname)
	}
}

func TestOrchestrator_MDNSNotCalledWhenAllHostnamesResolved(t *testing.T) {
	// mDNS should not be called if DNS resolved all hostnames
	pinger := &mockPinger{
		pingFunc: func(ctx context.Context, subnet string, concurrency int) ([]string, error) {
			return []string{"192.168.1.1"}, nil
		},
	}
	arp := &mockARPReader{
		readFunc: func() (map[string]string, error) {
			return map[string]string{"192.168.1.1": "00:11:22:aa:bb:cc"}, nil
		},
	}
	dns := &mockDNSResolver{
		resolveFunc: func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
			return map[string]string{"192.168.1.1": "gateway.home"}, nil
		},
	}
	mdnsCalled := false
	mdns := &mockMDNSResolver{
		resolveFunc: func(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
			mdnsCalled = true
			return map[string]string{}, nil
		},
	}
	oui := &mockOUILookup{
		lookupFunc: func(mac string) string { return "Cisco" },
	}

	orch := NewOrchestrator(pinger, arp, dns, mdns, oui, DefaultScanConfig())
	_, err := orch.Scan(context.Background(), "192.168.1.0/24")
	if err != nil {
		t.Fatalf("Scan() returned error: %v", err)
	}
	if mdnsCalled {
		t.Error("mDNS was called but all IPs already had hostnames from DNS")
	}
}
