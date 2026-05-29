package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/andresar/netwatch/internal/models"
)

// Scanner defines the scanning interface used by HTTP handlers.
type Scanner interface {
	Scan(ctx context.Context, subnet string) (*models.ScanResult, error)
}

// Handler holds HTTP handlers for the scanner API.
type Handler struct {
	scanner    Scanner
	subnet     string
	mu         sync.Mutex
	lastResult *models.ScanResult
	scanning   bool
}

// NewHandler creates a new Handler with the given scanner and subnet.
func NewHandler(scanner Scanner, subnet string) *Handler {
	return &Handler{
		scanner: scanner,
		subnet:  subnet,
	}
}

// GetDevices handles GET /api/devices.
// Without ?cached=true, triggers a full scan.
// With ?cached=true, returns the last scan result (or 404 if none).
// Returns 409 Conflict if a scan is already in progress.
func (h *Handler) GetDevices(w http.ResponseWriter, r *http.Request) {
	cached := r.URL.Query().Get("cached") == "true"

	h.mu.Lock()
	if cached {
		if h.lastResult == nil {
			h.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "no cached result"})
			return
		}
		result := h.lastResult
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}

	if h.scanning {
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "scan already in progress"})
		return
	}

	h.scanning = true
	h.mu.Unlock()

	result, err := h.scanner.Scan(r.Context(), h.subnet)

	h.mu.Lock()
	h.scanning = false
	if err != nil {
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	h.lastResult = result
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
