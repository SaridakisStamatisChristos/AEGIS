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

type StepStore struct {
	store *Store
}

func NewStepStore(store *Store) *StepStore {
	return &StepStore{store: store}
}

type StepRow struct {
	StepID       string         `db:"step_id"`
	RunID        string         `db:"run_id"`
	ParentStepID sql.NullString `db:"parent_step_id"`
	SeqNo        int            `db:"seq_no"`
	Name         string         `db:"name"`
	StateVector  []byte         `db:"state_vector"`
	StartedAt    time.Time      `db:"started_at"`
	EndedAt      sql.NullTime   `db:"ended_at"`
	Status       string         `db:"status"`
	Error        sql.NullString `db:"error"`
}

func (r *StepRow) ToStep() (*contracts.Step, error) {
	step := &contracts.Step{
		StepID:    r.StepID,
		RunID:     r.RunID,
		SeqNo:     r.SeqNo,
		Name:      r.Name,
		StartedAt: r.StartedAt,
		Status:    contracts.StepStatus(r.Status),
	}

	if r.ParentStepID.Valid {
		step.ParentStepID = &r.ParentStepID.String
	}

	if err := json.Unmarshal(r.StateVector, &step.StateVector); err != nil {
		return nil, fmt.Errorf("unmarshal state_vector: %w", err)
	}

	if r.EndedAt.Valid {
		step.EndedAt = &r.EndedAt.Time
	}

	if r.Error.Valid {
		step.Error = &r.Error.String
	}

	return step, nil
}

func (s *StepStore) Create(ctx context.Context, tx *sqlx.Tx, step *contracts.Step) error {
	stateVector, err := json.Marshal(step.StateVector)
	if err != nil {
		return fmt.Errorf("marshal state_vector: %w", err)
	}

	query := `
		INSERT INTO steps (
			step_id, run_id, parent_step_id, seq_no, name,
			state_vector, started_at, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)`

	_, err = tx.ExecContext(ctx, query,
		step.StepID,
		step.RunID,
		step.ParentStepID,
		step.SeqNo,
		step.Name,
		stateVector,
		step.StartedAt,
		step.Status,
	)

	if err != nil {
		return fmt.Errorf("insert step: %w", err)
	}

	return nil
}

func (s *StepStore) Get(ctx context.Context, stepID string) (*contracts.Step, error) {
	query := `
		SELECT 
			step_id, run_id, parent_step_id, seq_no, name,
			state_vector, started_at, ended_at, status, error
		FROM steps
		WHERE step_id = $1`

	var row StepRow
	if err := s.store.db.GetContext(ctx, &row, query, stepID); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewNotFoundError("step", stepID)
		}
		return nil, fmt.Errorf("get step: %w", err)
	}

	return row.ToStep()
}

func (s *StepStore) UpdateStatus(ctx context.Context, tx *sqlx.Tx, stepID string, status contracts.StepStatus, stepError *string) error {
	query := `
		UPDATE steps
		SET status = $1, ended_at = $2, error = $3
		WHERE step_id = $4`

	_, err := tx.ExecContext(ctx, query, status, time.Now(), stepError, stepID)
	if err != nil {
		return fmt.Errorf("update step status: %w", err)
	}

	return nil
}

func (s *StepStore) ListByRun(ctx context.Context, runID string) ([]*contracts.Step, error) {
	query := `
		SELECT 
			step_id, run_id, parent_step_id, seq_no, name,
			state_vector, started_at, ended_at, status, error
		FROM steps
		WHERE run_id = $1
		ORDER BY seq_no ASC`

	var rows []StepRow
	if err := s.store.db.SelectContext(ctx, &rows, query, runID); err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}

	steps := make([]*contracts.Step, len(rows))
	for i, row := range rows {
		step, err := row.ToStep()
		if err != nil {
			return nil, err
		}
		steps[i] = step
	}

	return steps, nil
}
