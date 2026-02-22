package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/jmoiron/sqlx"
)

type ToolCallStore struct {
	store *Store
}

func NewToolCallStore(store *Store) *ToolCallStore {
	return &ToolCallStore{store: store}
}

type ToolCallRow struct {
	ToolCallID       string       `db:"tool_call_id"`
	RunID            string       `db:"run_id"`
	StepID           string       `db:"step_id"`
	SeqNo            int          `db:"seq_no"`
	ToolName         string       `db:"tool_name"`
	Args             []byte       `db:"args"`
	ArgsRedacted     bool         `db:"args_redacted"`
	RequestedAt      time.Time    `db:"requested_at"`
	RespondedAt      sql.NullTime `db:"responded_at"`
	Decision         []byte       `db:"decision"`
	Response         []byte       `db:"response"`
	ResponseRedacted bool         `db:"response_redacted"`
	Metadata         []byte       `db:"metadata"`
}

func (r *ToolCallRow) ToToolCall() (*contracts.ToolCall, error) {
	tc := &contracts.ToolCall{
		ToolCallID:       r.ToolCallID,
		RunID:            r.RunID,
		StepID:           r.StepID,
		SeqNo:            r.SeqNo,
		ToolName:         r.ToolName,
		ArgsRedacted:     r.ArgsRedacted,
		RequestedAt:      r.RequestedAt,
		ResponseRedacted: r.ResponseRedacted,
	}

	if err := json.Unmarshal(r.Args, &tc.Args); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	if err := json.Unmarshal(r.Decision, &tc.Decision); err != nil {
		return nil, fmt.Errorf("unmarshal decision: %w", err)
	}

	if r.RespondedAt.Valid {
		tc.RespondedAt = &r.RespondedAt.Time
	}

	if len(r.Response) > 0 {
		var response contracts.ToolResponse
		if err := json.Unmarshal(r.Response, &response); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}
		tc.Response = &response
	}

	if err := json.Unmarshal(r.Metadata, &tc.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return tc, nil
}

func (s *ToolCallStore) Create(ctx context.Context, tx *sqlx.Tx, tc *contracts.ToolCall) error {
	argsJSON, err := json.Marshal(tc.Args)
	if err != nil {
		return fmt.Errorf("marshal args: %w", err)
	}

	decisionJSON, err := json.Marshal(tc.Decision)
	if err != nil {
		return fmt.Errorf("marshal decision: %w", err)
	}

	metadataJSON, err := json.Marshal(tc.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	query := `
		INSERT INTO tool_calls (
			tool_call_id, run_id, step_id, seq_no, tool_name,
			args, args_redacted, requested_at, decision, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)`

	_, err = tx.ExecContext(ctx, query,
		tc.ToolCallID,
		tc.RunID,
		tc.StepID,
		tc.SeqNo,
		tc.ToolName,
		argsJSON,
		tc.ArgsRedacted,
		tc.RequestedAt,
		decisionJSON,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("insert tool_call: %w", err)
	}

	return nil
}

func (s *ToolCallStore) UpdateResponse(ctx context.Context, tx *sqlx.Tx, toolCallID string, response *contracts.ToolResponse, redacted bool) error {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}

	query := `
		UPDATE tool_calls
		SET responded_at = $1, response = $2, response_redacted = $3
		WHERE tool_call_id = $4`

	_, err = tx.ExecContext(ctx, query, time.Now(), responseJSON, redacted, toolCallID)
	if err != nil {
		return fmt.Errorf("update tool_call response: %w", err)
	}

	return nil
}

func (s *ToolCallStore) Get(ctx context.Context, toolCallID string) (*contracts.ToolCall, error) {
	query := `
		SELECT 
			tool_call_id, run_id, step_id, seq_no, tool_name,
			args, args_redacted, requested_at, responded_at,
			decision, response, response_redacted, metadata
		FROM tool_calls
		WHERE tool_call_id = $1`

	var row ToolCallRow
	if err := s.store.db.GetContext(ctx, &row, query, toolCallID); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewNotFoundError("tool_call", toolCallID)
		}
		return nil, fmt.Errorf("get tool_call: %w", err)
	}

	return row.ToToolCall()
}

