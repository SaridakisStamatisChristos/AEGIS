package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/gibson042/canonicaljson-go"
)

// Validator validates policy specs and computes spec hashes
type Validator struct {
	compiler *Compiler
}

func NewValidator() *Validator {
	return &Validator{
		compiler: NewCompiler(),
	}
}

// Validate checks if a policy spec is valid
func (v *Validator) Validate(spec *contracts.PolicySpec) error {
	// Try to compile - this validates structure
	_, err := v.compiler.Compile(spec)
	if err != nil {
		return fmt.Errorf("policy validation failed: %w", err)
	}

	// Additional semantic checks
	if len(spec.Tools) == 0 {
		return fmt.Errorf("policy must define at least one tool")
	}

	// Check for duplicate tool names
	seen := make(map[string]bool)
	for _, tp := range spec.Tools {
		if seen[tp.Name] {
			return fmt.Errorf("duplicate tool name: %s", tp.Name)
		}
		seen[tp.Name] = true
	}

	// Validate budgets are non-negative
	if spec.Budgets.MaxToolCalls != nil && *spec.Budgets.MaxToolCalls < 0 {
		return fmt.Errorf("max_tool_calls must be non-negative")
	}
	if spec.Budgets.MaxWallClockSec != nil && *spec.Budgets.MaxWallClockSec < 0 {
		return fmt.Errorf("max_wall_clock_sec must be non-negative")
	}
	if spec.Budgets.MaxRetries != nil && *spec.Budgets.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be non-negative")
	}
	if spec.Budgets.MaxBytesEgressed != nil && *spec.Budgets.MaxBytesEgressed < 0 {
		return fmt.Errorf("max_bytes_egressed must be non-negative")
	}

	return nil
}

// ComputeSpecHash computes SHA256 hash of canonical JSON spec
func (v *Validator) ComputeSpecHash(spec *contracts.PolicySpec) (string, error) {
	canonical, err := canonicaljson.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("canonical JSON marshal: %w", err)
	}

	hash := sha256.Sum256(canonical)
	return hex.EncodeToString(hash[:]), nil
}

// ValidateVersion checks if version string is valid
func (v *Validator) ValidateVersion(version string) error {
	if len(version) < 2 || version[0] != 'v' {
		return fmt.Errorf("version must start with 'v' (e.g., v1)")
	}
	return nil
}
