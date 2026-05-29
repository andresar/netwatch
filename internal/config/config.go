package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	Subnet          string
	Port            int
	PingConcurrency int
	PingPrivileged  bool
	ScanTimeout     time.Duration
	DNSTimeout      time.Duration
}

// Load reads configuration from environment variables with NETWATCH_ prefix.
// Returns an error if required fields are missing or invalid.
func Load() (*Config, error) {
	subnet := os.Getenv("NETWATCH_SUBNET")
	if subnet == "" {
		return nil, fmt.Errorf("NETWATCH_SUBNET is required")
	}
	if _, _, err := net.ParseCIDR(subnet); err != nil {
		return nil, fmt.Errorf("NETWATCH_SUBNET %q is not a valid CIDR: %w", subnet, err)
	}

	port := 8080
	if p := os.Getenv("NETWATCH_PORT"); p != "" {
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("NETWATCH_PORT %q is not a valid port: %w", p, err)
		}
		port = v
	}

	concurrency := 64
	if c := os.Getenv("NETWATCH_PING_CONCURRENCY"); c != "" {
		v, err := strconv.Atoi(c)
		if err != nil {
			return nil, fmt.Errorf("NETWATCH_PING_CONCURRENCY %q is not a valid number: %w", c, err)
		}
		concurrency = v
	}

	scanTimeout := 30 * time.Second
	if t := os.Getenv("NETWATCH_SCAN_TIMEOUT"); t != "" {
		v, err := time.ParseDuration(t)
		if err != nil {
			return nil, fmt.Errorf("NETWATCH_SCAN_TIMEOUT %q is not a valid duration: %w", t, err)
		}
		scanTimeout = v
	}

	dnsTimeout := 1 * time.Second
	if t := os.Getenv("NETWATCH_DNS_TIMEOUT"); t != "" {
		v, err := time.ParseDuration(t)
		if err != nil {
			return nil, fmt.Errorf("NETWATCH_DNS_TIMEOUT %q is not a valid duration: %w", t, err)
		}
		dnsTimeout = v
	}

	return &Config{
		Subnet:          subnet,
		Port:            port,
		PingConcurrency: concurrency,
		ScanTimeout:     scanTimeout,
		DNSTimeout:      dnsTimeout,
	}, nil
}
