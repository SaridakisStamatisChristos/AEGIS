package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aegisrun/aegisrun/internal/auth"
	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/ledger"
	"github.com/aegisrun/aegisrun/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
	"go.uber.org/zap"
)

// RunsHandler handles HTTP requests for agent runs.
type RunsHandler struct {
	runStore      *store.RunStore
	stepStore     *store.StepStore
	toolCallStore *store.ToolCallStore
	eventStore    *store.EventStore
	logger        *zap.Logger
}

// NewRunsHandler creates a new RunsHandler.
func NewRunsHandler(
	runStore *store.RunStore,
	stepStore *store.StepStore,
	toolCallStore *store.ToolCallStore,
	eventStore *store.EventStore,
	logger *zap.Logger,
) *RunsHandler {
	return &RunsHandler{
		runStore:      runStore,
		stepStore:     stepStore,
		toolCallStore: toolCallStore,
		eventStore:    eventStore,
		logger:        logger,
	}
}

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

// CreateRunRequest is the JSON body for POST /runs.
type CreateRunRequest struct {
	PolicyRef      contracts.PolicyRef    `json:"policy_ref"`
	ParentRunID    *string                `json:"parent_run_id,omitempty"`
	StateSchemaRef *contracts.SchemaRef   `json:"state_schema_ref,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// RunResponse is the JSON representation returned to clients.
type RunResponse struct {
	RunID          string                 `json:"run_id"`
	OrgID          string                 `json:"org_id"`
	ParentRunID    *string                `json:"parent_run_id,omitempty"`
	PolicyRef      contracts.PolicyRef    `json:"policy_ref"`
	StateSchemaRef *contracts.SchemaRef   `json:"state_schema_ref,omitempty"`
	Metadata       map[string]interface{} `json:"metadata"`
	CreatedAt      time.Time              `json:"created_at"`
	EndedAt        *time.Time             `json:"ended_at,omitempty"`
	Status         contracts.RunStatus    `json:"status"`
	Outcome        *contracts.RunOutcome  `json:"outcome,omitempty"`
	Counters       contracts.RunCounters  `json:"counters"`
	EvidenceHash   *string                `json:"evidence_hash,omitempty"`
	Signature      *string                `json:"signature,omitempty"`
	SignerKeyID    *string                `json:"signer_key_id,omitempty"`
}

// StepResponse is the JSON representation of a step.
type StepResponse struct {
	StepID       string                 `json:"step_id"`
	RunID        string                 `json:"run_id"`
	ParentStepID *string                `json:"parent_step_id,omitempty"`
	SeqNo        int                    `json:"seq_no"`
	Name         string                 `json:"name"`
	StateVector  map[string]interface{} `json:"state_vector"`
	StartedAt    time.Time              `json:"started_at"`
	EndedAt      *time.Time             `json:"ended_at,omitempty"`
	Status       contracts.StepStatus   `json:"status"`
	Error        *string                `json:"error,omitempty"`
}

// EventResponse is the JSON representation of an event.
type EventResponse struct {
	EventID   string                 `json:"event_id"`
	RunID     string                 `json:"run_id"`
	SeqNo     int                    `json:"seq_no"`
	EventType contracts.EventType    `json:"event_type"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
	PrevHash  *string                `json:"prev_hash,omitempty"`
	EventHash string                 `json:"event_hash"`
}

// ---------------------------------------------------------------------------
// Handler methods
// ---------------------------------------------------------------------------

