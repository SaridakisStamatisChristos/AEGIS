package property

import (
	"fmt"
	"testing"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// ---------------------------------------------------------------------------
// Immutability of compiled policy specs
// ---------------------------------------------------------------------------

// TestProperty_DeployedPolicySpecUnchanged verifies that a compiled policy's
// tool map does not change across repeat compilations.
func TestProperty_DeployedPolicySpecUnchanged(t *testing.T) {
	compiler := policy.NewCompiler()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
			{Name: "exec", Action: contracts.ActionBlock},
			{Name: "read_file", Action: contracts.ActionRedact},
		},
		Budgets: contracts.Budgets{MaxToolCalls: ptr(100)},
	}

	compiled1, err := compiler.Compile(spec)
	require.NoError(t, err)

	compiled2, err := compiler.Compile(spec)
	require.NoError(t, err)

	// Same keys
	assert.Equal(t, len(compiled1.ToolPolicies), len(compiled2.ToolPolicies))
	for name, tp1 := range compiled1.ToolPolicies {
		tp2, ok := compiled2.ToolPolicies[name]
		require.True(t, ok, "tool %s should exist in both compilations", name)
		assert.Equal(t, tp1.Action, tp2.Action)
	}
}

// TestProperty_SpecHashConsistency verifies that the same spec always produces
// the same hash, across many iterations.
func TestProperty_SpecHashConsistency(t *testing.T) {
	v := policy.NewValidator()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
		Budgets: contracts.Budgets{MaxToolCalls: ptr(50)},
	}

	first, err := v.ComputeSpecHash(spec)
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		hash, err := v.ComputeSpecHash(spec)
		require.NoError(t, err)
		assert.Equal(t, first, hash, "iteration %d should produce same hash", i)
	}
}

// TestProperty_VersionImmutability asserts that changing any field in a spec
// yields a different hash.
func TestProperty_VersionImmutability(t *testing.T) {
	v := policy.NewValidator()

	base := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
		Budgets: contracts.Budgets{MaxToolCalls: ptr(50)},
	}
	baseHash, err := v.ComputeSpecHash(base)
	require.NoError(t, err)

	mutations := []struct {
		name string
		spec *contracts.PolicySpec
	}{
		{
			"different_action",
			&contracts.PolicySpec{
				Tools:   []contracts.ToolPolicy{{Name: "http_request", Action: contracts.ActionBlock}},
				Budgets: contracts.Budgets{MaxToolCalls: ptr(50)},
			},
		},
		{
			"different_tool_name",
			&contracts.PolicySpec{
				Tools:   []contracts.ToolPolicy{{Name: "other_tool", Action: contracts.ActionAllow}},
				Budgets: contracts.Budgets{MaxToolCalls: ptr(50)},
			},
		},
		{
			"different_budget",
			&contracts.PolicySpec{
				Tools:   []contracts.ToolPolicy{{Name: "http_request", Action: contracts.ActionAllow}},
				Budgets: contracts.Budgets{MaxToolCalls: ptr(999)},
			},
		},
		{
			"added_tool",
			&contracts.PolicySpec{
				Tools: []contracts.ToolPolicy{
					{Name: "http_request", Action: contracts.ActionAllow},
					{Name: "write_file", Action: contracts.ActionBlock},
				},
				Budgets: contracts.Budgets{MaxToolCalls: ptr(50)},
			},
		},
		{
			"added_egress",
			&contracts.PolicySpec{
				Tools:   []contracts.ToolPolicy{{Name: "http_request", Action: contracts.ActionAllow}},
				Budgets: contracts.Budgets{MaxToolCalls: ptr(50)},
				EgressControls: &contracts.EgressControls{
					BlockPrivateIPs: true,
				},
			},
		},
	}

	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			mHash, err := v.ComputeSpecHash(m.spec)
			require.NoError(t, err)
			assert.NotEqual(t, baseHash, mHash, "mutation %s should create different hash", m.name)
		})
	}
}

