package scanner

import (
	"context"
	"net"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// ReverseLookupBatch performs reverse DNS lookups for a batch of IPs
// using a bounded worker pool. It returns a map of IP to hostname.
// IPs without PTR records are omitted from the result map.
func ReverseLookupBatch(ctx context.Context, ips []string, timeout time.Duration, workers int) (map[string]string, error) {
	if len(ips) == 0 {
		return map[string]string{}, nil
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)

	var mu sync.Mutex
	hostnames := make(map[string]string, len(ips))

	for _, ip := range ips {
		ip := ip // capture
		g.Go(func() error {
			// Per-lookup timeout
			lookupCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			names, err := net.DefaultResolver.LookupAddr(lookupCtx, ip)
			if err != nil {
				// NXDOMAIN or timeout — not an error, just no hostname
				return nil
			}
			if len(names) > 0 {
				mu.Lock()
				hostnames[ip] = names[0]
				mu.Unlock()
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return hostnames, nil
}
