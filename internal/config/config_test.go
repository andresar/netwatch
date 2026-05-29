package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// CM-2: Default port 8080
	// CM-3: Default PING_CONCURRENCY=64, SCAN_TIMEOUT=30s, DNS_TIMEOUT=1s
	os.Clearenv()
	t.Setenv("NETWATCH_SUBNET", "192.168.1.0/24")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Subnet != "192.168.1.0/24" {
		t.Errorf("cfg.Subnet = %q, want %q", cfg.Subnet, "192.168.1.0/24")
	}
	if cfg.Port != 8080 {
		t.Errorf("cfg.Port = %d, want %d", cfg.Port, 8080)
	}
	if cfg.PingConcurrency != 64 {
		t.Errorf("cfg.PingConcurrency = %d, want %d", cfg.PingConcurrency, 64)
	}
	if cfg.ScanTimeout != 30*time.Second {
		t.Errorf("cfg.ScanTimeout = %v, want %v", cfg.ScanTimeout, 30*time.Second)
	}
	if cfg.DNSTimeout != 1*time.Second {
		t.Errorf("cfg.DNSTimeout = %v, want %v", cfg.DNSTimeout, 1*time.Second)
	}
}

func TestLoad_CustomEnv(t *testing.T) {
	// CM-2, CM-3: Custom env values override defaults
	os.Clearenv()
	t.Setenv("NETWATCH_SUBNET", "10.0.0.0/16")
	t.Setenv("NETWATCH_PORT", "9090")
	t.Setenv("NETWATCH_PING_CONCURRENCY", "16")
	t.Setenv("NETWATCH_SCAN_TIMEOUT", "60s")
	t.Setenv("NETWATCH_DNS_TIMEOUT", "5s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Subnet != "10.0.0.0/16" {
		t.Errorf("cfg.Subnet = %q, want %q", cfg.Subnet, "10.0.0.0/16")
	}
	if cfg.Port != 9090 {
		t.Errorf("cfg.Port = %d, want %d", cfg.Port, 9090)
	}
	if cfg.PingConcurrency != 16 {
		t.Errorf("cfg.PingConcurrency = %d, want %d", cfg.PingConcurrency, 16)
	}
	if cfg.ScanTimeout != 60*time.Second {
		t.Errorf("cfg.ScanTimeout = %v, want %v", cfg.ScanTimeout, 60*time.Second)
	}
	if cfg.DNSTimeout != 5*time.Second {
		t.Errorf("cfg.DNSTimeout = %v, want %v", cfg.DNSTimeout, 5*time.Second)
	}
}

func TestLoad_SubnetRequired(t *testing.T) {
	// CM-1: SUBNET required
	os.Clearenv()

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing SUBNET, got nil")
	}
}

func TestLoad_InvalidSubnet(t *testing.T) {
	// CM-1: Invalid SUBNET returns error
	os.Clearenv()
	t.Setenv("NETWATCH_SUBNET", "not-a-cidr")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for invalid SUBNET, got nil")
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	// CM-2: Invalid PORT returns error
	os.Clearenv()
	t.Setenv("NETWATCH_SUBNET", "192.168.1.0/24")
	t.Setenv("NETWATCH_PORT", "nope")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for invalid PORT, got nil")
	}
}

func TestLoad_InvalidTimeout(t *testing.T) {
	// CM-3: Invalid SCAN_TIMEOUT returns error
	os.Clearenv()
	t.Setenv("NETWATCH_SUBNET", "192.168.1.0/24")
	t.Setenv("NETWATCH_SCAN_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for invalid SCAN_TIMEOUT, got nil")
	}
}

func TestLoad_InvalidPingConcurrency(t *testing.T) {
	// CM-3: Invalid PING_CONCURRENCY returns error
	os.Clearenv()
	t.Setenv("NETWATCH_SUBNET", "192.168.1.0/24")
	t.Setenv("NETWATCH_PING_CONCURRENCY", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for invalid PING_CONCURRENCY, got nil")
	}
}
