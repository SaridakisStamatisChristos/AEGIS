package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aegisrun/aegisrun/internal/auth"
	"github.com/aegisrun/aegisrun/internal/ledger"
	"github.com/aegisrun/aegisrun/internal/store"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type EvidenceHandler struct {
	bundler    *ledger.Bundler
	eventStore *store.EventStore
	logger     *zap.Logger
}

func NewEvidenceHandler(
	bundler *ledger.Bundler,
	eventStore *store.EventStore,
	logger *zap.Logger,
) *EvidenceHandler {
	return &EvidenceHandler{
		bundler:    bundler,
		eventStore: eventStore,
		logger:     logger,
	}
}

// ExportBundle streams a ZIP evidence bundle for a run.
func (h *EvidenceHandler) ExportBundle(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	runID := chi.URLParam(r, "runID")
	if runID == "" {
		http.Error(w, "run_id is required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="evidence-%s.zip"`, runID))

	if err := h.bundler.CreateBundle(r.Context(), runID, w); err != nil {
		// If headers haven't been flushed yet we can still send an error.
		// In practice the ZIP writer may have already written bytes, so
		// the best we can do is log it.
		h.logger.Error("failed to create evidence bundle",
			zap.String("run_id", runID),
			zap.Error(err),
		)
		// Attempt to surface the error to the client if nothing was written yet.
		http.Error(w, "failed to create evidence bundle", http.StatusInternalServerError)
		return
	}
}

// VerifyBundleRequest is the JSON body for the verify endpoint.
type VerifyBundleRequest struct {
	RunID string `json:"run_id"`
}

// VerifyBundleResponse is returned from the verify endpoint.
type VerifyBundleResponse struct {
	RunID      string `json:"run_id"`
	ChainValid bool   `json:"chain_valid"`
	Message    string `json:"message,omitempty"`
}

// VerifyBundle checks the hash-chain integrity of the events for a given run.
func (h *EvidenceHandler) VerifyBundle(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req VerifyBundleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.RunID == "" {
		http.Error(w, "run_id is required", http.StatusBadRequest)
		return
	}

	valid, err := h.eventStore.VerifyChainIntegrity(r.Context(), req.RunID)
	if err != nil {
		h.logger.Error("chain integrity check failed",
			zap.String("run_id", req.RunID),
			zap.Error(err),
		)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := VerifyBundleResponse{
		RunID:      req.RunID,
		ChainValid: valid,
	}
	if valid {
		resp.Message = "all event hashes form a valid chain"
	} else {
		resp.Message = "chain integrity verification failed"
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode verify bundle response", zap.Error(err))
	}
}
