package scanner

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/sys/unix"
)

const (
	mdnsAddr    = "224.0.0.251:5353"
	mdnsTimeout = 1 * time.Second
)

// buildRevName builds a reverse DNS lookup name from an IP.
func buildRevName(ip string) string {
	parts := strings.Split(ip, ".")
	return fmt.Sprintf("%s.%s.%s.%s.in-addr.arpa.", parts[3], parts[2], parts[1], parts[0])
}

// buildPTRQuery builds a DNS PTR query packet for the given IP.
func buildPTRQuery(ip string) (string, []byte, error) {
	revName := buildRevName(ip)
	msg := dnsmessage.Message{
		Header: dnsmessage.Header{ID: 0, RecursionDesired: false},
		Questions: []dnsmessage.Question{{
			Name:  dnsmessage.MustNewName(revName),
			Type:  dnsmessage.TypePTR,
			Class: dnsmessage.ClassINET | 0x8000,
		}},
	}

	packed, err := msg.Pack()
	if err != nil {
		return "", nil, fmt.Errorf("pack mdns query: %w", err)
	}
	return revName, packed, nil
}

// ResolveMDNSBatch performs mDNS reverse lookups for a batch of IPs
// using a single shared socket with multicast membership.
func ResolveMDNSBatch(ctx context.Context, ips []string, timeout time.Duration) (map[string]string, error) {
	if len(ips) == 0 {
		return map[string]string{}, nil
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, fmt.Errorf("listen udp: %w", err)
	}
	defer conn.Close()

	// Join mDNS multicast group via raw syscall
	rc, err := conn.SyscallConn()
	if err == nil {
		rc.Control(func(fd uintptr) {
			unix.SetsockoptIPMreq(int(fd), unix.IPPROTO_IP, unix.IP_ADD_MEMBERSHIP,
				&unix.IPMreq{
					Multiaddr: [4]byte{224, 0, 0, 251},
					Interface: [4]byte{0, 0, 0, 0},
				})
		})
	}

	dst, err := net.ResolveUDPAddr("udp4", mdnsAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve mdns addr: %w", err)
	}

	pktIndex := make(map[string]string)
	for _, ip := range ips {
		if _, _, err := net.ParseCIDR(ip + "/32"); err != nil {
			continue
		}
		revName, pkt, err := buildPTRQuery(ip)
		if err != nil {
			continue
		}
		pktIndex[revName] = ip
		if _, err := conn.WriteTo(pkt, dst); err != nil {
			continue
		}
	}

	result := make(map[string]string, len(pktIndex))
	buf := make([]byte, 1500)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return result, nil
		default:
		}

		if err := conn.SetReadDeadline(deadline); err != nil {
			break
		}

		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			break
		}

		var resp dnsmessage.Message
		if err := resp.Unpack(buf[:n]); err != nil {
			continue
		}

		ip, hostname := matchPTR(&resp, pktIndex)
		if ip != "" && hostname != "" {
			if _, exists := result[ip]; !exists {
				result[ip] = hostname
			}
		}

		if len(result) == len(pktIndex) {
			break
		}
	}

	return result, nil
}

// matchPTR extracts (ip, hostname) from a DNS response by matching
// PTR records against our reverse-name index first, then falling back
// to A/AAAA records whose Name contains ".local" (Apple devices respond
// with an A record instead of PTR for reverse queries).
func matchPTR(msg *dnsmessage.Message, revIndex map[string]string) (string, string) {
	targetIPs := make(map[string]bool, len(revIndex))
	for _, ip := range revIndex {
		targetIPs[ip] = true
	}

	for _, set := range [][]dnsmessage.Resource{msg.Answers, msg.Additionals} {
		for _, ans := range set {
			if ans.Header.Type == dnsmessage.TypePTR {
				ptr, ok := ans.Body.(*dnsmessage.PTRResource)
				if !ok {
					continue
				}
				if ip, hit := revIndex[ans.Header.Name.String()]; hit {
					return ip, strings.TrimSuffix(ptr.PTR.String(), ".")
				}
			}
		}

		for _, ans := range set {
			name := strings.ToLower(ans.Header.Name.String())
			if !strings.Contains(name, ".local") {
				continue
			}
			switch body := ans.Body.(type) {
			case *dnsmessage.AResource:
				ip := net.IP(body.A[:]).String()
				if targetIPs[ip] {
					return ip, strings.TrimSuffix(ans.Header.Name.String(), ".")
				}
			case *dnsmessage.AAAAResource:
				ip := net.IP(body.AAAA[:]).String()
				if targetIPs[ip] {
					return ip, strings.TrimSuffix(ans.Header.Name.String(), ".")
				}
			}
		}
	}
	return "", ""
}
