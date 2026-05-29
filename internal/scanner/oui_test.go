package scanner

import (
	"testing"
)

func TestOUILookupInterface(t *testing.T) {
	// Verify OUILookup interface is satisfiable
	var _ OUILookup = (*mockOUILookup)(nil)
}

type mockOUILookup struct {
	lookupFunc func(mac string) string
}

func (m *mockOUILookup) Lookup(mac string) string {
	return m.lookupFunc(mac)
}

func TestLookupOUI_KnownPrefix(t *testing.T) {
	// DI-2: Known MAC prefix returns vendor
	vendor := LookupOUI("E8:0A:B9:33:44:55")
	if vendor == "" {
		t.Fatal("LookupOUI() returned empty string for known prefix")
	}
	if vendor != "Cisco Systems" {
		t.Errorf("LookupOUI() = %q, want %q", vendor, "Cisco Systems")
	}
}

func TestLookupOUI_AnotherKnownPrefix(t *testing.T) {
	// DI-2: Another known MAC prefix
	vendor := LookupOUI("F0:EE:7A:00:00:01")
	if vendor != "Apple" {
		t.Errorf("LookupOUI() = %q, want %q", vendor, "Apple")
	}
}

func TestLookupOUI_UnknownPrefix(t *testing.T) {
	// DI-2: Unknown MAC prefix returns "Unknown"
	vendor := LookupOUI("FF:FF:FF:00:00:00")
	if vendor != "Unknown" {
		t.Errorf("LookupOUI() = %q, want %q", vendor, "Unknown")
	}
}

func TestLookupOUI_EmptyMAC(t *testing.T) {
	// DI-2: Empty MAC returns "Unknown"
	vendor := LookupOUI("")
	if vendor != "Unknown" {
		t.Errorf("LookupOUI() = %q, want %q", vendor, "Unknown")
	}
}

func TestLookupOUI_Lowercase(t *testing.T) {
	// DI-2: Lowercase MAC should still match
	vendor := LookupOUI("e8:0a:b9:aa:bb:cc")
	if vendor == "" {
		t.Fatal("LookupOUI() returned empty for lowercase MAC")
	}
	if vendor != "Cisco Systems" {
		t.Errorf("LookupOUI() = %q, want %q", vendor, "Cisco Systems")
	}
}

func TestLookupOUI_RaspberryPi(t *testing.T) {
	// DI-2: Known Raspberry Pi prefix
	vendor := LookupOUI("B8:27:EB:12:34:56")
	if vendor == "" {
		t.Fatal("LookupOUI() returned empty for Raspberry Pi prefix")
	}
	if vendor != "Raspberry Pi Foundation" {
		t.Errorf("LookupOUI() = %q, want %q", vendor, "Raspberry Pi Foundation")
	}
}
