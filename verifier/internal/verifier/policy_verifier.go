// Package verifier provides evidence bundle verification logic
package verifier

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aegisrun/aegis-verify/internal/bundle"
	canonicaljson "github.com/gibson042/canonicaljson-go"
)

// Config holds configuration for verifiers
type Config struct {
	Verbose    bool
	JSONOutput bool
}

// CheckResult represents the outcome of a single verification check
type CheckResult struct {
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// VerificationResult holds the complete verification outcome
type VerificationResult struct {
	Valid           bool                   `json:"valid"`
	BundlePath      string                 `json:"bundle_path"`
	RunID           string                 `json:"run_id"`
	VerifiedAt      time.Time              `json:"verified_at"`
	EventCount      int                    `json:"event_count"`
	ChainIntegrity  CheckResult            `json:"chain_integrity"`
	SignatureValid  CheckResult            `json:"signature_valid"`
	PolicyIntegrity CheckResult            `json:"policy_integrity"`
	Warnings        []string               `json:"warnings,omitempty"`
	Errors          []string               `json:"errors,omitempty"`
	Details         map[string]interface{} `json:"details,omitempty"`
}

// NewResult creates a new VerificationResult
func NewResult(bundlePath, runID string, eventCount int) *VerificationResult {
	return &VerificationResult{
		BundlePath: bundlePath,
		RunID:      runID,
		EventCount: eventCount,
		VerifiedAt: time.Now().UTC(),
		Valid:      true,
		Details:    make(map[string]interface{}),
	}
}

// PolicyVerifier verifies policy integrity
type PolicyVerifier struct {
	config Config
}

// NewPolicyVerifier creates a new PolicyVerifier instance
func NewPolicyVerifier(config Config) *PolicyVerifier {
	return &PolicyVerifier{config: config}
}

// Verify verifies the policy snapshot integrity
func (v *PolicyVerifier) Verify(b *bundle.Bundle) CheckResult {
	if b.Policy == nil {
		return CheckResult{
			Passed:  false,
			Message: "Bundle has no policy snapshot",
		}
	}

	// Compute spec hash using canonical JSON
	specCanonical, err := canonicaljson.Marshal(b.Policy.Spec)
	if err != nil {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("Failed to canonicalize policy spec: %v", err),
		}
	}

	hash := sha256.Sum256(specCanonical)
	computedHash := hex.EncodeToString(hash[:])

	if computedHash != b.Policy.SpecHash {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("Policy spec hash mismatch: expected %s, got %s",
				b.Policy.SpecHash, computedHash),
		}
	}

	if v.config.Verbose {
		fmt.Printf("  Policy: %s (version %s)\n", b.Policy.Name, b.Policy.Version)
		fmt.Printf("  Policy hash: %s\n", computedHash)
	}

	return CheckResult{
		Passed:  true,
		Message: fmt.Sprintf("Policy '%s' version '%s' integrity verified",
			b.Policy.Name, b.Policy.Version),
	}
}

// OutputJSON outputs results as JSON
func (r *VerificationResult) OutputJSON() error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}

// OutputText outputs results as human-readable text
func (r *VerificationResult) OutputText() error {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           AegisRun Evidence Bundle Verification              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Printf("Bundle: %s\n", r.BundlePath)
	fmt.Printf("Run ID: %s\n", r.RunID)
	fmt.Printf("Events: %d\n", r.EventCount)
	fmt.Println()

	fmt.Println("Verification Checks:")
	fmt.Println("────────────────────")

	// Chain integrity
	checkMark := "✓"
	if !r.ChainIntegrity.Passed {
		checkMark = "✗"
	}
	fmt.Printf("  %s Event Chain Integrity: %s\n", checkMark, r.ChainIntegrity.Message)

	// Signature
	checkMark = "✓"
	if !r.SignatureValid.Passed {
		checkMark = "✗"
	}
	fmt.Printf("  %s Signature Verification: %s\n", checkMark, r.SignatureValid.Message)

	// Policy integrity
	checkMark = "✓"
	if !r.PolicyIntegrity.Passed {
		checkMark = "✗"
	}
	fmt.Printf("  %s Policy Integrity: %s\n", checkMark, r.PolicyIntegrity.Message)

	fmt.Println()

	// Warnings
	if len(r.Warnings) > 0 {
		fmt.Println("Warnings:")
		for _, w := range r.Warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}
		fmt.Println()
	}

	// Errors
	if len(r.Errors) > 0 {
		fmt.Println("Errors:")
		for _, e := range r.Errors {
			fmt.Printf("  ✗ %s\n", e)
		}
		fmt.Println()
	}

	// Final verdict
	fmt.Println("────────────────────")
	if r.Valid {
		fmt.Println("✓ VERIFICATION PASSED")
		fmt.Println()
		fmt.Println("This evidence bundle is valid and has not been tampered with.")
	} else {
		fmt.Println("✗ VERIFICATION FAILED")
		fmt.Println()
		fmt.Println("This evidence bundle failed verification. Do not rely on its contents.")
	}
	fmt.Println()

	return nil
}