func (s *ToolCallStore) ListByRun(ctx context.Context, runID string) ([]*contracts.ToolCall, error) {
	query := `
		SELECT 
			tool_call_id, run_id, step_id, seq_no, tool_name,
			args, args_redacted, requested_at, responded_at,
			decision, response, response_redacted, metadata
		FROM tool_calls
		WHERE run_id = $1
		ORDER BY seq_no ASC`

	var rows []ToolCallRow
	if err := s.store.db.SelectContext(ctx, &rows, query, runID); err != nil {
		return nil, fmt.Errorf("list tool_calls: %w", err)
	}

	tcs := make([]*contracts.ToolCall, len(rows))
	for i, row := range rows {
		tc, err := row.ToToolCall()
		if err != nil {
			return nil, err
		}
		tcs[i] = tc
	}

	return tcs, nil
}

func (s *ToolCallStore) ListByStep(ctx context.Context, stepID string) ([]*contracts.ToolCall, error) {
	query := `
		SELECT 
			tool_call_id, run_id, step_id, seq_no, tool_name,
			args, args_redacted, requested_at, responded_at,
			decision, response, response_redacted, metadata
		FROM tool_calls
		WHERE step_id = $1
		ORDER BY seq_no ASC`

	var rows []ToolCallRow
	if err := s.store.db.SelectContext(ctx, &rows, query, stepID); err != nil {
		return nil, fmt.Errorf("list tool_calls by step: %w", err)
	}

	tcs := make([]*contracts.ToolCall, len(rows))
	for i, row := range rows {
		tc, err := row.ToToolCall()
		if err != nil {
			return nil, err
		}
		tcs[i] = tc
	}

	return tcs, nil
}

type ToolCallFilter struct {
	OrgID     string
	ToolName  *string
	Action    *contracts.PolicyAction
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Offset    int
}

func (s *ToolCallStore) Search(ctx context.Context, filter ToolCallFilter) ([]*contracts.ToolCall, error) {
	query := `
		SELECT 
			tc.tool_call_id, tc.run_id, tc.step_id, tc.seq_no, tc.tool_name,
			tc.args, tc.args_redacted, tc.requested_at, tc.responded_at,
			tc.decision, tc.response, tc.response_redacted, tc.metadata
		FROM tool_calls tc
		INNER JOIN runs r ON tc.run_id = r.run_id
		WHERE r.org_id = $1`

	args := []interface{}{filter.OrgID}
	argPos := 2

	if filter.ToolName != nil {
		query += fmt.Sprintf(" AND tc.tool_name = $%d", argPos)
		args = append(args, *filter.ToolName)
		argPos++
	}

	if filter.Action != nil {
		query += fmt.Sprintf(" AND tc.decision->>'action' = $%d", argPos)
		args = append(args, string(*filter.Action))
		argPos++
	}

	if filter.StartTime != nil {
		query += fmt.Sprintf(" AND tc.requested_at >= $%d", argPos)
		args = append(args, *filter.StartTime)
		argPos++
	}

	if filter.EndTime != nil {
		query += fmt.Sprintf(" AND tc.requested_at <= $%d", argPos)
		args = append(args, *filter.EndTime)
		argPos++
	}

	query += " ORDER BY tc.requested_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filter.Limit)
		argPos++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filter.Offset)
	}

	var rows []ToolCallRow
	if err := s.store.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("search tool_calls: %w", err)
	}

	tcs := make([]*contracts.ToolCall, len(rows))
	for i, row := range rows {
		tc, err := row.ToToolCall()
		if err != nil {
			return nil, err
		}
		tcs[i] = tc
	}

	return tcs, nil
}
