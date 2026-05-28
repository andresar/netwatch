package scanner

import (
	"strings"
	"testing"
)

func TestParseARPTABLE(t *testing.T) {
	// NS-3: Parse valid /proc/net/arp content
	sample := `IP address       HW type     Flags       HW address            Mask     Device
192.168.1.1      0x1         0x2         00:11:22:aa:bb:cc     *        eth0
192.168.1.100    0x1         0x2         aa:bb:cc:dd:ee:ff     *        eth0
192.168.1.200    0x1         0x2         invalid                *        eth0
`

	entries, err := ParseARPTable(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("ParseARPTable() returned error: %v", err)
	}

	// Valid entry with proper MAC
	if mac, ok := entries["192.168.1.1"]; !ok {
		t.Error("entries missing 192.168.1.1")
	} else if mac != "00:11:22:aa:bb:cc" {
		t.Errorf("entries[192.168.1.1] = %q, want %q", mac, "00:11:22:aa:bb:cc")
	}

	// Another valid entry
	if mac, ok := entries["192.168.1.100"]; !ok {
		t.Error("entries missing 192.168.1.100")
	} else if mac != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("entries[192.168.1.100] = %q, want %q", mac, "aa:bb:cc:dd:ee:ff")
	}

	// Invalid MAC should be skipped
	if _, ok := entries["192.168.1.200"]; ok {
		t.Error("entries should not include 192.168.1.200 with invalid MAC")
	}
}

func TestParseARPTABLE_MissingEntry(t *testing.T) {
	// NS-3: IP with no ARP entry — MAC missing from result
	sample := `IP address       HW type     Flags       HW address            Mask     Device
192.168.1.1      0x1         0x2         00:11:22:aa:bb:cc     *        eth0
`

	entries, err := ParseARPTable(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("ParseARPTable() returned error: %v", err)
	}

	if _, ok := entries["192.168.1.99"]; ok {
		t.Error("entries should not contain unknown IP 192.168.1.99")
	}
}

func TestParseARPTABLE_Empty(t *testing.T) {
	// Empty ARP table returns empty map
	sample := `IP address       HW type     Flags       HW address            Mask     Device
`
	entries, err := ParseARPTable(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("ParseARPTable() returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0", len(entries))
	}
}

func TestParseARPTABLE_IncompleteEntry(t *testing.T) {
	// Entry with incomplete fields should be skipped
	sample := `IP address       HW type     Flags       HW address            Mask     Device
192.168.1.1      0x1         0x2         00:11:22:aa:bb:cc     *        eth0
192.168.1.5      0x1         0x2                                 *        eth0
`

	entries, err := ParseARPTable(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("ParseARPTable() returned error: %v", err)
	}

	if _, ok := entries["192.168.1.5"]; ok {
		t.Error("entries should not include 192.168.1.5 with missing MAC")
	}
	if mac, ok := entries["192.168.1.1"]; !ok {
		t.Error("entries missing 192.168.1.1")
	} else if mac != "00:11:22:aa:bb:cc" {
		t.Errorf("entries[192.168.1.1] = %q, want %q", mac, "00:11:22:aa:bb:cc")
	}
}

func TestParseARPTABLE_CompleteEntry(t *testing.T) {
	// NS-3: Live host with valid ARP entry populates MAC
	sample := `IP address       HW type     Flags       HW address            Mask     Device
192.168.1.1      0x1         0x2         00:11:22:33:44:55     *        eth0
`
	entries, err := ParseARPTable(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("ParseARPTable() returned error: %v", err)
	}
	mac, ok := entries["192.168.1.1"]
	if !ok {
		t.Fatal("entries missing 192.168.1.1")
	}
	if mac != "00:11:22:33:44:55" {
		t.Errorf("entries[192.168.1.1] = %q, want %q", mac, "00:11:22:33:44:55")
	}
}
