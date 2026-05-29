//go:build mdns

package scanner

import (
	"context"
	"net"
	"os"
	"sync"
	"time"
)

// defaultMDNSTimeout is the default timeout for mDNS queries.
const defaultMDNSTimeout = 5 * time.Second

// isMDNSEnabled checks whether mDNS discovery is enabled via env var.
func isMDNSEnabled() bool {
	return os.Getenv("NETWATCH_MDNS_ENABLED") == "true"
}

// MDNSResolve performs mDNS hostname resolution for the given IPs.
// Gated by the mdns build tag (compile time) and NETWATCH_MDNS_ENABLED env var (runtime).
// Returns a map of IP to hostname. IPs without mDNS resolution are omitted.
//
// MD-1: Only available when built with -tags=mdns AND NETWATCH_MDNS_ENABLED=true.
// MD-2: Applies configurable timeout (default 5s). Timeouts return empty, not error.
func MDNSResolve(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
	if !isMDNSEnabled() {
		return map[string]string{}, nil
	}

	if len(ips) == 0 {
		return map[string]string{}, nil
	}

	if timeout <= 0 {
		timeout = defaultMDNSTimeout
	}

	var mu sync.Mutex
	hostnames := make(map[string]string, len(ips))

	var wg sync.WaitGroup
	for _, ip := range ips {
		ip := ip // capture
		wg.Add(1)
		go func() {
			defer wg.Done()

			lookupCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Use net.LookupAddr which on modern systems with Avahi/systemd-resolved
			// handles mDNS resolution for .local addresses transparently.
			names, err := net.DefaultResolver.LookupAddr(lookupCtx, ip)
			if err != nil {
				return // not an error — just no hostname
			}
			if len(names) > 0 {
				mu.Lock()
				hostnames[ip] = names[0]
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	return hostnames, nil
}
