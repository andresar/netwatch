package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/andresar/netwatch/internal/models"
)

// mockScanner implements Scanner for handler testing.
type mockScanner struct {
	scanFunc func(ctx context.Context, subnet string) (*models.ScanResult, error)
}

func (m *mockScanner) Scan(ctx context.Context, subnet string) (*models.ScanResult, error) {
	return m.scanFunc(ctx, subnet)
}

func TestGetDevices_TriggersScan(t *testing.T) {
	// RA-1: GET /api/devices triggers a scan and returns 200 with JSON
	expected := &models.ScanResult{
		Devices: []models.Device{
			{IP: "192.168.1.1", MAC: "00:11:22:aa:bb:cc", Hostname: "router", Vendor: "Cisco"},
		},
		ScannedAt: time.Now(),
		Total:     1,
	}

	mock := &mockScanner{
		scanFunc: func(ctx context.Context, subnet string) (*models.ScanResult, error) {
			return expected, nil
		},
	}

	h := NewHandler(mock, "192.168.1.0/24")
	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	w := httptest.NewRecorder()

	h.GetDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var result models.ScanResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json.Decode error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Total = %d, want 1", result.Total)
	}
	if len(result.Devices) != 1 {
		t.Fatalf("len(Devices) = %d, want 1", len(result.Devices))
	}
	if result.Devices[0].IP != "192.168.1.1" {
		t.Errorf("Device IP = %q, want 192.168.1.1", result.Devices[0].IP)
	}
}

func TestGetDevices_CachedReturnsPrior(t *testing.T) {
	// RA-2: GET /api/devices?cached=true returns prior cached results
	firstResult := &models.ScanResult{
		Devices: []models.Device{
			{IP: "192.168.1.1", MAC: "00:11:22:aa:bb:cc"},
		},
		ScannedAt: time.Now(),
		Total:     1,
	}

	scanCount := 0
	mock := &mockScanner{
		scanFunc: func(ctx context.Context, subnet string) (*models.ScanResult, error) {
			scanCount++
			return firstResult, nil
		},
	}

	h := NewHandler(mock, "192.168.1.0/24")

	// First request triggers scan
	req1 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	w1 := httptest.NewRecorder()
	h.GetDevices(w1, req1)
	if w1.Result().StatusCode != http.StatusOK {
		t.Fatal("First request did not return 200")
	}
	w1.Result().Body.Close()

	if scanCount != 1 {
		t.Fatalf("scanCount = %d, want 1", scanCount)
	}

	// Second request with ?cached=true returns cached, does NOT trigger scan
	req2 := httptest.NewRequest(http.MethodGet, "/api/devices?cached=true", nil)
	w2 := httptest.NewRecorder()
	h.GetDevices(w2, req2)

	resp2 := w2.Result()
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Cached request Status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}

	ct := resp2.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Cached Content-Type = %q, want application/json", ct)
	}

	var result models.ScanResult
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		t.Fatalf("json.Decode error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Cached Total = %d, want 1", result.Total)
	}

	// Scan should NOT have been called a second time
	if scanCount != 1 {
		t.Errorf("scanCount = %d, want 1 (scan should not be triggered by cached request)", scanCount)
	}
}

func TestGetDevices_CachedNoPreviousScan(t *testing.T) {
	// RA-2: GET /api/devices?cached=true with no prior scan returns 404
	mock := &mockScanner{
		scanFunc: func(ctx context.Context, subnet string) (*models.ScanResult, error) {
			return &models.ScanResult{Devices: []models.Device{}}, nil
		},
	}

	h := NewHandler(mock, "192.168.1.0/24")
	req := httptest.NewRequest(http.MethodGet, "/api/devices?cached=true", nil)
	w := httptest.NewRecorder()

	h.GetDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Status = %d, want %d (no cached result)", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetDevices_ConcurrentScanConflict(t *testing.T) {
	// RA-1: Concurrent scan request returns 409 Conflict
	scanStarted := make(chan struct{})
	scanDone := make(chan struct{})

	mock := &mockScanner{
		scanFunc: func(ctx context.Context, subnet string) (*models.ScanResult, error) {
			close(scanStarted)
			// Block until test signals completion
			<-scanDone
			return &models.ScanResult{Devices: []models.Device{}, ScannedAt: time.Now(), Total: 0}, nil
		},
	}

	h := NewHandler(mock, "192.168.1.0/24")

	// Start first scan in background
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
		w := httptest.NewRecorder()
		h.GetDevices(w, req)
		w.Result().Body.Close()
	}()

	// Wait for first scan to start
	<-scanStarted

	// Second concurrent request should get 409
	req2 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	w2 := httptest.NewRecorder()
	h.GetDevices(w2, req2)

	resp2 := w2.Result()
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("Concurrent request Status = %d, want %d", resp2.StatusCode, http.StatusConflict)
	}

	// Allow first scan to complete
	close(scanDone)
	wg.Wait()
}

func TestGetDevices_ContentTypeJSON(t *testing.T) {
	// RA-3: Response always has Content-Type: application/json
	mock := &mockScanner{
		scanFunc: func(ctx context.Context, subnet string) (*models.ScanResult, error) {
			return &models.ScanResult{Devices: []models.Device{}, Total: 0}, nil
		},
	}

	h := NewHandler(mock, "192.168.1.0/24")
	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	w := httptest.NewRecorder()

	h.GetDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	// Verify body is valid JSON
	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("Response body is not valid JSON: %v", err)
	}
}

func TestGetDevices_ScannerError(t *testing.T) {
	// Scanner error returns 500
	mock := &mockScanner{
		scanFunc: func(ctx context.Context, subnet string) (*models.ScanResult, error) {
			return nil, context.DeadlineExceeded
		},
	}

	h := NewHandler(mock, "192.168.1.0/24")
	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	w := httptest.NewRecorder()

	h.GetDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Errorf("Response body is not valid JSON: %v", err)
	}
	if body["error"] == nil {
		t.Error("Response should contain 'error' field")
	}
}
