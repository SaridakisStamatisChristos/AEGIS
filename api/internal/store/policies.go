package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aegisrun/aegisrun/internal/contracts"
)

type PolicyStore struct {
	store *Store
}

func NewPolicyStore(store *Store) *PolicyStore {
	return &PolicyStore{store: store}
}

type PolicyRow struct {
	PolicyID   string       `db:"policy_id"`
	OrgID      string       `db:"org_id"`
	Name       string       `db:"name"`
	Version    string       `db:"version"`
	Status     string       `db:"status"`
	Spec       []byte       `db:"spec"`
	SpecHash   string       `db:"spec_hash"`
	CreatedAt  time.Time    `db:"created_at"`
	CreatedBy  string       `db:"created_by"`
	ApprovedAt sql.NullTime `db:"approved_at"`
	ApprovedBy []byte       `db:"approved_by"`
	DeployedAt sql.NullTime `db:"deployed_at"`
	Metadata   []byte       `db:"metadata"`
}

func (r *PolicyRow) ToPolicy() (*contracts.Policy, error) {
	policy := &contracts.Policy{
		PolicyID:  r.PolicyID,
		OrgID:     r.OrgID,
		Name:      r.Name,
		Version:   r.Version,
		Status:    contracts.PolicyStatus(r.Status),
		SpecHash:  r.SpecHash,
		CreatedAt: r.CreatedAt,
	}

	if err := json.Unmarshal(r.Spec, &policy.Spec); err != nil {
		return nil, fmt.Errorf("unmarshal spec: %w", err)
	}

	if r.ApprovedAt.Valid {
		policy.ApprovedAt = &r.ApprovedAt.Time
	}

	if len(r.ApprovedBy) > 0 {
		if err := json.Unmarshal(r.ApprovedBy, &policy.ApprovedBy); err != nil {
			return nil, fmt.Errorf("unmarshal approved_by: %w", err)
		}
	}

	return policy, nil
}

func (s *PolicyStore) Create(ctx context.Context, policy *contracts.Policy, createdBy string) error {
	specJSON, err := json.Marshal(policy.Spec)
	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}

	query := `
		INSERT INTO policies (
			policy_id, org_id, name, version, status,
			spec, spec_hash, created_at, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)`

	_, err = s.store.db.ExecContext(ctx, query,
		policy.PolicyID,
		policy.OrgID,
		policy.Name,
		policy.Version,
		policy.Status,
		specJSON,
		policy.SpecHash,
		policy.CreatedAt,
		createdBy,
	)

	if err != nil {
		return fmt.Errorf("insert policy: %w", err)
	}

	return nil
}

func (s *PolicyStore) Get(ctx context.Context, policyID string, version string) (*contracts.Policy, error) {
	query := `
		SELECT 
			policy_id, org_id, name, version, status, spec, spec_hash,
			created_at, created_by, approved_at, approved_by, deployed_at, metadata
		FROM policies
		WHERE policy_id = $1 AND version = $2`

	var row PolicyRow
	if err := s.store.db.GetContext(ctx, &row, query, policyID, version); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewNotFoundError("policy", policyID+":"+version)
		}
		return nil, fmt.Errorf("get policy: %w", err)
	}

	return row.ToPolicy()
}

