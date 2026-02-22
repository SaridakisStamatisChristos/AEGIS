// Package verifier provides evidence bundle verification logic
package verifier

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/aegisrun/aegis-verify/internal/bundle"
	canonicaljson "github.com/gibson042/canonicaljson-go"
)

// SignatureVerifier verifies Ed25519 signatures on evidence bundles
type SignatureVerifier struct {
	config Config
}

// NewSignatureVerifier creates a new SignatureVerifier instance
func NewSignatureVerifier(config Config) *SignatureVerifier {
	return &SignatureVerifier{config: config}
}

// Verify verifies the Ed25519 signature on the evidence bundle
func (v *SignatureVerifier) Verify(b *bundle.Bundle) CheckResult {
	if b.Manifest.Signature == "" {
		return CheckResult{
			Passed:  false,
			Message: "Bundle has no signature",
		}
	}

	if b.PublicKey == nil {
		return CheckResult{
			Passed:  false,
			Message: "Bundle has no public key for signature verification",
		}
	}

	// Compute evidence hash
	if len(b.Events) == 0 {
		return CheckResult{
			Passed:  false,
			Message: "Cannot verify signature: no events in bundle",
		}
	}

	if b.Policy == nil {
		return CheckResult{
			Passed:  false,
			Message: "Cannot verify signature: bundle has no policy snapshot",
		}
	}

	lastEventHash := b.Events[len(b.Events)-1].EventHash
	policySpecHash := b.Policy.SpecHash

	// Compute outcome canonical (empty if no outcome)
	var outcomeCanonical []byte
	if b.Run != nil && b.Run.Outcome != nil {
		var err error
		outcomeCanonical, err = canonicaljson.Marshal(b.Run.Outcome)
		if err != nil {
			return CheckResult{
				Passed:  false,
				Message: fmt.Sprintf("Failed to canonicalize outcome: %v", err),
			}
		}
	}

	// Compute evidence hash: SHA256(last_event_hash || policy_spec_hash || outcome_canonical)
	lastEventBytes, err := hex.DecodeString(lastEventHash)
	if err != nil {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("Failed to decode last_event_hash: %v", err),
		}
	}

	policySpecBytes, err := hex.DecodeString(policySpecHash)
	if err != nil {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("Failed to decode policy_spec_hash: %v", err),
		}
	}

	toHash := append(lastEventBytes, policySpecBytes...)
	toHash = append(toHash, outcomeCanonical...)
	hash := sha256.Sum256(toHash)
	evidenceHash := hex.EncodeToString(hash[:])

	if v.config.Verbose {
		fmt.Printf("  Evidence hash: %s\n", evidenceHash)
	}

	// Verify signature
	valid, err := v.verifyEd25519Signature(evidenceHash, b.Manifest.Signature, b.PublicKey)
	if err != nil {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("Signature verification failed: %v", err),
		}
	}

	if !valid {
		return CheckResult{
			Passed:  false,
			Message: "Signature verification failed: invalid signature",
		}
	}

	return CheckResult{
		Passed:  true,
		Message: fmt.Sprintf("Signature verified with key %s", b.Manifest.SignerKeyID),
	}
}

// verifyEd25519Signature verifies an Ed25519 signature
func (v *SignatureVerifier) verifyEd25519Signature(message string, signatureBase64 string, publicKey ed25519.PublicKey) (bool, error) {
	messageBytes, err := hex.DecodeString(message)
	if err != nil {
		return false, fmt.Errorf("decode message: %w", err)
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return false, fmt.Errorf("decode signature: %w", err)
	}

	return ed25519.Verify(publicKey, messageBytes, signatureBytes), nil
}
