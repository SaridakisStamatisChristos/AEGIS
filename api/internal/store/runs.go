package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/jmoiron/sqlx"
)

type RunStore struct {
	store *Store
}

func NewRunStore(store *Store) *RunStore {
	return &RunStore{store: store}
}

// validMetadataKey ensures metadata keys contain only safe characters to prevent SQL injection.
// Compiled once at package level to avoid per-call overhead.
var validMetadataKey = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,63}$`)

type RunRow struct {
	RunID          string         `db:"run_id"`
	OrgID          string         `db:"org_id"`
	ParentRunID    sql.NullString `db:"parent_run_id"`
	PolicyID       string         `db:"policy_id"`
	PolicyVersion  string         `db:"policy_version"`
	StateSchemaID  sql.NullString `db:"state_schema_id"`
	StateSchemaVer sql.NullString `db:"state_schema_version"`
	Metadata       []byte         `db:"metadata"`
	CreatedAt      time.Time      `db:"created_at"`
	EndedAt        sql.NullTime   `db:"ended_at"`
	Status         string         `db:"status"`
	Outcome        []byte         `db:"outcome"`
	Counters       []byte         `db:"counters"`
	EvidenceHash   sql.NullString `db:"evidence_hash"`
	Signature      sql.NullString `db:"signature"`
	SignerKeyID    sql.NullString `db:"signer_key_id"`
}

func (r *RunRow) ToRun() (*contracts.Run, error) {
	run := &contracts.Run{
		RunID: r.RunID,
		OrgID: r.OrgID,
		PolicyRef: contracts.PolicyRef{
			PolicyID: r.PolicyID,
			Version:  r.PolicyVersion,
		},
		CreatedAt: r.CreatedAt,
		Status:    contracts.RunStatus(r.Status),
	}

	if r.ParentRunID.Valid {
		run.ParentRunID = &r.ParentRunID.String
	}

	if r.StateSchemaID.Valid {
		run.StateSchemaRef = &contracts.SchemaRef{
			SchemaID: r.StateSchemaID.String,
			Version:  r.StateSchemaVer.String,
		}
	}

	if err := json.Unmarshal(r.Metadata, &run.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	if r.EndedAt.Valid {
		run.EndedAt = &r.EndedAt.Time
	}

	if len(r.Outcome) > 0 {
		var outcome contracts.RunOutcome
		if err := json.Unmarshal(r.Outcome, &outcome); err != nil {
			return nil, fmt.Errorf("unmarshal outcome: %w", err)
		}
		run.Outcome = &outcome
	}

	if err := json.Unmarshal(r.Counters, &run.Counters); err != nil {
		return nil, fmt.Errorf("unmarshal counters: %w", err)
	}

	if r.EvidenceHash.Valid {
		run.EvidenceHash = &r.EvidenceHash.String
	}

	if r.Signature.Valid {
		run.Signature = &r.Signature.String
	}

	if r.SignerKeyID.Valid {
		run.SignerKeyID = &r.SignerKeyID.String
	}

	return run, nil
}

func (s *RunStore) Create(ctx context.Context, run *contracts.Run) error {
	metadata, err := json.Marshal(run.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	counters, err := json.Marshal(run.Counters)
	if err != nil {
		return fmt.Errorf("marshal counters: %w", err)
	}

	query := `
		INSERT INTO runs (
			run_id, org_id, parent_run_id, policy_id, policy_version,
			state_schema_id, state_schema_version, metadata, created_at, status, counters
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)`

	var stateSchemaID, stateSchemaVer interface{}
	if run.StateSchemaRef != nil {
		stateSchemaID = run.StateSchemaRef.SchemaID
		stateSchemaVer = run.StateSchemaRef.Version
	}

	_, err = s.store.db.ExecContext(ctx, query,
		run.RunID,
		run.OrgID,
		run.ParentRunID,
		run.PolicyRef.PolicyID,
		run.PolicyRef.Version,
		stateSchemaID,
		stateSchemaVer,
		metadata,
		run.CreatedAt,
		run.Status,
		counters,
	)

	if err != nil {
		return fmt.Errorf("insert run: %w", err)
	}

	return nil
}

func (s *RunStore) Get(ctx context.Context, runID string) (*contracts.Run, error) {
	query := `
		SELECT 
			run_id, org_id, parent_run_id, policy_id, policy_version,
			state_schema_id, state_schema_version, metadata, created_at, ended_at,
			status, outcome, counters, evidence_hash, signature, signer_key_id
		FROM runs
		WHERE run_id = $1`

	var row RunRow
	if err := s.store.db.GetContext(ctx, &row, query, runID); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewNotFoundError("run", runID)
		}
		return nil, fmt.Errorf("get run: %w", err)
	}

	return row.ToRun()
}

func (s *RunStore) UpdateStatus(ctx context.Context, tx *sqlx.Tx, runID string, status contracts.RunStatus, outcome *contracts.RunOutcome) error {
	var outcomeJSON []byte
	var err error

	if outcome != nil {
		outcomeJSON, err = json.Marshal(outcome)
		if err != nil {
			return fmt.Errorf("marshal outcome: %w", err)
		}
	}

	query := `
		UPDATE runs
		SET status = $1, ended_at = $2, outcome = $3
		WHERE run_id = $4`

	_, err = tx.ExecContext(ctx, query, status, time.Now(), outcomeJSON, runID)
	if err != nil {
		return fmt.Errorf("update run status: %w", err)
	}

	return nil
}

// allowedCounterFields is the whitelist of valid counter field names to prevent SQL injection.
var allowedCounterFields = map[string]bool{
	"tool_calls":     true,
	"steps":          true,
	"tokens_in":      true,
	"tokens_out":     true,
	"wall_clock_ms":  true,
	"retries":        true,
	"blocks":         true,
	"approvals":      true,
	"bytes_egressed": true,
}

// counterQueries holds pre-built SQL for each allowed counter field, eliminating
// any runtime string interpolation in SQL statements (addresses L-01).
var counterQueries = func() map[string]string {
	m := make(map[string]string, len(allowedCounterFields))
	for field := range allowedCounterFields {
		m[field] = `
		UPDATE runs
		SET counters = jsonb_set(
			counters,
			'{` + field + `}',
			(COALESCE((counters->>'` + field + `')::int, 0) + $1)::text::jsonb
		)
		WHERE run_id = $2`
	}
	return m
}()

func (s *RunStore) IncrementCounters(ctx context.Context, tx *sqlx.Tx, runID string, field string, delta int) error {
	query, ok := counterQueries[field]
	if !ok {
		return fmt.Errorf("invalid counter field: %q", field)
	}

	_, err := tx.ExecContext(ctx, query, delta, runID)
	if err != nil {
		return fmt.Errorf("increment counter %s: %w", field, err)
	}

	return nil
}

func (s *RunStore) SetEvidence(ctx context.Context, runID string, evidenceHash string, signature string, signerKeyID string) error {
	query := `
		UPDATE runs
		SET evidence_hash = $1, signature = $2, signer_key_id = $3
		WHERE run_id = $4`

	_, err := s.store.db.ExecContext(ctx, query, evidenceHash, signature, signerKeyID, runID)
	if err != nil {
		return fmt.Errorf("set evidence: %w", err)
	}

	return nil
}

type RunFilter struct {
	OrgID        string
	Status       []contracts.RunStatus
	PolicyID     *string
	StartTime    *time.Time
	EndTime      *time.Time
	MetadataTags map[string]string
	Limit        int
	Offset       int
}

func (s *RunStore) List(ctx context.Context, filter RunFilter) ([]*contracts.Run, error) {
	query := `
		SELECT 
			run_id, org_id, parent_run_id, policy_id, policy_version,
			state_schema_id, state_schema_version, metadata, created_at, ended_at,
			status, outcome, counters, evidence_hash, signature, signer_key_id
		FROM runs
		WHERE org_id = $1`

	args := []interface{}{filter.OrgID}
	argPos := 2

	if len(filter.Status) > 0 {
		query += fmt.Sprintf(" AND status = ANY($%d)", argPos)
		statusStrs := make([]string, len(filter.Status))
		for i, s := range filter.Status {
			statusStrs[i] = string(s)
		}
		args = append(args, statusStrs)
		argPos++
	}

	if filter.PolicyID != nil {
		query += fmt.Sprintf(" AND policy_id = $%d", argPos)
		args = append(args, *filter.PolicyID)
		argPos++
	}

	if filter.StartTime != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argPos)
		args = append(args, *filter.StartTime)
		argPos++
	}

	if filter.EndTime != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argPos)
		args = append(args, *filter.EndTime)
		argPos++
	}

	for key, value := range filter.MetadataTags {
		if !validMetadataKey.MatchString(key) {
			return nil, fmt.Errorf("invalid metadata key: %q", key)
		}
		query += fmt.Sprintf(" AND metadata->>'%s' = $%d", key, argPos)
		args = append(args, value)
		argPos++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filter.Limit)
		argPos++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filter.Offset)
	}

	var rows []RunRow
	if err := s.store.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}

	runs := make([]*contracts.Run, len(rows))
	for i, row := range rows {
		run, err := row.ToRun()
		if err != nil {
			return nil, err
		}
		runs[i] = run
	}

	return runs, nil
}

func (s *RunStore) GetCounters(ctx context.Context, runID string) (*contracts.RunCounters, error) {
	query := `SELECT counters FROM runs WHERE run_id = $1`

	var countersJSON []byte
	if err := s.store.db.GetContext(ctx, &countersJSON, query, runID); err != nil {
		return nil, fmt.Errorf("get counters: %w", err)
	}

	var counters contracts.RunCounters
	if err := json.Unmarshal(countersJSON, &counters); err != nil {
		return nil, fmt.Errorf("unmarshal counters: %w", err)
	}

	return &counters, nil
}

// RunStats holds server-computed dashboard statistics.
type RunStats struct {
	TotalRuns      int            `json:"total_runs" db:"total_runs"`
	ActiveRuns     int            `json:"active_runs" db:"active_runs"`
	FailedRuns     int            `json:"failed_runs" db:"failed_runs"`
	CompletedRuns  int            `json:"completed_runs" db:"completed_runs"`
	TotalToolCalls int            `json:"total_tool_calls"`
	TotalBlocks    int            `json:"total_blocks"`
	StatusCounts   map[string]int `json:"status_counts"`
}

// GetStats returns aggregate statistics computed in the database rather than
// forcing every client to fetch all runs and derive them locally.
func (s *RunStore) GetStats(ctx context.Context, orgID string) (*RunStats, error) {
	stats := &RunStats{StatusCounts: make(map[string]int)}

	// Aggregate counts by status
	type statusRow struct {
		Status string `db:"status"`
		Count  int    `db:"cnt"`
	}
	var rows []statusRow
	q := `SELECT status, COUNT(*) AS cnt FROM runs WHERE org_id = $1 GROUP BY status`
	if err := s.store.db.SelectContext(ctx, &rows, q, orgID); err != nil {
		return nil, fmt.Errorf("get run stats: %w", err)
	}

	for _, r := range rows {
		stats.StatusCounts[r.Status] = r.Count
		stats.TotalRuns += r.Count
		switch r.Status {
		case "running":
			stats.ActiveRuns += r.Count
		case "failed":
			stats.FailedRuns += r.Count
		case "completed":
			stats.CompletedRuns += r.Count
		}
	}

	// Aggregate counter totals (tool_calls, blocks) from the JSONB counters column
	var agg struct {
		ToolCalls int `db:"total_tool_calls"`
		Blocks    int `db:"total_blocks"`
	}
	aggQ := `SELECT
		COALESCE(SUM((counters->>'tool_calls')::int), 0) AS total_tool_calls,
		COALESCE(SUM((counters->>'blocks')::int), 0) AS total_blocks
	 FROM runs WHERE org_id = $1`
	if err := s.store.db.GetContext(ctx, &agg, aggQ, orgID); err != nil {
		return nil, fmt.Errorf("get aggregate counters: %w", err)
	}
	stats.TotalToolCalls = agg.ToolCalls
	stats.TotalBlocks = agg.Blocks

	return stats, nil
}
