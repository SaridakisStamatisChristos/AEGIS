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

type EventStore struct {
	store *Store
}

func NewEventStore(store *Store) *EventStore {
	return &EventStore{store: store}
}

// Store returns the underlying Store for transaction management.
func (s *EventStore) Store() *Store {
	return s.store
}

type EventRow struct {
	EventID   string         `db:"event_id"`
	RunID     string         `db:"run_id"`
	SeqNo     int            `db:"seq_no"`
	EventType string         `db:"event_type"`
	Timestamp time.Time      `db:"timestamp"`
	Payload   []byte         `db:"payload"`
	PrevHash  sql.NullString `db:"prev_hash"`
	EventHash string         `db:"event_hash"`
}

func (r *EventRow) ToEvent() (*contracts.Event, error) {
	event := &contracts.Event{
		EventID:   r.EventID,
		RunID:     r.RunID,
		SeqNo:     r.SeqNo,
		EventType: contracts.EventType(r.EventType),
		Timestamp: r.Timestamp,
		EventHash: r.EventHash,
	}

	if r.PrevHash.Valid {
		event.PrevHash = &r.PrevHash.String
	}

	if err := json.Unmarshal(r.Payload, &event.Payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	return event, nil
}

func (s *EventStore) Append(ctx context.Context, tx *sqlx.Tx, event *contracts.Event) error {
	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	query := `
		INSERT INTO events (
			event_id, run_id, seq_no, event_type, timestamp,
			payload, prev_hash, event_hash
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)`

	_, err = tx.ExecContext(ctx, query,
		event.EventID,
		event.RunID,
		event.SeqNo,
		event.EventType,
		event.Timestamp,
		payloadJSON,
		event.PrevHash,
		event.EventHash,
	)

	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	return nil
}

func (s *EventStore) GetByRun(ctx context.Context, runID string) ([]*contracts.Event, error) {
	query := `
		SELECT 
			event_id, run_id, seq_no, event_type, timestamp,
			payload, prev_hash, event_hash
		FROM events
		WHERE run_id = $1
		ORDER BY seq_no ASC`

	var rows []EventRow
	if err := s.store.db.SelectContext(ctx, &rows, query, runID); err != nil {
		return nil, fmt.Errorf("get events by run: %w", err)
	}

	events := make([]*contracts.Event, len(rows))
	for i, row := range rows {
		event, err := row.ToEvent()
		if err != nil {
			return nil, err
		}
		events[i] = event
	}

	return events, nil
}

func (s *EventStore) GetLastEvent(ctx context.Context, tx *sqlx.Tx, runID string) (*contracts.Event, error) {
	query := `
		SELECT 
			event_id, run_id, seq_no, event_type, timestamp,
			payload, prev_hash, event_hash
		FROM events
		WHERE run_id = $1
		ORDER BY seq_no DESC
		LIMIT 1`

	var row EventRow
	if err := tx.GetContext(ctx, &row, query, runID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No events yet
		}
		return nil, fmt.Errorf("get last event: %w", err)
	}

	return row.ToEvent()
}

func (s *EventStore) GetNextSeqNo(ctx context.Context, tx *sqlx.Tx, runID string) (int, error) {
	query := `
		SELECT COALESCE(MAX(seq_no), -1) + 1
		FROM events
		WHERE run_id = $1`

	var seqNo int
	if err := tx.GetContext(ctx, &seqNo, query, runID); err != nil {
		return 0, fmt.Errorf("get next seq_no: %w", err)
	}

	return seqNo, nil
}

func (s *EventStore) VerifyChainIntegrity(ctx context.Context, runID string) (bool, error) {
	events, err := s.GetByRun(ctx, runID)
	if err != nil {
		return false, err
	}

	if len(events) == 0 {
		return true, nil
	}

	// First event should have nil prev_hash
	if events[0].PrevHash != nil {
		return false, fmt.Errorf("first event has non-nil prev_hash")
	}

	// Verify chain links
	for i := 1; i < len(events); i++ {
		if events[i].PrevHash == nil {
			return false, fmt.Errorf("event %d has nil prev_hash", i)
		}
		if *events[i].PrevHash != events[i-1].EventHash {
			return false, fmt.Errorf("broken chain at event %d", i)
		}
	}

	return true, nil
}

type EventFilter struct {
	OrgID     string
	EventType *contracts.EventType
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Offset    int
}

func (s *EventStore) Search(ctx context.Context, filter EventFilter) ([]*contracts.Event, error) {
	query := `
		SELECT 
			e.event_id, e.run_id, e.seq_no, e.event_type, e.timestamp,
			e.payload, e.prev_hash, e.event_hash
		FROM events e
		INNER JOIN runs r ON e.run_id = r.run_id
		WHERE r.org_id = $1`

	args := []interface{}{filter.OrgID}
	argPos := 2

	if filter.EventType != nil {
		query += fmt.Sprintf(" AND e.event_type = $%d", argPos)
		args = append(args, string(*filter.EventType))
		argPos++
	}

	if filter.StartTime != nil {
		query += fmt.Sprintf(" AND e.timestamp >= $%d", argPos)
		args = append(args, *filter.StartTime)
		argPos++
	}

	if filter.EndTime != nil {
		query += fmt.Sprintf(" AND e.timestamp <= $%d", argPos)
		args = append(args, *filter.EndTime)
		argPos++
	}

	query += " ORDER BY e.timestamp DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filter.Limit)
		argPos++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filter.Offset)
	}

	var rows []EventRow
	if err := s.store.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("search events: %w", err)
	}

	events := make([]*contracts.Event, len(rows))
	for i, row := range rows {
		event, err := row.ToEvent()
		if err != nil {
			return nil, err
		}
		events[i] = event
	}

	return events, nil
}
