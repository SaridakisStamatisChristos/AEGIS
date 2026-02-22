package ledger

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/gibson042/canonicaljson-go"
)

// Signer handles Ed25519 signing operations
type Signer struct{}

func NewSigner() *Signer {
	return &Signer{}
}

// GenerateKey generates a new Ed25519 key pair
func (s *Signer) GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}
	return pub, priv, nil
}

// SignEvidence signs an evidence hash
func (s *Signer) SignEvidence(evidenceHash string, privateKey ed25519.PrivateKey) (string, error) {
	evidenceBytes, err := hex.DecodeString(evidenceHash)
	if err != nil {
		return "", fmt.Errorf("decode evidence hash: %w", err)
	}

	signature := ed25519.Sign(privateKey, evidenceBytes)
	return base64.StdEncoding.EncodeToString(signature), nil
}

// VerifySignature verifies an evidence signature
func (s *Signer) VerifySignature(evidenceHash string, signature string, publicKey ed25519.PublicKey) (bool, error) {
	evidenceBytes, err := hex.DecodeString(evidenceHash)
	if err != nil {
		return false, fmt.Errorf("decode evidence hash: %w", err)
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("decode signature: %w", err)
	}

	valid := ed25519.Verify(publicKey, evidenceBytes, signatureBytes)
	return valid, nil
}

// SignRun computes evidence hash and signs a run
func (s *Signer) SignRun(run *contracts.Run, events []*contracts.Event, policy *contracts.Policy, privateKey ed25519.PrivateKey) (string, string, error) {
	hasher := NewHasher()

	// Get last event hash
	if len(events) == 0 {
		return "", "", fmt.Errorf("no events to sign")
	}
	lastEventHash := events[len(events)-1].EventHash

	// Get policy spec hash
	policySpecHash := policy.SpecHash

	// Compute outcome canonical
	outcomeCanonical, err := canonicalizeOutcome(run.Outcome)
	if err != nil {
		return "", "", fmt.Errorf("canonicalize outcome: %w", err)
	}

	// Compute evidence hash
	evidenceHash, err := hasher.HashRunEvidence(lastEventHash, policySpecHash, outcomeCanonical)
	if err != nil {
		return "", "", fmt.Errorf("compute evidence hash: %w", err)
	}

	// Sign evidence hash
	signature, err := s.SignEvidence(evidenceHash, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("sign evidence: %w", err)
	}

	return evidenceHash, signature, nil
}

func canonicalizeOutcome(outcome *contracts.RunOutcome) ([]byte, error) {
	if outcome == nil {
		return []byte{}, nil
	}

	// Use canonical JSON for deterministic serialization
	canonical, err := canonicaljson.Marshal(outcome)
	if err != nil {
		return nil, fmt.Errorf("canonical marshal outcome: %w", err)
	}

	return canonical, nil
}
