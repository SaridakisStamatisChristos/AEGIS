// Package verifier provides evidence bundle verification logic
package verifier

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/aegisrun/aegis-verify/internal/bundle"
	canonicaljson "github.com/gibson042/canonicaljson-go"
)

// ChainVerifier verifies event chain integrity
type ChainVerifier struct {
	config Config
}

// NewChainVerifier creates a new ChainVerifier instance
func NewChainVerifier(config Config) *ChainVerifier {
	return &ChainVerifier{config: config}
}

// Verify verifies the hash chain integrity of events
func (v *ChainVerifier) Verify(events []bundle.Event) CheckResult {
	if len(events) == 0 {
		return CheckResult{
			Passed:  true,
			Message: "No events to verify (empty chain)",
		}
	}

	// First event should have nil prev_hash
	if events[0].PrevHash != nil && *events[0].PrevHash != "" {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("First event has non-nil prev_hash: %s", *events[0].PrevHash),
		}
	}

	// Verify first event hash
	computedHash, err := v.computeEventHash(&events[0])
	if err != nil {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("Failed to compute hash for event 0: %v", err),
		}
	}
	if computedHash != events[0].EventHash {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("Event 0 hash mismatch: expected %s, got %s", events[0].EventHash, computedHash),
		}
	}

	// Verify first event seq_no (store uses 0-based sequence numbers)
	if events[0].SeqNo != 0 {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("First event has incorrect seq_no: expected 0, got %d", events[0].SeqNo),
		}
	}

	if v.config.Verbose {
		fmt.Printf("  Event 0: hash verified (%s...)\n", events[0].EventHash[:16])
	}

	// Verify subsequent events
	for i := 1; i < len(events); i++ {
		event := &events[i]
		prevEvent := &events[i-1]

		// Check prev_hash link
		if event.PrevHash == nil {
			return CheckResult{
				Passed:  false,
				Message: fmt.Sprintf("Event %d has nil prev_hash", i),
			}
		}

		if *event.PrevHash != prevEvent.EventHash {
			return CheckResult{
				Passed: false,
				Message: fmt.Sprintf("Broken chain at event %d: prev_hash=%s, previous event_hash=%s",
					i, *event.PrevHash, prevEvent.EventHash),
			}
		}

		// Verify event hash
		computedHash, err := v.computeEventHash(event)
		if err != nil {
			return CheckResult{
				Passed:  false,
				Message: fmt.Sprintf("Failed to compute hash for event %d: %v", i, err),
			}
		}
		if computedHash != event.EventHash {
			return CheckResult{
				Passed:  false,
				Message: fmt.Sprintf("Event %d hash mismatch: expected %s, got %s", i, event.EventHash, computedHash),
			}
		}

		// Check sequence numbers (0-based: event at index i should have seq_no i)
		expectedSeqNo := i
		if event.SeqNo != expectedSeqNo {
			return CheckResult{
				Passed:  false,
				Message: fmt.Sprintf("Event %d has incorrect seq_no: expected %d, got %d", i, expectedSeqNo, event.SeqNo),
			}
		}

		if v.config.Verbose {
			fmt.Printf("  Event %d: hash verified (%s...)\n", i, event.EventHash[:16])
		}
	}

	return CheckResult{
		Passed:  true,
		Message: fmt.Sprintf("All %d events verified with intact hash chain", len(events)),
	}
}

// computeEventHash computes the hash for an event
// Hash calculation: SHA256(canonical_json(event_without_hash) || prev_hash)
func (v *ChainVerifier) computeEventHash(event *bundle.Event) (string, error) {
	// Create copy without event_hash
	eventCopy := *event
	eventCopy.EventHash = ""

	canonical, err := canonicaljson.Marshal(eventCopy)
	if err != nil {
		return "", fmt.Errorf("canonical marshal: %w", err)
	}

	// Append prev_hash bytes if present
	var prevHashBytes []byte
	if event.PrevHash != nil && *event.PrevHash != "" {
		prevHashBytes, err = hex.DecodeString(*event.PrevHash)
		if err != nil {
			return "", fmt.Errorf("decode prev_hash: %w", err)
		}
	}

	toHash := append(canonical, prevHashBytes...)
	hash := sha256.Sum256(toHash)
	return hex.EncodeToString(hash[:]), nil
}
