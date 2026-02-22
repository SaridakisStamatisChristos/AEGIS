package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/aegisrun/aegisrun/internal/auth"
	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/oklog/ulid/v2"
	"go.uber.org/zap"
)

// requiredApprovals is the minimum number of approvals needed to mark a policy as approved.
const requiredApprovals = 2

// ApprovalsHandler handles policy approval endpoints.
type ApprovalsHandler struct {
	approvalStore *store.ApprovalStore
	policyStore   *store.PolicyStore
	logger        *zap.Logger
}

// NewApprovalsHandler creates a new ApprovalsHandler.
func NewApprovalsHandler(
	approvalStore *store.ApprovalStore,
	policyStore *store.PolicyStore,
	logger *zap.Logger,
) *ApprovalsHandler {
	return &ApprovalsHandler{
		approvalStore: approvalStore,
		policyStore:   policyStore,
		logger:        logger,
	}
}

// --- Request / Response types ---

// ApprovalResponse is the JSON representation of a single approval.
type ApprovalResponse struct {
	ApprovalID string    `json:"approval_id"`
	PolicyID   string    `json:"policy_id"`
	Version    string    `json:"version"`
	ApproverID string    `json:"approver_id"`
	Decision   string    `json:"decision"`
	Comment    *string   `json:"comment,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ApproveRejectRequest is the body for Approve / Reject endpoints.
type ApproveRejectRequest struct {
	Comment string `json:"comment,omitempty"`
}

// --- Helpers ---

func approvalToResponse(a *contracts.Approval) ApprovalResponse {
	return ApprovalResponse{
		ApprovalID: a.ApprovalID,
		PolicyID:   a.PolicyID,
		Version:    a.Version,
		ApproverID: a.ApproverID,
		Decision:   a.Decision,
		Comment:    a.Comment,
		CreatedAt:  a.CreatedAt,
	}
}

// --- Handlers ---

// List returns all approvals for a given policy version.
// Query params: policy_id (required), version (required).
func (h *ApprovalsHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID := r.URL.Query().Get("policy_id")
	version := r.URL.Query().Get("version")
	if policyID == "" || version == "" {
		http.Error(w, "policy_id and version query parameters are required", http.StatusBadRequest)
		return
	}

	approvals, err := h.approvalStore.ListByPolicy(r.Context(), policyID, version)
	if err != nil {
		h.logger.Error("failed to list approvals", zap.String("policy_id", policyID), zap.String("version", version), zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := make([]ApprovalResponse, len(approvals))
	for i, a := range approvals {
		resp[i] = approvalToResponse(a)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(struct {
		Approvals []ApprovalResponse `json:"approvals"`
		Total     int                `json:"total"`
	}{
		Approvals: resp,
		Total:     len(resp),
	}); err != nil {
		h.logger.Error("failed to encode approvals response", zap.Error(err))
	}
}

// Get returns a single approval by its ID.
// URL param: approvalID.
func (h *ApprovalsHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	approvalID := chi.URLParam(r, "approvalID")
	if approvalID == "" {
		http.Error(w, "approvalID is required", http.StatusBadRequest)
		return
	}

	approval, err := h.approvalStore.Get(r.Context(), approvalID)
	if err != nil {
		if isNotFound(err) {
			http.Error(w, "approval not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get approval", zap.String("approval_id", approvalID), zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(approvalToResponse(approval)); err != nil {
		h.logger.Error("failed to encode approval response", zap.Error(err))
	}
}

// Approve records an "approved" decision for a policy version.
// URL param: policyID. Query param: version (required).
func (h *ApprovalsHandler) Approve(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID := chi.URLParam(r, "policyID")
	version := r.URL.Query().Get("version")
	if policyID == "" || version == "" {
		http.Error(w, "policyID path param and version query param are required", http.StatusBadRequest)
		return
	}

	// Verify the policy exists.
	policy, err := h.policyStore.GetByID(r.Context(), policyID)
	if err != nil {
		if isNotFound(err) {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get policy", zap.String("policy_id", policyID), zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Only policies in review status may be approved.
	if policy.Status != contracts.PolicyStatusReview {
		http.Error(w, "policy is not in review status", http.StatusConflict)
		return
	}

	// Check if user already voted on this version.
	already, err := h.approvalStore.HasUserApproved(r.Context(), policyID, version, user.UserID)
	if err != nil {
		h.logger.Error("failed to check prior approval", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if already {
		http.Error(w, "user has already submitted a decision for this policy version", http.StatusConflict)
		return
	}

	// Parse optional comment.
	var req ApproveRejectRequest
	if r.Body != nil && r.Body != http.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
	}

	now := time.Now().UTC()
	var comment *string
	if req.Comment != "" {
		comment = &req.Comment
	}

	approval := &contracts.Approval{
		ApprovalID: ulid.Make().String(),
		PolicyID:   policyID,
		Version:    version,
		ApproverID: user.UserID,
		Decision:   "approved",
		Comment:    comment,
		CreatedAt:  now,
	}

	if err := h.approvalStore.Create(r.Context(), approval); err != nil {
		h.logger.Error("failed to create approval", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Check whether enough approvals have been collected to promote the policy.
	count, err := h.approvalStore.CountApprovals(r.Context(), policyID, version, "approved")
	if err != nil {
		h.logger.Error("failed to count approvals", zap.Error(err))
		// The approval was recorded; don't fail the request.
	} else if count >= requiredApprovals {
		// Collect approver list.
		approvals, listErr := h.approvalStore.ListByPolicy(r.Context(), policyID, version)
		if listErr == nil {
			approvers := make([]string, 0, len(approvals))
			for _, a := range approvals {
				if a.Decision == "approved" {
					approvers = append(approvers, a.ApproverID)
				}
			}
			if err := h.policyStore.SetApproved(r.Context(), policyID, version, approvers); err != nil {
				h.logger.Error("failed to set policy approved", zap.Error(err))
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(approvalToResponse(approval)); err != nil {
		h.logger.Error("failed to encode approval response", zap.Error(err))
	}
}

// Reject records a "rejected" decision for a policy version.
// URL param: policyID. Query param: version (required).
func (h *ApprovalsHandler) Reject(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID := chi.URLParam(r, "policyID")
	version := r.URL.Query().Get("version")
	if policyID == "" || version == "" {
		http.Error(w, "policyID path param and version query param are required", http.StatusBadRequest)
		return
	}

	// Verify the policy exists.
	_, err := h.policyStore.GetByID(r.Context(), policyID)
	if err != nil {
		if isNotFound(err) {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get policy", zap.String("policy_id", policyID), zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Check if user already voted on this version.
	already, err := h.approvalStore.HasUserApproved(r.Context(), policyID, version, user.UserID)
	if err != nil {
		h.logger.Error("failed to check prior approval", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if already {
		http.Error(w, "user has already submitted a decision for this policy version", http.StatusConflict)
		return
	}

	var req ApproveRejectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Comment == "" {
		http.Error(w, "comment is required for rejection", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	comment := &req.Comment

	approval := &contracts.Approval{
		ApprovalID: ulid.Make().String(),
		PolicyID:   policyID,
		Version:    version,
		ApproverID: user.UserID,
		Decision:   "rejected",
		Comment:    comment,
		CreatedAt:  now,
	}

	if err := h.approvalStore.Create(r.Context(), approval); err != nil {
		h.logger.Error("failed to create rejection", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Optionally move policy back to draft on rejection.
	if err := h.policyStore.UpdateStatus(r.Context(), policyID, version, contracts.PolicyStatusDraft); err != nil {
		h.logger.Error("failed to update policy status after rejection", zap.Error(err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(approvalToResponse(approval)); err != nil {
		h.logger.Error("failed to encode approval response", zap.Error(err))
	}
}
