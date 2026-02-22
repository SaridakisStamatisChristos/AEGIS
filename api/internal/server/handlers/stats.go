package handlers

import (
	"net/http"

	"github.com/aegisrun/aegisrun/internal/auth"
	"github.com/aegisrun/aegisrun/internal/store"
	"go.uber.org/zap"
)

// StatsHandler serves aggregated dashboard statistics.
type StatsHandler struct {
	runStore *store.RunStore
	logger   *zap.Logger
}

func NewStatsHandler(runStore *store.RunStore, logger *zap.Logger) *StatsHandler {
	return &StatsHandler{runStore: runStore, logger: logger}
}

// Get returns aggregate statistics for the authenticated user's org.
func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	stats, err := h.runStore.GetStats(r.Context(), user.OrgID)
	if err != nil {
		h.logger.Error("failed to get stats", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
