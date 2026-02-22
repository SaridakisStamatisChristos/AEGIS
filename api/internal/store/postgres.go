package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type Store struct {
	db     *sqlx.DB
	logger *zap.Logger
}

func New(cfg Config, logger *zap.Logger) (*Store, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("database connection established",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Database))

	return &Store{
		db:     db,
		logger: logger,
	}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sqlx.DB {
	return s.db
}

// WithTx executes a function within a transaction
func (s *Store) WithTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			s.logger.Error("failed to rollback transaction",
				zap.Error(rbErr),
				zap.Error(err))
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SetCurrentUserID sets the current user ID in session for audit triggers
func SetCurrentUserID(ctx context.Context, tx *sqlx.Tx, userID string) error {
	_, err := tx.ExecContext(ctx, "SELECT set_config('app.current_user_id', $1, true)", userID)
	return err
}

// AcquireRunLock acquires an advisory lock for a run to serialize event writes
func (s *Store) AcquireRunLock(ctx context.Context, tx *sqlx.Tx, runID string) error {
	query := `SELECT pg_advisory_xact_lock(hashtext($1))`
	_, err := tx.ExecContext(ctx, query, runID)
	if err != nil {
		return fmt.Errorf("failed to acquire run lock: %w", err)
	}
	return nil
}

// Health check
func (s *Store) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var result int
	if err := s.db.GetContext(ctx, &result, "SELECT 1"); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	return nil
}

// Ping is an alias for Health (used by health handler)
func (s *Store) Ping(ctx context.Context) error {
	return s.Health(ctx)
}

// Helper for nullable strings
func NullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func StringFromNull(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

// Helper for nullable times
func NullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func TimeFromNull(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}
