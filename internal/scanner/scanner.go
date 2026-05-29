package scanner

import (
	"context"
	"time"

	"github.com/andresar/netwatch/internal/models"
)

// Scanner defines the network scanning interface.
// Implementations must support context cancellation and timeout.
type Scanner interface {
	Scan(ctx context.Context, subnet string) (*models.ScanResult, error)
}

// ScanConfig holds configuration for the scanning pipeline.
type ScanConfig struct {
	Subnet          string
	PingConcurrency int
	ScanTimeout     time.Duration
	DNSTimeout      time.Duration
}

// DefaultScanConfig returns a ScanConfig with sensible defaults.
func DefaultScanConfig() ScanConfig {
	return ScanConfig{
		PingConcurrency: 64,
		ScanTimeout:     30 * time.Second,
		DNSTimeout: 1 * time.Second,
	}
}

// Pinger performs ICMP echo sweeps across a subnet.
type Pinger interface {
	Ping(ctx context.Context, subnet string, concurrency int) ([]string, error)
}

// ARPReader parses the system ARP table to resolve MAC addresses.
type ARPReader interface {
	Read() (map[string]string, error)
}

// DNSResolver performs reverse DNS lookups for a set of IPs.
type DNSResolver interface {
	Resolve(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error)
}

// OUILookup resolves vendor information from MAC addresses.
type OUILookup interface {
	Lookup(mac string) string
}

// MDNSResolver performs mDNS reverse lookups to discover .local hostnames.
type MDNSResolver interface {
	ResolveMDNS(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error)
}
