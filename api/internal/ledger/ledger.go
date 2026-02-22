package ledger

import (
	"context"
	"fmt"

	"github.com/aegisrun/aegisrun/internal/store"
	"go.uber.org/zap"
)

// Ledger manages the tamper-evident event ledger
type Ledger struct {
	eventStore  *store.EventStore
	runStore    *store.RunStore
	policyStore *store.PolicyStore
	keyStore    *store.KeyStore
	hasher      *Hasher
	signer      *Signer
	logger      *zap.Logger
}

func NewLedger(
	eventStore *store.EventStore,
	runStore *store.RunStore,
	policyStore *store.PolicyStore,
	keyStore *store.KeyStore,
	logger *zap.Logger,
) *Ledger {
	return &Ledger{
		eventStore:  eventStore,
		runStore:    runStore,
		policyStore: policyStore,
		keyStore:    keyStore,
		hasher:      NewHasher(),
		signer:      NewSigner(),
		logger:      logger,
	}
}

// VerifyRunIntegrity verifies the integrity of a run's event chain
func (l *Ledger) VerifyRunIntegrity(ctx context.Context, runID string) error {
	events, err := l.eventStore.GetByRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("get events: %w", err)
	}

	if len(events) == 0 {
		return fmt.Errorf("no events found for run")
	}

	// Verify hash chain
	if err := l.hasher.VerifyEventChain(events); err != nil {
		return fmt.Errorf("chain verification failed: %w", err)
	}

	// Verify sequence numbers
	for i, event := range events {
		if event.SeqNo != i {
			return fmt.Errorf("sequence number mismatch at index %d: expected %d, got %d", i, i, event.SeqNo)
		}
	}

	l.logger.Info("run integrity verified", zap.String("run_id", runID), zap.Int("events", len(events)))
	return nil
}

// FinalizeRun computes evidence hash and signs the run
func (l *Ledger) FinalizeRun(ctx context.Context, runID string) error {
	// Get run
	run, err := l.runStore.Get(ctx, runID)
	if err != nil {
		return fmt.Errorf("get run: %w", err)
	}

	// Get events
	events, err := l.eventStore.GetByRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("get events: %w", err)
	}

	if len(events) == 0 {
		return fmt.Errorf("cannot finalize run with no events")
	}

	// Get policy
	policy, err := l.policyStore.Get(ctx, run.PolicyRef.PolicyID, run.PolicyRef.Version)
	if err != nil {
		return fmt.Errorf("get policy: %w", err)
	}

	// Get active signing key
	key, err := l.keyStore.GetActive(ctx, run.OrgID)
	if err != nil {
		return fmt.Errorf("get signing key: %w", err)
	}

	// Sign run
	evidenceHash, signature, err := l.signer.SignRun(run, events, policy, key.PrivateKey)
	if err != nil {
		return fmt.Errorf("sign run: %w", err)
	}

	// Update run with evidence
	if err := l.runStore.SetEvidence(ctx, runID, evidenceHash, signature, key.KeyID); err != nil {
		return fmt.Errorf("set evidence: %w", err)
	}

	l.logger.Info("run finalized",
		zap.String("run_id", runID),
		zap.String("evidence_hash", evidenceHash),
		zap.String("key_id", key.KeyID))

	return nil
}

// VerifyRunSignature verifies a run's signature
func (l *Ledger) VerifyRunSignature(ctx context.Context, runID string) (bool, error) {
	run, err := l.runStore.Get(ctx, runID)
	if err != nil {
		return false, fmt.Errorf("get run: %w", err)
	}

	if run.EvidenceHash == nil || run.Signature == nil {
		return false, fmt.Errorf("run not signed")
	}

	// Get signing key using the signer key ID (not the signature itself)
	if run.SignerKeyID == nil {
		return false, fmt.Errorf("run has no signer key ID")
	}
	key, err := l.keyStore.Get(ctx, *run.SignerKeyID)
	if err != nil {
		return false, fmt.Errorf("get signing key: %w", err)
	}

	// Verify signature
	valid, err := l.signer.VerifySignature(*run.EvidenceHash, *run.Signature, key.PublicKey)
	if err != nil {
		return false, fmt.Errorf("verify signature: %w", err)
	}

	return valid, nil
}
