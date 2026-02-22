// Package main provides the AegisRun evidence bundle verifier CLI
package main

import (
	"fmt"
	"os"

	"github.com/aegisrun/aegis-verify/internal/bundle"
	"github.com/aegisrun/aegis-verify/internal/verifier"
	"github.com/spf13/cobra"
)

var (
	version    = "1.0.0"
	verbose    bool
	jsonOutput bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "aegis-verify [bundle.zip]",
		Short: "Verify AegisRun evidence bundles",
		Long: `AegisRun Evidence Bundle Verifier

This tool verifies the integrity and authenticity of evidence bundles
exported from the AegisRun Agent Control Plane.

Verification includes:
  - Event chain integrity (hash chaining)
  - Policy immutability verification
  - Ed25519 signature validation
  - Tamper detection

Example:
  aegis-verify evidence-run123.zip
  aegis-verify --verbose evidence-run123.zip
  aegis-verify --json evidence-run123.zip`,
		Args: cobra.ExactArgs(1),
		RunE: runVerify,
	}

	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output results as JSON")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("aegis-verify version %s\n", version)
		},
	}

	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runVerify(cmd *cobra.Command, args []string) error {
	bundlePath := args[0]

	// Check file exists
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return fmt.Errorf("bundle file not found: %s", bundlePath)
	}

	// Load the bundle
	b, err := bundle.Load(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to load bundle: %w", err)
	}

	// Create verifiers
	config := verifier.Config{
		Verbose:    verbose,
		JSONOutput: jsonOutput,
	}

	chainVerifier := verifier.NewChainVerifier(config)
	sigVerifier := verifier.NewSignatureVerifier(config)
	policyVerifier := verifier.NewPolicyVerifier(config)

	// Run verification
	result := verifier.NewResult(bundlePath, b.Manifest.RunID, len(b.Events))

	if verbose {
		fmt.Printf("Verifying bundle for run: %s\n", b.Manifest.RunID)
		fmt.Printf("Event count: %d\n", len(b.Events))
	}

	// 1. Verify event chain integrity
	chainResult := chainVerifier.Verify(b.Events)
	result.ChainIntegrity = chainResult
	if !chainResult.Passed {
		result.Valid = false
		result.Errors = append(result.Errors, chainResult.Message)
	}

	// 2. Verify signature
	sigResult := sigVerifier.Verify(b)
	result.SignatureValid = sigResult
	if !sigResult.Passed {
		result.Valid = false
		result.Errors = append(result.Errors, sigResult.Message)
	}

	// 3. Verify policy integrity
	policyResult := policyVerifier.Verify(b)
	result.PolicyIntegrity = policyResult
	if !policyResult.Passed {
		result.Valid = false
		result.Errors = append(result.Errors, policyResult.Message)
	}

	// 4. Additional checks
	if b.Run != nil {
		if b.Run.Status != "completed" && b.Run.Status != "blocked" {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Run status is '%s' (not completed or blocked)", b.Run.Status))
		}
	}

	// Store details
	result.Details["manifest_version"] = b.Manifest.BundleVersion
	result.Details["exported_at"] = b.Manifest.ExportedAt
	result.Details["signer_key_id"] = b.Manifest.SignerKeyID

	// Output results
	if jsonOutput {
		return result.OutputJSON()
	}

	return result.OutputText()
}
