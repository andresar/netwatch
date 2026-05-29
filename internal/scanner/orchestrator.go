package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/andresar/netwatch/internal/models"
)

// Orchestrator runs the scan pipeline: ICMP → ARP → DNS → mDNS → OUI.
type Orchestrator struct {
	pinger       Pinger
	arpReader    ARPReader
	dnsResolver  DNSResolver
	mdnsResolver MDNSResolver
	ouiLookup    OUILookup
	config       ScanConfig
}

// NewOrchestrator creates a new scan pipeline coordinator.
func NewOrchestrator(pinger Pinger, arpReader ARPReader, dnsResolver DNSResolver, mdnsResolver MDNSResolver, ouiLookup OUILookup, config ScanConfig) *Orchestrator {
	return &Orchestrator{
		pinger:       pinger,
		arpReader:    arpReader,
		dnsResolver:  dnsResolver,
		mdnsResolver: mdnsResolver,
		ouiLookup:    ouiLookup,
		config:       config,
	}
}

// mergeIPs unions two IP slices, deduplicating while preserving order.
// aliveIPs come first (responded to ICMP), then any ARP-only IPs.
func mergeIPs(aliveIPs []string, arpIPs []string) []string {
	seen := make(map[string]bool, len(aliveIPs)+len(arpIPs))
	result := make([]string, 0, len(aliveIPs)+len(arpIPs))
	for _, ip := range aliveIPs {
		if !seen[ip] {
			seen[ip] = true
			result = append(result, ip)
		}
	}
	for _, ip := range arpIPs {
		if !seen[ip] {
			seen[ip] = true
			result = append(result, ip)
		}
	}
	return result
}

// Scan executes the full pipeline: ICMP → ARP → DNS → OUI.
// ICMP pings discover live hosts. ARP enriches with MACs AND provides
// additional IPs that didn't respond to ping (e.g., iOS devices in sleep).
// The final device list is the union of both sources.
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

	// Union: combine ICMP-alive IPs with ARP-visible IPs
	arpIPs := make([]string, 0, len(arpEntries))
	for ip := range arpEntries {
		arpIPs = append(arpIPs, ip)
	}
	allIPs := mergeIPs(aliveIPs, arpIPs)

	// Phase 3: DNS Resolution — reverse lookup hostnames
	hostnames, err := o.dnsResolver.Resolve(scanCtx, allIPs, o.config.DNSTimeout)
	if err != nil {
		return nil, fmt.Errorf("dns phase: %w", err)
	}

	// Phase 4: mDNS Enrichment — resolve .local hostnames for IPs without DNS
	var mdnsIPs []string
	for _, ip := range allIPs {
		if hostnames[ip] == "" {
			mdnsIPs = append(mdnsIPs, ip)
		}
	}
	if len(mdnsIPs) > 0 && o.mdnsResolver != nil {
		mdnsNames, err := o.mdnsResolver.ResolveMDNS(scanCtx, mdnsIPs, o.config.DNSTimeout)
		if err == nil {
			for ip, name := range mdnsNames {
				hostnames[ip] = name
			}
		}
	}

	// Phase 5: Build devices with OUI vendor lookup
	devices := make([]models.Device, 0, len(allIPs))
	for _, ip := range allIPs {
		mac := arpEntries[ip]

		devices = append(devices, models.Device{
			IP:       ip,
			MAC:      mac,
			Hostname: hostnames[ip],
			Vendor:   o.ouiLookup.Lookup(mac),
			LocalMAC: mac != "" && IsLocalMAC(mac),
		})
	}

	return &models.ScanResult{
		Devices:   devices,
		ScannedAt: time.Now(),
		Total:     len(devices),
	}, nil
}
