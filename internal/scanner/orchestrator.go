package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/andresar/netwatch/internal/models"
)

// Orchestrator runs the scan pipeline: ICMP → ARP → DNS → OUI.
type Orchestrator struct {
	pinger      Pinger
	arpReader   ARPReader
	dnsResolver DNSResolver
	ouiLookup   OUILookup
	config      ScanConfig
}

// NewOrchestrator creates a new scan pipeline coordinator.
func NewOrchestrator(pinger Pinger, arpReader ARPReader, dnsResolver DNSResolver, ouiLookup OUILookup, config ScanConfig) *Orchestrator {
	return &Orchestrator{
		pinger:      pinger,
		arpReader:   arpReader,
		dnsResolver: dnsResolver,
		ouiLookup:   ouiLookup,
		config:      config,
	}
}

// Scan executes the full pipeline: ICMP → ARP → DNS → OUI.
// Phases run sequentially — each phase consumes the output of the previous.
func (o *Orchestrator) Scan(ctx context.Context, subnet string) (*models.ScanResult, error) {
	scanCtx, cancel := context.WithTimeout(ctx, o.config.ScanTimeout)
	defer cancel()

	// Phase 1: ICMP Ping Sweep — discover live hosts
	aliveIPs, err := o.pinger.Ping(scanCtx, subnet, o.config.PingConcurrency)
	if err != nil {
		return nil, fmt.Errorf("ping phase: %w", err)
	}

	// Phase 2: ARP Table Read — resolve MAC addresses
	arpEntries, err := o.arpReader.Read()
	if err != nil {
		return nil, fmt.Errorf("arp phase: %w", err)
	}

	// Phase 3: DNS Resolution — reverse lookup hostnames
	hostnames, err := o.dnsResolver.Resolve(scanCtx, aliveIPs, o.config.DNSTimeout)
	if err != nil {
		return nil, fmt.Errorf("dns phase: %w", err)
	}

	// Phase 4: Build devices with OUI vendor lookup
	devices := make([]models.Device, 0, len(aliveIPs))
	for _, ip := range aliveIPs {
		mac := arpEntries[ip]
		hostname := hostnames[ip]
		vendor := o.ouiLookup.Lookup(mac)

		devices = append(devices, models.Device{
			IP:       ip,
			MAC:      mac,
			Hostname: hostname,
			Vendor:   vendor,
		})
	}

	return &models.ScanResult{
		Devices:   devices,
		ScannedAt: time.Now(),
		Total:     len(devices),
	}, nil
}