func (s *PolicyStore) GetByID(ctx context.Context, policyID string) (*contracts.Policy, error) {
	query := `
		SELECT 
			policy_id, org_id, name, version, status, spec, spec_hash,
			created_at, created_by, approved_at, approved_by, deployed_at, metadata
		FROM policies
		WHERE policy_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	var row PolicyRow
	if err := s.store.db.GetContext(ctx, &row, query, policyID); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewNotFoundError("policy", policyID)
		}
		return nil, fmt.Errorf("get policy: %w", err)
	}

	return row.ToPolicy()
}

func (s *PolicyStore) UpdateStatus(ctx context.Context, policyID string, version string, status contracts.PolicyStatus) error {
	query := `
		UPDATE policies
		SET status = $1
		WHERE policy_id = $2 AND version = $3`

	result, err := s.store.db.ExecContext(ctx, query, status, policyID, version)
	if err != nil {
		return fmt.Errorf("update policy status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return NewNotFoundError("policy", policyID+":"+version)
	}

	return nil
}

func (s *PolicyStore) SetApproved(ctx context.Context, policyID string, version string, approvers []string) error {
	approvedByJSON, err := json.Marshal(approvers)
	if err != nil {
		return fmt.Errorf("marshal approvers: %w", err)
	}

	query := `
		UPDATE policies
		SET status = $1, approved_at = $2, approved_by = $3
		WHERE policy_id = $4 AND version = $5`

	_, err = s.store.db.ExecContext(ctx, query,
		contracts.PolicyStatusApproved,
		time.Now(),
		approvedByJSON,
		policyID,
		version,
	)

	if err != nil {
		return fmt.Errorf("set policy approved: %w", err)
	}

	return nil
}

func (s *PolicyStore) SetDeployed(ctx context.Context, policyID string, version string) error {
	query := `
		UPDATE policies
		SET status = $1, deployed_at = $2
		WHERE policy_id = $3 AND version = $4`

	_, err := s.store.db.ExecContext(ctx, query,
		contracts.PolicyStatusDeployed,
		time.Now(),
		policyID,
		version,
	)

	if err != nil {
		return fmt.Errorf("set policy deployed: %w", err)
	}

	return nil
}

func (s *PolicyStore) ListVersions(ctx context.Context, orgID string, name string) ([]*contracts.Policy, error) {
	query := `
		SELECT 
			policy_id, org_id, name, version, status, spec, spec_hash,
			created_at, created_by, approved_at, approved_by, deployed_at, metadata
		FROM policies
		WHERE org_id = $1 AND name = $2
		ORDER BY created_at DESC`

	var rows []PolicyRow
	if err := s.store.db.SelectContext(ctx, &rows, query, orgID, name); err != nil {
		return nil, fmt.Errorf("list policy versions: %w", err)
	}

	policies := make([]*contracts.Policy, len(rows))
	for i, row := range rows {
		policy, err := row.ToPolicy()
		if err != nil {
			return nil, err
		}
		policies[i] = policy
	}

	return policies, nil
}

func (s *PolicyStore) List(ctx context.Context, orgID string, status *contracts.PolicyStatus) ([]*contracts.Policy, error) {
	query := `
		SELECT 
			policy_id, org_id, name, version, status, spec, spec_hash,
			created_at, created_by, approved_at, approved_by, deployed_at, metadata
		FROM policies
		WHERE org_id = $1`

	args := []interface{}{orgID}

	if status != nil {
		query += " AND status = $2"
		args = append(args, string(*status))
	}

	query += " ORDER BY created_at DESC"

	var rows []PolicyRow
	if err := s.store.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}

	policies := make([]*contracts.Policy, len(rows))
	for i, row := range rows {
		policy, err := row.ToPolicy()
		if err != nil {
			return nil, err
		}
		policies[i] = policy
	}

	return policies, nil
}

func (s *PolicyStore) GetDeployed(ctx context.Context, orgID string, name string) (*contracts.Policy, error) {
	query := `
		SELECT 
			policy_id, org_id, name, version, status, spec, spec_hash,
			created_at, created_by, approved_at, approved_by, deployed_at, metadata
		FROM policies
		WHERE org_id = $1 AND name = $2 AND status = $3
		ORDER BY deployed_at DESC
		LIMIT 1`

	var row PolicyRow
	if err := s.store.db.GetContext(ctx, &row, query, orgID, name, contracts.PolicyStatusDeployed); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no deployed policy found: %s", name)
		}
		return nil, fmt.Errorf("get deployed policy: %w", err)
	}

	return row.ToPolicy()
}
