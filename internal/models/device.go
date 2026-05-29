package models

import (
	"encoding/json"
	"time"
)

// Device represents a discovered network device.
type Device struct {
	IP       string `json:"ip"`
	MAC      string `json:"mac"`
	Hostname string `json:"hostname,omitempty"`
	Vendor   string `json:"vendor,omitempty"`
	LocalMAC bool   `json:"local_mac,omitempty"`
}

// ScanResult holds the output of a network scan.
type ScanResult struct {
	Devices   []Device  `json:"devices"`
	ScannedAt time.Time `json:"scanned_at"`
	Total     int       `json:"total"`
}

// UnmarshalJSON implements json.Unmarshaler and normalizes null devices to empty slice.
func (sr *ScanResult) UnmarshalJSON(data []byte) error {
	type Alias ScanResult
	aux := &struct{ *Alias }{Alias: (*Alias)(sr)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if sr.Devices == nil {
		sr.Devices = []Device{}
	}
	return nil
}
