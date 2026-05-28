package scanner

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

// ARPEntry represents a parsed /proc/net/arp entry.
type ARPEntry struct {
	IP   string
	MAC  string
	Device string
}

// ParseARPTable parses /proc/net/arp format from an io.Reader.
// Returns a map of IP address to MAC address for entries with valid MACs.
func ParseARPTable(r io.Reader) (map[string]string, error) {
	entries := make(map[string]string)
	scanner := bufio.NewScanner(r)

	// Skip header line
	if !scanner.Scan() {
		return entries, nil
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		ip := fields[0]
		mac := fields[3]

		// Skip incomplete entries
		if mac == "" || strings.Count(mac, ":") < 5 {
			continue
		}

		// Validate MAC format
		if _, err := net.ParseMAC(mac); err != nil {
			continue
		}

		entries[ip] = mac
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan arp table: %w", err)
	}

	return entries, nil
}

// ReadARPFile parses /proc/net/arp and returns the IP-to-MAC mapping.
func ReadARPFile() (map[string]string, error) {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil, fmt.Errorf("open /proc/net/arp: %w", err)
	}
	defer f.Close()

	return ParseARPTable(f)
}