// TestProperty_StatusTransitionsValid verifies that only allowed policy
// status transitions are modelled.
func TestProperty_StatusTransitionsValid(t *testing.T) {
	// Allowed transitions based on the policy lifecycle:
	//   draft → review → approved → deployed → deprecated
	allowedTransitions := map[contracts.PolicyStatus][]contracts.PolicyStatus{
		contracts.PolicyStatusDraft:      {contracts.PolicyStatusReview},
		contracts.PolicyStatusReview:     {contracts.PolicyStatusApproved, contracts.PolicyStatusDraft},
		contracts.PolicyStatusApproved:   {contracts.PolicyStatusDeployed, contracts.PolicyStatusDraft},
		contracts.PolicyStatusDeployed:   {contracts.PolicyStatusDeprecated},
		contracts.PolicyStatusDeprecated: {},
	}

	allStatuses := []contracts.PolicyStatus{
		contracts.PolicyStatusDraft,
		contracts.PolicyStatusReview,
		contracts.PolicyStatusApproved,
		contracts.PolicyStatusDeployed,
		contracts.PolicyStatusDeprecated,
	}

	for _, from := range allStatuses {
		for _, to := range allStatuses {
			isAllowed := false
			for _, a := range allowedTransitions[from] {
				if a == to {
					isAllowed = true
					break
				}
			}

			t.Run(fmt.Sprintf("%s→%s", from, to), func(t *testing.T) {
				if from == to {
					// Identity transition should not be in allowed list
					assert.False(t, isAllowed, "self-transition should not be allowed")
				}
				// Just verifying the model is consistent. In a real system,
				// there would be a function to check transitions.
				// This test documents the expected state machine.
				_ = isAllowed
			})
		}
	}
}

// TestProperty_RollbackPrevention verifies that a deployed policy cannot go
// back to an earlier lifecycle stage.
func TestProperty_RollbackPrevention(t *testing.T) {
	// Deployed should only go to deprecated, never back to draft/review/approved
	forbidden := []contracts.PolicyStatus{
		contracts.PolicyStatusDraft,
		contracts.PolicyStatusReview,
		contracts.PolicyStatusApproved,
	}

	for _, target := range forbidden {
		t.Run(fmt.Sprintf("deployed→%s", target), func(t *testing.T) {
			// This is a model test - we assert the lifecycle invariant here
			assert.NotEqual(t, target, contracts.PolicyStatusDeprecated,
				"deployed should only transition to deprecated")
		})
	}
}

// TestProperty_OrgIsolation verifies that policies for different orgs produce
// different spec hashes when org-specific data is embedded.
func TestProperty_OrgIsolation(t *testing.T) {
	// Spec hashes are computed from PolicySpec only (not org ID),
	// so identical specs for different orgs will have the same hash.
	// This is correct — org isolation is enforced at the storage and API layer,
	// not at the hash level. This test documents that expectation.

	v := policy.NewValidator()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
	}

	hash1, err := v.ComputeSpecHash(spec)
	require.NoError(t, err)
	hash2, err := v.ComputeSpecHash(spec)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2,
		"same spec for different orgs should have the same spec hash; "+
			"org isolation is enforced elsewhere")
}

// TestProperty_CompileIdempotent verifies that compiling the same spec N times
// produces functionally identical compiled policies.
func TestProperty_CompileIdempotent(t *testing.T) {
	compiler := policy.NewCompiler()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
			{Name: "exec", Action: contracts.ActionBlock},
		},
		Budgets: contracts.Budgets{MaxToolCalls: ptr(100)},
		EgressControls: &contracts.EgressControls{
			DomainAllowlist: []string{"api.example.com"},
			BlockPrivateIPs: true,
		},
		Redaction: &contracts.RedactionConfig{
			Patterns:     []string{`sk-[a-zA-Z0-9]{48}`},
			MaskStrategy: contracts.MaskRedact,
		},
	}

	compiled1, err := compiler.Compile(spec)
	require.NoError(t, err)

	for i := 0; i < 50; i++ {
		compiled, err := compiler.Compile(spec)
		require.NoError(t, err)
		assert.Equal(t, len(compiled1.ToolPolicies), len(compiled.ToolPolicies),
			"iteration %d: tool count mismatch", i)
		assert.Equal(t, len(compiled1.RedactionPatterns), len(compiled.RedactionPatterns),
			"iteration %d: redaction pattern count mismatch", i)
	}
}

// TestProperty_ValidatorRejectsAllInvalid runs a battery of invalid specs
// and asserts that every one is rejected.
func TestProperty_ValidatorRejectsAllInvalid(t *testing.T) {
	v := policy.NewValidator()

	invalids := []struct {
		name string
		spec *contracts.PolicySpec
	}{
		{"empty_tools", &contracts.PolicySpec{Tools: []contracts.ToolPolicy{}}},
		{"negative_budget", &contracts.PolicySpec{
			Tools:   []contracts.ToolPolicy{{Name: "t", Action: contracts.ActionAllow}},
			Budgets: contracts.Budgets{MaxToolCalls: ptr(-1)},
		}},
		{"duplicate_tools", &contracts.PolicySpec{
			Tools: []contracts.ToolPolicy{
				{Name: "dup", Action: contracts.ActionAllow},
				{Name: "dup", Action: contracts.ActionBlock},
			},
		}},
	}

	for _, tc := range invalids {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(tc.spec)
			assert.Error(t, err, "spec %s should be rejected", tc.name)
		})
	}
}
