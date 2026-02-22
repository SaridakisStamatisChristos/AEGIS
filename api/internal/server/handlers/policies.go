package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aegisrun/aegisrun/internal/auth"
	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/policy"
	"github.com/aegisrun/aegisrun/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/oklog/ulid/v2"
	"go.uber.org/zap"
)

// PoliciesHandler handles CRUD operations for policies.
type PoliciesHandler struct {
	policyStore *store.PolicyStore
	validator   *policy.Validator
	logger      *zap.Logger
}

// NewPoliciesHandler creates a new PoliciesHandler.
func NewPoliciesHandler(
	policyStore *store.PolicyStore,
	logger *zap.Logger,
) *PoliciesHandler {
	return &PoliciesHandler{
		policyStore: policyStore,
		validator:   policy.NewValidator(),
		logger:      logger,
	}
}

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

// CreatePolicyRequest is the payload for creating a new policy.
type CreatePolicyRequest struct {
	Name string               `json:"name"`
	Spec contracts.PolicySpec `json:"spec"`
}

// UpdatePolicyRequest is the payload for creating a new version of a policy.
type UpdatePolicyRequest struct {
	Spec contracts.PolicySpec `json:"spec"`
}

// PolicyResponse is the JSON representation returned to callers.
type PolicyResponse struct {
	PolicyID   string               `json:"policy_id"`
	OrgID      string               `json:"org_id"`
	Name       string               `json:"name"`
	Version    string               `json:"version"`
	Status     string               `json:"status"`
	CreatedAt  time.Time            `json:"created_at"`
	ApprovedAt *time.Time           `json:"approved_at,omitempty"`
	ApprovedBy []string             `json:"approved_by,omitempty"`
	Spec       contracts.PolicySpec `json:"spec"`
	SpecHash   string               `json:"spec_hash"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func policyToResponse(p *contracts.Policy) PolicyResponse {
	return PolicyResponse{
		PolicyID:   p.PolicyID,
		OrgID:      p.OrgID,
		Name:       p.Name,
		Version:    p.Version,
		Status:     string(p.Status),
		CreatedAt:  p.CreatedAt,
		ApprovedAt: p.ApprovedAt,
		ApprovedBy: p.ApprovedBy,
		Spec:       p.Spec,
		SpecHash:   p.SpecHash,
	}
}

// isNotFound and writeJSON are in helpers.go

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// List returns all policies for the authenticated user's org.
// Query params:
//   - status  (optional) – filter by PolicyStatus value
func (h *PoliciesHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var statusFilter *contracts.PolicyStatus
	if s := r.URL.Query().Get("status"); s != "" {
		ps := contracts.PolicyStatus(s)
		statusFilter = &ps
	}

	policies, err := h.policyStore.List(r.Context(), user.OrgID, statusFilter)
	if err != nil {
		h.logger.Error("failed to list policies", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := make([]PolicyResponse, len(policies))
	for i, p := range policies {
		resp[i] = policyToResponse(p)
	}

	writeJSON(w, http.StatusOK, struct {
		Policies []PolicyResponse `json:"policies"`
	}{
		Policies: resp,
	})
}

// Create creates a new policy in draft status.
func (h *PoliciesHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	// Validate the spec via the policy validator.
	if err := h.validator.Validate(&req.Spec); err != nil {
		http.Error(w, fmt.Sprintf("invalid policy spec: %s", err), http.StatusBadRequest)
		return
	}

	specHash, err := h.validator.ComputeSpecHash(&req.Spec)
	if err != nil {
		h.logger.Error("failed to compute spec hash", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	p := &contracts.Policy{
		PolicyID:  ulid.Make().String(),
		OrgID:     user.OrgID,
		Name:      req.Name,
		Version:   "v1",
		Status:    contracts.PolicyStatusDraft,
		CreatedAt: time.Now().UTC(),
		Spec:      req.Spec,
		SpecHash:  specHash,
	}

	if err := h.policyStore.Create(r.Context(), p, user.UserID); err != nil {
		h.logger.Error("failed to create policy", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, policyToResponse(p))
}

// Get returns a single policy by ID (latest version).
func (h *PoliciesHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID := chi.URLParam(r, "policyID")
	if policyID == "" {
		http.Error(w, "missing policy ID", http.StatusBadRequest)
		return
	}

	// If a version query param is provided, fetch that specific version.
	version := r.URL.Query().Get("version")

	var (
		p   *contracts.Policy
		err error
	)
	if version != "" {
		p, err = h.policyStore.Get(r.Context(), policyID, version)
	} else {
		p, err = h.policyStore.GetByID(r.Context(), policyID)
	}

	if err != nil {
		if isNotFound(err) {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get policy", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Ensure the policy belongs to the caller's org.
	if p.OrgID != user.OrgID {
		http.Error(w, "policy not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, policyToResponse(p))
}

// Update creates a new version of an existing policy with updated spec.
func (h *PoliciesHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID := chi.URLParam(r, "policyID")
	if policyID == "" {
		http.Error(w, "missing policy ID", http.StatusBadRequest)
		return
	}

	var req UpdatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the new spec.
	if err := h.validator.Validate(&req.Spec); err != nil {
		http.Error(w, fmt.Sprintf("invalid policy spec: %s", err), http.StatusBadRequest)
		return
	}

	// Fetch the latest version to derive the next version string.
	existing, err := h.policyStore.GetByID(r.Context(), policyID)
	if err != nil {
		if isNotFound(err) {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get policy for update", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if existing.OrgID != user.OrgID {
		http.Error(w, "policy not found", http.StatusNotFound)
		return
	}

	specHash, err := h.validator.ComputeSpecHash(&req.Spec)
	if err != nil {
		h.logger.Error("failed to compute spec hash", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Derive next version (v1 -> v2, v2 -> v3, …).
	nextVersion := nextVersionString(existing.Version)

	newPolicy := &contracts.Policy{
		PolicyID:  existing.PolicyID,
		OrgID:     existing.OrgID,
		Name:      existing.Name,
		Version:   nextVersion,
		Status:    contracts.PolicyStatusDraft,
		CreatedAt: time.Now().UTC(),
		Spec:      req.Spec,
		SpecHash:  specHash,
	}

	if err := h.policyStore.Create(r.Context(), newPolicy, user.UserID); err != nil {
		h.logger.Error("failed to create new policy version", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, policyToResponse(newPolicy))
}

// Delete deprecates a policy (soft-delete via status change).
func (h *PoliciesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID := chi.URLParam(r, "policyID")
	if policyID == "" {
		http.Error(w, "missing policy ID", http.StatusBadRequest)
		return
	}

	existing, err := h.policyStore.GetByID(r.Context(), policyID)
	if err != nil {
		if isNotFound(err) {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get policy for delete", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if existing.OrgID != user.OrgID {
		http.Error(w, "policy not found", http.StatusNotFound)
		return
	}

	if err := h.policyStore.UpdateStatus(r.Context(), policyID, existing.Version, contracts.PolicyStatusDeprecated); err != nil {
		h.logger.Error("failed to deprecate policy", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Activate deploys a policy (sets status to deployed).
func (h *PoliciesHandler) Activate(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID := chi.URLParam(r, "policyID")
	if policyID == "" {
		http.Error(w, "missing policy ID", http.StatusBadRequest)
		return
	}

	p, err := h.policyStore.GetByID(r.Context(), policyID)
	if err != nil {
		if isNotFound(err) {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get policy for activate", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if p.OrgID != user.OrgID {
		http.Error(w, "policy not found", http.StatusNotFound)
		return
	}

	if err := h.policyStore.SetDeployed(r.Context(), p.PolicyID, p.Version); err != nil {
		h.logger.Error("failed to deploy policy", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Re-fetch to get updated fields.
	p, err = h.policyStore.Get(r.Context(), p.PolicyID, p.Version)
	if err != nil {
		h.logger.Error("failed to re-fetch policy after deploy", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, policyToResponse(p))
}

// Deactivate sets a policy back to draft status.
func (h *PoliciesHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	policyID := chi.URLParam(r, "policyID")
	if policyID == "" {
		http.Error(w, "missing policy ID", http.StatusBadRequest)
		return
	}

	p, err := h.policyStore.GetByID(r.Context(), policyID)
	if err != nil {
		if isNotFound(err) {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get policy for deactivate", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if p.OrgID != user.OrgID {
		http.Error(w, "policy not found", http.StatusNotFound)
		return
	}

	if err := h.policyStore.UpdateStatus(r.Context(), p.PolicyID, p.Version, contracts.PolicyStatusDraft); err != nil {
		h.logger.Error("failed to deactivate policy", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Re-fetch to get updated fields.
	p, err = h.policyStore.Get(r.Context(), p.PolicyID, p.Version)
	if err != nil {
		h.logger.Error("failed to re-fetch policy after deactivate", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, policyToResponse(p))
}

// ---------------------------------------------------------------------------
// Version helpers
// ---------------------------------------------------------------------------

// nextVersionString increments a "vN" version string. Falls back to "v1" on
// parse failure.
func nextVersionString(current string) string {
	if len(current) < 2 || current[0] != 'v' {
		return "v1"
	}
	n := 0
	for _, ch := range current[1:] {
		if ch < '0' || ch > '9' {
			return "v1"
		}
		n = n*10 + int(ch-'0')
	}
	return fmt.Sprintf("v%d", n+1)
}
