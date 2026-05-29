package scanner

import (
	"bufio"
	_ "embed"
	"strings"
	"sync"
)

//go:embed oui_data/oui.csv
var ouiData []byte

var (
	ouiOnce   sync.Once
	ouiPrefix map[string]string
)

// parseOUIData parses the embedded OUI CSV data.
// Format: prefix,vendor (one per line).
func parseOUIData(data []byte) map[string]string {
	m := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			continue
		}
		prefix := strings.ToUpper(strings.TrimSpace(parts[0]))
		vendor := strings.TrimSpace(parts[1])
		if prefix != "" && vendor != "" {
			m[prefix] = vendor
		}
	}
	return m
}

// initOUIPrefix loads the OUI prefix map on first call.
func initOUIPrefix() {
	ouiOnce.Do(func() {
		ouiPrefix = parseOUIData(ouiData)
	})
}

// LookupOUI returns the vendor name for a given MAC address.
// Returns "Unknown" if the MAC prefix is not found in the OUI database.
func LookupOUI(mac string) string {
	initOUIPrefix()

	if mac == "" {
		return "Unknown"
	}

	// Normalize to uppercase
	mac = strings.ToUpper(strings.TrimSpace(mac))

	// Extract the first 3 octets (OUI prefix)
	parts := strings.Split(mac, ":")
	if len(parts) < 3 {
		return "Unknown"
	}
	prefix := strings.Join(parts[:3], ":")

	if vendor, ok := ouiPrefix[prefix]; ok {
		return vendor
	}
	return "Unknown"
}

// IsLocalMAC returns true if the MAC address is locally administered
// (randomized/private). iOS 14+, Android 10+, and Windows 10 use
// locally administered MACs by default per network.
func IsLocalMAC(mac string) bool {
	if mac == "" {
		return false
	}
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(mac)), ":")
	if len(parts) < 1 {
		return false
	}
	// The second-least-significant bit of the first octet indicates
	// locally administered (0x02 mask). E.g., ca:7a:77 → 0xCA & 0x02 = 0x02
	if len(parts[0]) < 2 {
		return false
	}
	b := (hexDigit(parts[0][0]) << 4) | hexDigit(parts[0][1])
	return b&0x02 != 0
}

// hexDigit converts a hex character (0-9, A-F) to its numeric value.
func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'F':
		return int(c - 'A' + 10)
	default:
		return 0
	}
}
