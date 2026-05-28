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
