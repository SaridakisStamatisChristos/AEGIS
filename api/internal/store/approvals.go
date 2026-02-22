package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/aegisrun/aegisrun/internal/contracts"
)

type ApprovalStore struct {
	store *Store
}

func NewApprovalStore(store *Store) *ApprovalStore {
	return &ApprovalStore{store: store}
}

type ApprovalRow struct {
	ApprovalID string         `db:"approval_id"`
	PolicyID   string         `db:"policy_id"`
	Version    string         `db:"version"`
	ApproverID string         `db:"approver_id"`
	Decision   string         `db:"decision"`
	Comment    sql.NullString `db:"comment"`
	CreatedAt  time.Time      `db:"created_at"`
}

func (r *ApprovalRow) ToApproval() *contracts.Approval {
	approval := &contracts.Approval{
		ApprovalID: r.ApprovalID,
		PolicyID:   r.PolicyID,
		Version:    r.Version,
		ApproverID: r.ApproverID,
		Decision:   r.Decision,
		CreatedAt:  r.CreatedAt,
	}

	if r.Comment.Valid {
		approval.Comment = &r.Comment.String
	}

	return approval
}

func (s *ApprovalStore) Create(ctx context.Context, approval *contracts.Approval) error {
	query := `
		INSERT INTO approvals (
			approval_id, policy_id, version, approver_id,
			decision, comment, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)`

	_, err := s.store.db.ExecContext(ctx, query,
		approval.ApprovalID,
		approval.PolicyID,
		approval.Version,
		approval.ApproverID,
		approval.Decision,
		approval.Comment,
		approval.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert approval: %w", err)
	}

	return nil
}

func (s *ApprovalStore) Get(ctx context.Context, approvalID string) (*contracts.Approval, error) {
	query := `
		SELECT 
			approval_id, policy_id, version, approver_id,
			decision, comment, created_at
		FROM approvals
		WHERE approval_id = $1`

	var row ApprovalRow
	if err := s.store.db.GetContext(ctx, &row, query, approvalID); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewNotFoundError("approval", approvalID)
		}
		return nil, fmt.Errorf("get approval: %w", err)
	}

	return row.ToApproval(), nil
}

func (s *ApprovalStore) ListByPolicy(ctx context.Context, policyID string, version string) ([]*contracts.Approval, error) {
	query := `
		SELECT 
			approval_id, policy_id, version, approver_id,
			decision, comment, created_at
		FROM approvals
		WHERE policy_id = $1 AND version = $2
		ORDER BY created_at DESC`

	var rows []ApprovalRow
	if err := s.store.db.SelectContext(ctx, &rows, query, policyID, version); err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}

	approvals := make([]*contracts.Approval, len(rows))
	for i, row := range rows {
		approvals[i] = row.ToApproval()
	}

	return approvals, nil
}

func (s *ApprovalStore) CountApprovals(ctx context.Context, policyID string, version string, decision string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM approvals
		WHERE policy_id = $1 AND version = $2 AND decision = $3`

	var count int
	if err := s.store.db.GetContext(ctx, &count, query, policyID, version, decision); err != nil {
		return 0, fmt.Errorf("count approvals: %w", err)
	}

	return count, nil
}

func (s *ApprovalStore) HasUserApproved(ctx context.Context, policyID string, version string, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM approvals
			WHERE policy_id = $1 AND version = $2 AND approver_id = $3
		)`

	var exists bool
	if err := s.store.db.GetContext(ctx, &exists, query, policyID, version, userID); err != nil {
		return false, fmt.Errorf("check user approval: %w", err)
	}

	return exists, nil
}
