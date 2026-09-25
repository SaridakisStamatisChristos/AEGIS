package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/aegisrun/aegisrun/internal/store"
)

type HealthHandler struct {
	store   *store.Store
	version string
}

func NewHealthHandler(store *store.Store, version string) *HealthHandler {
	if version == "" {
		version = "dev"
	}
	return &HealthHandler{store: store, version: version}
}

type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:  "ok",
		Version: h.version,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

type ReadyResponse struct {
	Status   string            `json:"status"`
	Checks   map[string]string `json:"checks"`
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string)
	allHealthy := true

	// Check database
	if err := h.store.Ping(r.Context()); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		allHealthy = false
	} else {
		checks["database"] = "healthy"
	}

	resp := ReadyResponse{
		Checks: checks,
	}

	if allHealthy {
		resp.Status = "ready"
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	} else {
		resp.Status = "not ready"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	}
}