// List handles GET /runs — returns runs for the authenticated org.
func (h *RunsHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse pagination
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	// Build filter
	filter := store.RunFilter{
		OrgID:  user.OrgID,
		Limit:  limit,
		Offset: offset,
	}

	// Optional status filter (comma-separated, e.g. "running,blocked")
	if statusParam := r.URL.Query().Get("status"); statusParam != "" {
		for _, s := range strings.Split(statusParam, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				filter.Status = append(filter.Status, contracts.RunStatus(s))
			}
		}
	}

	// Optional policy_id filter
	if pid := r.URL.Query().Get("policy_id"); pid != "" {
		filter.PolicyID = &pid
	}

	// Optional time range filters
	if st := r.URL.Query().Get("start_time"); st != "" {
		if t, err := time.Parse(time.RFC3339, st); err == nil {
			filter.StartTime = &t
		}
	}
	if et := r.URL.Query().Get("end_time"); et != "" {
		if t, err := time.Parse(time.RFC3339, et); err == nil {
			filter.EndTime = &t
		}
	}

	runs, err := h.runStore.List(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list runs", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := struct {
		Runs []RunResponse `json:"runs"`
	}{
		Runs: make([]RunResponse, 0, len(runs)),
	}
	for _, run := range runs {
		resp.Runs = append(resp.Runs, runToResponse(run))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Create handles POST /runs — creates a new run.
func (h *RunsHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.PolicyRef.PolicyID == "" {
		http.Error(w, "policy_ref.policy_id is required", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	run := &contracts.Run{
		RunID:          ulid.Make().String(),
		OrgID:          user.OrgID,
		ParentRunID:    req.ParentRunID,
		PolicyRef:      req.PolicyRef,
		StateSchemaRef: req.StateSchemaRef,
		Metadata:       req.Metadata,
		CreatedAt:      now,
		Status:         contracts.RunStatusRunning,
		Counters:       contracts.RunCounters{},
	}

	if run.Metadata == nil {
		run.Metadata = make(map[string]interface{})
	}

	if err := h.runStore.Create(r.Context(), run); err != nil {
		h.logger.Error("failed to create run", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(runToResponse(run))
}

// Get handles GET /runs/{runID} — returns a single run.
func (h *RunsHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	runID := chi.URLParam(r, "runID")
	if runID == "" {
		http.Error(w, "missing run ID", http.StatusBadRequest)
		return
	}

	run, err := h.runStore.Get(r.Context(), runID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "run not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get run", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Verify the run belongs to the caller's org
	if run.OrgID != user.OrgID {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runToResponse(run))
}

// ListSteps handles GET /runs/{runID}/steps
func (h *RunsHandler) ListSteps(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	runID := chi.URLParam(r, "runID")
	if runID == "" {
		http.Error(w, "missing run ID", http.StatusBadRequest)
		return
	}

	// Verify run exists and belongs to org
	run, err := h.runStore.Get(r.Context(), runID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "run not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get run", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if run.OrgID != user.OrgID {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	steps, err := h.stepStore.ListByRun(r.Context(), runID)
	if err != nil {
		h.logger.Error("failed to list steps", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := struct {
		Steps []StepResponse `json:"steps"`
	}{
		Steps: make([]StepResponse, 0, len(steps)),
	}
	for _, step := range steps {
		resp.Steps = append(resp.Steps, stepToResponse(step))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ListEvents handles GET /runs/{runID}/events
func (h *RunsHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	runID := chi.URLParam(r, "runID")
	if runID == "" {
		http.Error(w, "missing run ID", http.StatusBadRequest)
		return
	}

	// Verify run exists and belongs to org
	run, err := h.runStore.Get(r.Context(), runID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "run not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get run", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if run.OrgID != user.OrgID {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	events, err := h.eventStore.GetByRun(r.Context(), runID)
	if err != nil {
		h.logger.Error("failed to list events", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := struct {
		Events []EventResponse `json:"events"`
	}{
		Events: make([]EventResponse, 0, len(events)),
	}
	for _, event := range events {
		resp.Events = append(resp.Events, eventToResponse(event))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// SubmitEventRequest is the JSON body for POST /runs/{runID}/events.
type SubmitEventRequest struct {
	EventType string                 `json:"event_type"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Timestamp *string                `json:"timestamp,omitempty"`
}

// SubmitEvent handles POST /runs/{runID}/events – appends a hash-chained event.
func (h *RunsHandler) SubmitEvent(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	runID := chi.URLParam(r, "runID")
	if runID == "" {
		http.Error(w, "missing run ID", http.StatusBadRequest)
		return
	}

	var req SubmitEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.EventType == "" {
		http.Error(w, "event_type is required", http.StatusBadRequest)
		return
	}

	// Whitelist the event types that SDK callers may submit.
	switch contracts.EventType(req.EventType) {
	case contracts.EventRunStarted, contracts.EventRunEnded,
		contracts.EventStepStarted, contracts.EventStepEnded,
		contracts.EventStateUpdated:
		// allowed
	default:
		http.Error(w, "disallowed event_type", http.StatusBadRequest)
		return
	}

	// Verify run exists and belongs to org
	run, err := h.runStore.Get(r.Context(), runID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "run not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get run", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if run.OrgID != user.OrgID {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	ts := time.Now().UTC()
	if req.Timestamp != nil {
		if parsed, err := time.Parse(time.RFC3339Nano, *req.Timestamp); err == nil {
			ts = parsed.UTC()
		}
	}

	payload := req.Payload
	if payload == nil {
		payload = map[string]interface{}{}
	}

	hasher := ledger.NewHasher()

	var created *contracts.Event

	err = h.eventStore.Store().WithTx(r.Context(), func(tx *sqlx.Tx) error {
		seqNo, err := h.eventStore.GetNextSeqNo(r.Context(), tx, runID)
		if err != nil {
			return err
		}

		lastEvent, err := h.eventStore.GetLastEvent(r.Context(), tx, runID)
		if err != nil {
			return err
		}

		var prevHash *string
		if lastEvent != nil {
			prevHash = &lastEvent.EventHash
		}

		event := &contracts.Event{
			EventID:   ulid.Make().String(),
			RunID:     runID,
			SeqNo:     seqNo,
			EventType: contracts.EventType(req.EventType),
			Timestamp: ts,
			Payload:   payload,
			PrevHash:  prevHash,
		}

		hash, err := hasher.HashEvent(event)
		if err != nil {
			return fmt.Errorf("compute event hash: %w", err)
		}
		event.EventHash = hash

		if err := h.eventStore.Append(r.Context(), tx, event); err != nil {
			return err
		}

		created = event
		return nil
	})
	if err != nil {
		h.logger.Error("failed to submit event", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(eventToResponse(created))
}

// ---------------------------------------------------------------------------
// Converters
// ---------------------------------------------------------------------------

func runToResponse(run *contracts.Run) RunResponse {
	return RunResponse{
		RunID:          run.RunID,
		OrgID:          run.OrgID,
		ParentRunID:    run.ParentRunID,
		PolicyRef:      run.PolicyRef,
		StateSchemaRef: run.StateSchemaRef,
		Metadata:       run.Metadata,
		CreatedAt:      run.CreatedAt,
		EndedAt:        run.EndedAt,
		Status:         run.Status,
		Outcome:        run.Outcome,
		Counters:       run.Counters,
		EvidenceHash:   run.EvidenceHash,
		Signature:      run.Signature,
		SignerKeyID:    run.SignerKeyID,
	}
}

func stepToResponse(step *contracts.Step) StepResponse {
	return StepResponse{
		StepID:       step.StepID,
		RunID:        step.RunID,
		ParentStepID: step.ParentStepID,
		SeqNo:        step.SeqNo,
		Name:         step.Name,
		StateVector:  step.StateVector,
		StartedAt:    step.StartedAt,
		EndedAt:      step.EndedAt,
		Status:       step.Status,
		Error:        step.Error,
	}
}

func eventToResponse(event *contracts.Event) EventResponse {
	return EventResponse{
		EventID:   event.EventID,
		RunID:     event.RunID,
		SeqNo:     event.SeqNo,
		EventType: event.EventType,
		Timestamp: event.Timestamp,
		Payload:   event.Payload,
		PrevHash:  event.PrevHash,
		EventHash: event.EventHash,
	}
}
