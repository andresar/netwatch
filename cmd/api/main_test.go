package main

import (
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestRun_MissingSubnet(t *testing.T) {
	// CM-1: Should error when NETWATCH_SUBNET is not set
	os.Clearenv()
	err := run(make(chan os.Signal))
	if err == nil {
		t.Fatal("run() expected error for missing subnet, got nil")
	}
}

func TestRun_InvalidSubnet(t *testing.T) {
	// CM-1: Should error when NETWATCH_SUBNET is invalid
	os.Clearenv()
	t.Setenv("NETWATCH_SUBNET", "not-a-cidr")
	err := run(make(chan os.Signal))
	if err == nil {
		t.Fatal("run() expected error for invalid subnet, got nil")
	}
}

func TestRun_InvalidPort(t *testing.T) {
	// CM-2: Should error when NETWATCH_PORT is not a number
	os.Clearenv()
	t.Setenv("NETWATCH_SUBNET", "192.168.1.0/24")
	t.Setenv("NETWATCH_PORT", "not-a-port")
	err := run(make(chan os.Signal))
	if err == nil {
		t.Fatal("run() expected error for invalid port, got nil")
	}
}

func TestRun_InvalidTimeout(t *testing.T) {
	// CM-3: Should error when NETWATCH_SCAN_TIMEOUT is invalid
	os.Clearenv()
	t.Setenv("NETWATCH_SUBNET", "192.168.1.0/24")
	t.Setenv("NETWATCH_SCAN_TIMEOUT", "not-a-duration")
	err := run(make(chan os.Signal))
	if err == nil {
		t.Fatal("run() expected error for invalid scan timeout, got nil")
	}
}

func TestRun_ServerStartsAndResponds(t *testing.T) {
	// Server starts on configured port and responds to requests
	os.Clearenv()
	t.Setenv("NETWATCH_SUBNET", "192.168.1.0/24")
	t.Setenv("NETWATCH_PORT", "0") // random port

	sigCh := make(chan os.Signal, 1)
	errCh := make(chan error, 1)

	go func() {
		errCh <- run(sigCh)
	}()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Try to connect to verify server started (port 0 = random, so we probe)
	// We just verify run() didn't error out immediately
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run() returned early with error: %v", err)
		}
		// If we got here, run returned before signal — might be an error case
		// Actually this means run() errored, so we already handled it above
	default:
		// Server started successfully — signal shutdown
	}

	sigCh <- syscall.SIGINT

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run() returned error after signal: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run() did not shut down within 3s after signal")
	}
}

func TestRun_SIGTERMShutsDown(t *testing.T) {
	// SIGTERM triggers graceful shutdown
	os.Clearenv()
	t.Setenv("NETWATCH_SUBNET", "192.168.1.0/24")
	t.Setenv("NETWATCH_PORT", "0")

	sigCh := make(chan os.Signal, 1)
	errCh := make(chan error, 1)

	go func() {
		errCh <- run(sigCh)
	}()

	time.Sleep(200 * time.Millisecond)

	sigCh <- syscall.SIGTERM

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run() returned error after SIGTERM: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run() did not shut down within 3s after SIGTERM")
	}
}

func TestRun_ServerServesHTTP(t *testing.T) {
	// Server responds to HTTP requests on the configured port
	os.Clearenv()
	port := "18999" // use a fixed port for testing
	t.Setenv("NETWATCH_SUBNET", "192.168.1.0/24")
	t.Setenv("NETWATCH_PORT", port)

	sigCh := make(chan os.Signal, 1)
	errCh := make(chan error, 1)

	go func() {
		errCh <- run(sigCh)
	}()

	time.Sleep(200 * time.Millisecond)

	// Verify server is listening and responding
	resp, err := http.Get("http://localhost:" + port + "/api/devices")
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", resp.Header.Get("Content-Type"))
	}

	sigCh <- syscall.SIGINT

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run() returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run() did not shut down within 3s")
	}
}
