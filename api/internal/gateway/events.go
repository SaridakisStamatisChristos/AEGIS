package gateway

import (
	"context"
	"fmt"
	"time"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/ledger"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
)

func (g *Gateway) emitToolRequestedEvent(ctx context.Context, tx *sqlx.Tx, tc *contracts.ToolCall) error {
	seqNo, err := g.eventStore.GetNextSeqNo(ctx, tx, tc.RunID)
	if err != nil {
		return err
	}

	lastEvent, err := g.eventStore.GetLastEvent(ctx, tx, tc.RunID)
	if err != nil {
		return err
	}

	var prevHash *string
	if lastEvent != nil {
		prevHash = &lastEvent.EventHash
	}

	event := &contracts.Event{
		EventID:   ulid.Make().String(),
		RunID:     tc.RunID,
		SeqNo:     seqNo,
		EventType: contracts.EventToolRequested,
		Timestamp: time.Now().UTC(),
		Payload: map[string]interface{}{
			"tool_call_id": tc.ToolCallID,
			"step_id":      tc.StepID,
			"tool_name":    tc.ToolName,
			"args":         tc.Args,
		},
		PrevHash: prevHash,
	}

	hash, err := computeEventHash(event)
	if err != nil {
		return fmt.Errorf("compute event hash: %w", err)
	}
	event.EventHash = hash

	return g.eventStore.Append(ctx, tx, event)
}

func (g *Gateway) emitToolDecidedEvent(ctx context.Context, tx *sqlx.Tx, tc *contracts.ToolCall) error {
	seqNo, err := g.eventStore.GetNextSeqNo(ctx, tx, tc.RunID)
	if err != nil {
		return err
	}

	lastEvent, err := g.eventStore.GetLastEvent(ctx, tx, tc.RunID)
	if err != nil {
		return err
	}

	var prevHash *string
	if lastEvent != nil {
		prevHash = &lastEvent.EventHash
	}

	event := &contracts.Event{
		EventID:   ulid.Make().String(),
		RunID:     tc.RunID,
		SeqNo:     seqNo,
		EventType: contracts.EventToolDecided,
		Timestamp: time.Now().UTC(),
		Payload: map[string]interface{}{
			"tool_call_id": tc.ToolCallID,
			"decision":     tc.Decision,
		},
		PrevHash: prevHash,
	}

	hash, err := computeEventHash(event)
	if err != nil {
		return fmt.Errorf("compute event hash: %w", err)
	}
	event.EventHash = hash

	return g.eventStore.Append(ctx, tx, event)
}

func (g *Gateway) emitToolRespondedEvent(ctx context.Context, tx *sqlx.Tx, tc *contracts.ToolCall, resp *ToolCallResponse) error {
	seqNo, err := g.eventStore.GetNextSeqNo(ctx, tx, tc.RunID)
	if err != nil {
		return err
	}

	lastEvent, err := g.eventStore.GetLastEvent(ctx, tx, tc.RunID)
	if err != nil {
		return err
	}

	var prevHash *string
	if lastEvent != nil {
		prevHash = &lastEvent.EventHash
	}

	payload := map[string]interface{}{
		"tool_call_id": tc.ToolCallID,
	}

	if resp.Result != nil {
		payload["result"] = resp.Result
	}
	if resp.Error != "" {
		payload["error"] = resp.Error
	}

	event := &contracts.Event{
		EventID:   ulid.Make().String(),
		RunID:     tc.RunID,
		SeqNo:     seqNo,
		EventType: contracts.EventToolResponded,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
		PrevHash:  prevHash,
	}

	hash, err := computeEventHash(event)
	if err != nil {
		return fmt.Errorf("compute event hash: %w", err)
	}
	event.EventHash = hash

	return g.eventStore.Append(ctx, tx, event)
}

// computeEventHash delegates to ledger.Hasher.HashEvent to avoid duplicate hashing logic.
func computeEventHash(event *contracts.Event) (string, error) {
	hasher := ledger.NewHasher()
	return hasher.HashEvent(event)
}
