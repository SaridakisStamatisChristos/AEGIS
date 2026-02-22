package ledger

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/store"
	"go.uber.org/zap"
)

// Bundler creates offline-verifiable evidence bundles
type Bundler struct {
	eventStore  *store.EventStore
	runStore    *store.RunStore
	policyStore *store.PolicyStore
	keyStore    *store.KeyStore
	logger      *zap.Logger
}

func NewBundler(
	eventStore *store.EventStore,
	runStore *store.RunStore,
	policyStore *store.PolicyStore,
	keyStore *store.KeyStore,
	logger *zap.Logger,
) *Bundler {
	return &Bundler{
		eventStore:  eventStore,
		runStore:    runStore,
		policyStore: policyStore,
		keyStore:    keyStore,
		logger:      logger,
	}
}

// CreateBundle creates a ZIP archive with all evidence for a run
func (b *Bundler) CreateBundle(ctx context.Context, runID string, w io.Writer) error {
	// Get run
	run, err := b.runStore.Get(ctx, runID)
	if err != nil {
		return fmt.Errorf("get run: %w", err)
	}

	if run.EvidenceHash == nil || run.Signature == nil {
		return fmt.Errorf("run not finalized (missing evidence hash or signature)")
	}

	// Get events
	events, err := b.eventStore.GetByRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("get events: %w", err)
	}

	// Get policy
	policy, err := b.policyStore.Get(ctx, run.PolicyRef.PolicyID, run.PolicyRef.Version)
	if err != nil {
		return fmt.Errorf("get policy: %w", err)
	}

	// Get signing key (public key only for bundle)
	var signerKeyID string
	if run.Signature != nil {
		// In real implementation, we'd store signer_key_id separately
		// For now, get active key
		key, err := b.keyStore.GetActive(ctx, run.OrgID)
		if err != nil {
			return fmt.Errorf("get signing key: %w", err)
		}
		signerKeyID = key.KeyID
	}

	// Create bundle
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// 1. Write manifest.json
	manifest := contracts.EvidenceBundleManifest{
		BundleVersion: "1.0.0",
		RunID:         runID,
		ExportedAt:    time.Now().UTC(),
		EventCount:    len(events),
		RootHash:      *run.EvidenceHash,
		Signature:     *run.Signature,
		SignerKeyID:   signerKeyID,
	}

	if err := b.writeJSON(zipWriter, "manifest.json", manifest); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// 2. Write events.jsonl
	if err := b.writeEventsJSONL(zipWriter, events); err != nil {
		return fmt.Errorf("write events: %w", err)
	}

	// 3. Write policy_snapshot.json
	if err := b.writeJSON(zipWriter, "policy_snapshot.json", policy); err != nil {
		return fmt.Errorf("write policy: %w", err)
	}

	// 4. Write run.json
	if err := b.writeJSON(zipWriter, "run.json", run); err != nil {
		return fmt.Errorf("write run: %w", err)
	}

	// 5. Write public_key.pem (if available)
	if signerKeyID != "" {
		key, err := b.keyStore.Get(ctx, signerKeyID)
		if err == nil {
			if err := b.writePublicKey(zipWriter, key); err != nil {
				b.logger.Warn("failed to write public key", zap.Error(err))
			}
		}
	}

	// 6. Write README.txt
	if err := b.writeReadme(zipWriter, runID); err != nil {
		return fmt.Errorf("write readme: %w", err)
	}

	b.logger.Info("evidence bundle created",
		zap.String("run_id", runID),
		zap.Int("events", len(events)))

	return nil
}

func (b *Bundler) writeJSON(zw *zip.Writer, filename string, data interface{}) error {
	f, err := zw.Create(filename)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (b *Bundler) writeEventsJSONL(zw *zip.Writer, events []*contracts.Event) error {
	f, err := zw.Create("events.jsonl")
	if err != nil {
		return err
	}

	for _, event := range events {
		eventJSON, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(eventJSON, '\n')); err != nil {
			return err
		}
	}

	return nil
}

func (b *Bundler) writePublicKey(zw *zip.Writer, key *contracts.SigningKey) error {
	f, err := zw.Create("public_key.pem")
	if err != nil {
		return err
	}

	// Write PEM-encoded public key
	pem := fmt.Sprintf("-----BEGIN PUBLIC KEY-----\n%s\n-----END PUBLIC KEY-----\n",
		base64Encode(key.PublicKey))

	_, err = f.Write([]byte(pem))
	return err
}

func (b *Bundler) writeReadme(zw *zip.Writer, runID string) error {
	f, err := zw.Create("README.txt")
	if err != nil {
		return err
	}

	readme := fmt.Sprintf(`AegisRun Evidence Bundle
========================

Run ID: %s
Exported: %s

This bundle contains tamper-evident evidence for an agent run.

Contents:
- manifest.json: Bundle metadata and signature
- events.jsonl: Complete event chain (one event per line)
- policy_snapshot.json: Policy version used for this run
- run.json: Run metadata and outcome
- public_key.pem: Public key for signature verification

Verification:
Use the AegisRun verifier CLI to verify this bundle:

  aegis-verify bundle.zip

The verifier will:
1. Check event chain integrity (hash chaining)
2. Verify policy immutability
3. Validate Ed25519 signature
4. Check for tampering

For more information, visit: https://aegisrun.io/docs/evidence
`, runID, time.Now().UTC().Format(time.RFC3339))

	_, err = f.Write([]byte(readme))
	return err
}

func base64Encode(data []byte) string {
	// Simple base64 encoding in chunks
	encoded := base64.StdEncoding.EncodeToString(data)

	// Break into 64-char lines
	var result string
	for i := 0; i < len(encoded); i += 64 {
		end := i + 64
		if end > len(encoded) {
			end = len(encoded)
		}
		result += encoded[i:end] + "\n"
	}

	return result
}
