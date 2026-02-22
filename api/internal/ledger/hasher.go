package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/gibson042/canonicaljson-go"
)

// Hasher computes canonical hashes for ledger objects
type Hasher struct{}

func NewHasher() *Hasher {
	return &Hasher{}
}

// HashEvent computes the hash for an event (excluding the event_hash field)
func (h *Hasher) HashEvent(event *contracts.Event) (string, error) {
	// Create copy without event_hash
	eventCopy := *event
	eventCopy.EventHash = ""

	canonical, err := canonicaljson.Marshal(eventCopy)
	if err != nil {
		return "", fmt.Errorf("canonical marshal: %w", err)
	}

	// Append prev_hash bytes if present
	var prevHashBytes []byte
	if event.PrevHash != nil {
		prevHashBytes, err = hex.DecodeString(*event.PrevHash)
		if err != nil {
			return "", fmt.Errorf("decode prev_hash: %w", err)
		}
	}

	toHash := append(canonical, prevHashBytes...)
	hash := sha256.Sum256(toHash)
	return hex.EncodeToString(hash[:]), nil
}

// HashPolicySpec computes the hash for a policy spec
func (h *Hasher) HashPolicySpec(spec *contracts.PolicySpec) (string, error) {
	canonical, err := canonicaljson.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("canonical marshal: %w", err)
	}

	hash := sha256.Sum256(canonical)
	return hex.EncodeToString(hash[:]), nil
}

// HashRunEvidence computes the root evidence hash for a run
func (h *Hasher) HashRunEvidence(lastEventHash string, policySpecHash string, outcomeCanonical []byte) (string, error) {
	// evidence_hash = SHA256(last_event_hash || policy_spec_hash || outcome_canonical)
	lastEventBytes, err := hex.DecodeString(lastEventHash)
	if err != nil {
		return "", fmt.Errorf("decode last_event_hash: %w", err)
	}

	policySpecBytes, err := hex.DecodeString(policySpecHash)
	if err != nil {
		return "", fmt.Errorf("decode policy_spec_hash: %w", err)
	}

	toHash := append(lastEventBytes, policySpecBytes...)
	toHash = append(toHash, outcomeCanonical...)

	hash := sha256.Sum256(toHash)
	return hex.EncodeToString(hash[:]), nil
}

// VerifyEventChain verifies the hash chain integrity
func (h *Hasher) VerifyEventChain(events []*contracts.Event) error {
	if len(events) == 0 {
		return nil
	}

	// First event should have nil prev_hash
	if events[0].PrevHash != nil {
		return fmt.Errorf("first event has non-nil prev_hash")
	}

	// Verify first event hash
	computedHash, err := h.HashEvent(events[0])
	if err != nil {
		return fmt.Errorf("compute hash for event 0: %w", err)
	}
	if computedHash != events[0].EventHash {
		return fmt.Errorf("event 0 hash mismatch: expected %s, got %s", events[0].EventHash, computedHash)
	}

	// Verify subsequent events
	for i := 1; i < len(events); i++ {
		if events[i].PrevHash == nil {
			return fmt.Errorf("event %d has nil prev_hash", i)
		}

		if *events[i].PrevHash != events[i-1].EventHash {
			return fmt.Errorf("event %d broken chain: prev_hash=%s, previous event_hash=%s",
				i, *events[i].PrevHash, events[i-1].EventHash)
		}

		computedHash, err := h.HashEvent(events[i])
		if err != nil {
			return fmt.Errorf("compute hash for event %d: %w", i, err)
		}
		if computedHash != events[i].EventHash {
			return fmt.Errorf("event %d hash mismatch: expected %s, got %s", i, events[i].EventHash, computedHash)
		}
	}

	return nil
}
