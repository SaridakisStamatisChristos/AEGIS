package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/aegisrun/aegisrun/internal/auth"
	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/gateway"
	"go.uber.org/zap"
)

type GatewayHandler struct {
	gateway *gateway.Gateway
	logger  *zap.Logger
}

func NewGatewayHandler(
	gw *gateway.Gateway,
	logger *zap.Logger,
) *GatewayHandler {
	return &GatewayHandler{
		gateway: gw,
		logger:  logger,
	}
}

// Execute handles a tool-call request through the policy gateway.
func (h *GatewayHandler) Execute(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req gateway.ToolCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields (all string ULIDs)
	if req.RunID == "" {
		http.Error(w, "run_id is required", http.StatusBadRequest)
		return
	}
	if req.StepID == "" {
		http.Error(w, "step_id is required", http.StatusBadRequest)
		return
	}
	if req.ToolName == "" {
		http.Error(w, "tool_name is required", http.StatusBadRequest)
		return
	}

	// Execute through gateway
	resp, err := h.gateway.ExecuteToolCall(r.Context(), &req)
	if err != nil {
		h.logger.Error("gateway execution failed",
			zap.Error(err),
			zap.String("tool", req.ToolName),
			zap.String("run_id", req.RunID),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"error": "gateway execution failed",
		}); err != nil {
			h.logger.Error("failed to encode gateway error response", zap.Error(err))
		}
		return
	}

	// Determine HTTP status from the policy decision action
	var statusCode int
	switch resp.Decision.Action {
	case contracts.ActionAllow, contracts.ActionWarn, contracts.ActionRedact:
		statusCode = http.StatusOK
	case contracts.ActionBlock:
		statusCode = http.StatusForbidden
	case contracts.ActionRequireApproval:
		statusCode = http.StatusAccepted
	default:
		statusCode = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode gateway response", zap.Error(err))
	}
}
