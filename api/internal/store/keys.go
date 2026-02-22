package store

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"fmt"
	"time"

	"github.com/aegisrun/aegisrun/internal/contracts"
)

type KeyStore struct {
	store *Store
}

func NewKeyStore(store *Store) *KeyStore {
	return &KeyStore{store: store}
}

type KeyRow struct {
	KeyID      string    `db:"key_id"`
	OrgID      string    `db:"org_id"`
	PublicKey  []byte    `db:"public_key"`
	PrivateKey []byte    `db:"private_key"`
	CreatedAt  time.Time `db:"created_at"`
	Status     string    `db:"status"`
}

func (r *KeyRow) ToSigningKey() *contracts.SigningKey {
	return &contracts.SigningKey{
		KeyID:      r.KeyID,
		OrgID:      r.OrgID,
		PublicKey:  ed25519.PublicKey(r.PublicKey),
		PrivateKey: ed25519.PrivateKey(r.PrivateKey),
		CreatedAt:  r.CreatedAt,
		Status:     contracts.KeyStatus(r.Status),
	}
}

func (s *KeyStore) Create(ctx context.Context, key *contracts.SigningKey) error {
	query := `
		INSERT INTO signing_keys (
			key_id, org_id, public_key, private_key, created_at, status
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)`

	_, err := s.store.db.ExecContext(ctx, query,
		key.KeyID,
		key.OrgID,
		[]byte(key.PublicKey),
		[]byte(key.PrivateKey),
		key.CreatedAt,
		key.Status,
	)

	if err != nil {
		return fmt.Errorf("insert signing_key: %w", err)
	}

	return nil
}

func (s *KeyStore) Get(ctx context.Context, keyID string) (*contracts.SigningKey, error) {
	query := `
		SELECT key_id, org_id, public_key, private_key, created_at, status
		FROM signing_keys
		WHERE key_id = $1`

	var row KeyRow
	if err := s.store.db.GetContext(ctx, &row, query, keyID); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewNotFoundError("signing_key", keyID)
		}
		return nil, fmt.Errorf("get signing_key: %w", err)
	}

	return row.ToSigningKey(), nil
}

func (s *KeyStore) GetActive(ctx context.Context, orgID string) (*contracts.SigningKey, error) {
	query := `
		SELECT key_id, org_id, public_key, private_key, created_at, status
		FROM signing_keys
		WHERE org_id = $1 AND status = $2
		ORDER BY created_at DESC
		LIMIT 1`

	var row KeyRow
	if err := s.store.db.GetContext(ctx, &row, query, orgID, contracts.KeyStatusActive); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active signing_key found for org: %s", orgID)
		}
		return nil, fmt.Errorf("get active signing_key: %w", err)
	}

	return row.ToSigningKey(), nil
}

func (s *KeyStore) UpdateStatus(ctx context.Context, keyID string, status contracts.KeyStatus) error {
	query := `
		UPDATE signing_keys
		SET status = $1
		WHERE key_id = $2`

	result, err := s.store.db.ExecContext(ctx, query, status, keyID)
	if err != nil {
		return fmt.Errorf("update signing_key status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return NewNotFoundError("signing_key", keyID)
	}

	return nil
}

func (s *KeyStore) List(ctx context.Context, orgID string) ([]*contracts.SigningKey, error) {
	query := `
		SELECT key_id, org_id, public_key, private_key, created_at, status
		FROM signing_keys
		WHERE org_id = $1
		ORDER BY created_at DESC`

	var rows []KeyRow
	if err := s.store.db.SelectContext(ctx, &rows, query, orgID); err != nil {
		return nil, fmt.Errorf("list signing_keys: %w", err)
	}

	keys := make([]*contracts.SigningKey, len(rows))
	for i, row := range rows {
		keys[i] = row.ToSigningKey()
	}

	return keys, nil
}
