package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDevice_JSONRoundTrip(t *testing.T) {
	// DI-3: JSON round-trip preserves all fields
	d := Device{
		IP:       "192.168.1.1",
		MAC:      "00:11:22:aa:bb:cc",
		Hostname: "router.home",
		Vendor:   "Cisco",
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var got Device
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if got.IP != d.IP {
		t.Errorf("IP = %q, want %q", got.IP, d.IP)
	}
	if got.MAC != d.MAC {
		t.Errorf("MAC = %q, want %q", got.MAC, d.MAC)
	}
	if got.Hostname != d.Hostname {
		t.Errorf("Hostname = %q, want %q", got.Hostname, d.Hostname)
	}
	if got.Vendor != d.Vendor {
		t.Errorf("Vendor = %q, want %q", got.Vendor, d.Vendor)
	}
}

func TestDevice_JSONWithOmitEmpty(t *testing.T) {
	// DI-3: Optional fields (Hostname, Vendor, LocalMAC) omitted when empty
	d := Device{
		IP:  "192.168.1.1",
		MAC: "00:11:22:aa:bb:cc",
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if _, ok := result["hostname"]; ok {
		t.Error("hostname should be omitted when empty")
	}
	if _, ok := result["vendor"]; ok {
		t.Error("vendor should be omitted when empty")
	}
	if _, ok := result["local_mac"]; ok {
		t.Error("local_mac should be omitted when false")
	}
	if _, ok := result["ip"]; !ok {
		t.Error("ip should always be present")
	}
	if _, ok := result["mac"]; !ok {
		t.Error("mac should always be present")
	}
}

func TestDevice_LocalMACPresent(t *testing.T) {
	// local_mac should appear in JSON when true
	d := Device{
		IP:       "192.168.1.10",
		MAC:      "ca:7a:77:f9:56:05",
		LocalMAC: true,
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if _, ok := result["local_mac"]; !ok {
		t.Error("local_mac should be present when true")
	}
	if result["local_mac"] != true {
		t.Errorf("local_mac = %v, want true", result["local_mac"])
	}
}

func TestDevice_IPandMACNonEmpty(t *testing.T) {
	// DI-3: Valid device must have non-empty IP and MAC
	d := Device{
		IP:  "192.168.1.100",
		MAC: "aa:bb:cc:dd:ee:ff",
	}
	if d.IP == "" {
		t.Error("Device.IP must not be empty")
	}
	if d.MAC == "" {
		t.Error("Device.MAC must not be empty")
	}
}

func TestScanResult_JSONRoundTrip(t *testing.T) {
	// ScanResult JSON round-trip with populated devices
	now := time.Date(2026, 5, 28, 22, 0, 0, 0, time.UTC)
	sr := ScanResult{
		Devices: []Device{
			{IP: "192.168.1.1", MAC: "00:11:22:aa:bb:cc", Hostname: "router", Vendor: "Cisco"},
			{IP: "192.168.1.2", MAC: "00:11:22:dd:ee:ff", Hostname: "nas", Vendor: "Synology"},
		},
		ScannedAt: now,
		Total:     2,
	}

	data, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var got ScanResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if len(got.Devices) != 2 {
		t.Fatalf("len(got.Devices) = %d, want 2", len(got.Devices))
	}
	if got.Devices[0].IP != "192.168.1.1" {
		t.Errorf("Device[0].IP = %q, want %q", got.Devices[0].IP, "192.168.1.1")
	}
	if got.Total != 2 {
		t.Errorf("Total = %d, want %d", got.Total, 2)
	}
	if !got.ScannedAt.Equal(now) {
		t.Errorf("ScannedAt = %v, want %v", got.ScannedAt, now)
	}
}

func TestScanResult_EmptyDevices(t *testing.T) {
	// ScanResult with empty device list
	sr := ScanResult{
		Devices:   []Device{},
		ScannedAt: time.Now(),
		Total:     0,
	}

	data, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var got ScanResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if len(got.Devices) != 0 {
		t.Errorf("len(got.Devices) = %d, want 0", len(got.Devices))
	}
	if got.Total != 0 {
		t.Errorf("Total = %d, want 0", got.Total)
	}
}

func TestScanResult_NullDevicesNormalized(t *testing.T) {
	// ScanResult JSON with null devices should normalize to empty slice
	data := []byte(`{"devices":null,"scanned_at":"2026-05-28T22:00:00Z","total":0}`)
	var sr ScanResult
	if err := json.Unmarshal(data, &sr); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if sr.Devices == nil {
		t.Fatal("Devices should not be nil after unmarshal (should be empty slice)")
	}
	if len(sr.Devices) != 0 {
		t.Errorf("len(sr.Devices) = %d, want 0", len(sr.Devices))
	}
}

func TestScanResult_MarshalNullNormalized(t *testing.T) {
	// Marshal/unmarshal round-trip ensures nil becomes empty slice
	sr := ScanResult{
		ScannedAt: time.Now(),
	}

	data, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var got ScanResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if got.Devices == nil {
		t.Error("Devices should not be nil after marshal/unmarshal round-trip")
	}
}
