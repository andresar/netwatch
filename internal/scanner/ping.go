package scanner

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/go-ping/ping"
	"golang.org/x/sync/errgroup"
)

// defaultPingCount is the number of echo requests per target.
const defaultPingCount = 1

// pingTimeout is the timeout per individual ping probe.
const pingTimeout = 2 * time.Second

// privilegedPing controls whether ICMP uses raw sockets (privileged) or
// unprivileged UDP-mode ICMP. Auto-detected at startup.
var privilegedPing bool

// SetPrivilegedPing configures the ICMP ping mode.
// Set to true when CAP_NET_RAW is available (e.g., Docker with --cap-add).
// Set to false for unprivileged ICMP (works on modern Linux kernels).
func SetPrivilegedPing(privileged bool) {
	privilegedPing = privileged
}

// IPsInCIDR returns all usable host IPs in a CIDR subnet.
// Excludes the network address and broadcast address.
func IPsInCIDR(cidr string) ([]string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("parse cidr %q: %w", cidr, err)
	}

	ones, bits := ipnet.Mask.Size()
	size := 1 << (bits - ones)

	if size < 4 {
		// For /31 or /32, return all usable
		if size == 2 {
			// /31: both addresses are usable (RFC 3021)
			ips := make([]string, 0, size)
			ip := ipnet.IP.Mask(ipnet.Mask)
			for i := 0; i < size; i++ {
				ips = append(ips, ip.String())
				ip = nextIP(ip)
			}
			return ips, nil
		}
		if size == 1 {
			// /32: loopback
			return []string{ipnet.IP.String()}, nil
		}
	}

	ips := make([]string, 0, size-2)
	ip := ipnet.IP.Mask(ipnet.Mask)
	// Skip network address (first)
	ip = nextIP(ip)
	for i := 1; i < size-1; i++ {
		ips = append(ips, ip.String())
		ip = nextIP(ip)
	}
	return ips, nil
}

// nextIP increments an IP address by one.
func nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for j := len(next) - 1; j >= 0; j-- {
		next[j]++
		if next[j] > 0 {
			break
		}
	}
	return next
}

// pingHost sends a single ICMP echo to the target and returns true if alive.
func pingHost(ctx context.Context, target string) (bool, error) {
	pinger, err := ping.NewPinger(target)
	if err != nil {
		return false, fmt.Errorf("create pinger for %q: %w", target, err)
	}

	pinger.SetPrivileged(privilegedPing)
	pinger.Count = defaultPingCount
	pinger.Timeout = pingTimeout

	// Run in a separate goroutine to respect context cancellation
	type result struct {
		alive bool
		err   error
	}

	ch := make(chan result, 1)
	go func() {
		err := pinger.Run()
		alive := pinger.Statistics().PacketsRecv > 0
		ch <- result{alive, err}
		close(ch)
	}()

	select {
	case <-ctx.Done():
		pinger.Stop()
		return false, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return false, r.err
		}
		return r.alive, nil
	}
}

// PingICMP performs a full ICMP echo sweep across the given subnet.
// It respects context cancellation and limits concurrency.
func PingICMP(ctx context.Context, subnet string, concurrency int) ([]string, error) {
	ips, err := IPsInCIDR(subnet)
	if err != nil {
		return nil, err
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	var mu sync.Mutex
	var alive []string

	for _, ip := range ips {
		ip := ip // capture
		g.Go(func() error {
			ok, err := pingHost(ctx, ip)
			if err != nil {
				// Ignore individual ping errors — they just mean the host is down
				return nil
			}
			if ok {
				mu.Lock()
				alive = append(alive, ip)
				mu.Unlock()
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	if alive == nil {
		alive = []string{}
	}
	return alive, nil
}
