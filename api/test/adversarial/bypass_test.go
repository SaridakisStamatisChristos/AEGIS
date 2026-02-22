package adversarial

import (
	"strings"
	"testing"

	"github.com/aegisrun/aegisrun/internal/auth"
	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: build a compiled policy that only allows "http_request"
func allowOnlyHTTPRequest(t *testing.T) *policy.CompiledPolicy {
	t.Helper()
	compiler := policy.NewCompiler()
	cp, err := compiler.Compile(&contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
	})
	require.NoError(t, err)
	return cp
}

// TestBypass_PolicyBypass tests that calling an undefined tool is blocked by
// the default-deny policy even when another tool is explicitly allowed.
func TestBypass_PolicyBypass(t *testing.T) {
	cp := allowOnlyHTTPRequest(t)
	evaluator := policy.NewEvaluator()

	decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "shell_exec",
		Args:     map[string]interface{}{"cmd": "ls"},
		Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, decision.Action)
	assert.Contains(t, decision.Reason, "not defined in policy")
}

// TestBypass_UnicodeConfusables tests that Unicode confusable tool names do
// not match the allowlisted ASCII tool name.
func TestBypass_UnicodeConfusables(t *testing.T) {
	cp := allowOnlyHTTPRequest(t)
	evaluator := policy.NewEvaluator()

	confusables := []struct {
		label      string
		confusable string
	}{
		{"cyrillic_p", "htt\u0440_request"}, // Cyrillic 'р'
		{"cyrillic_e", "sh\u0435ll_exec"},   // Cyrillic 'е'
		{"cyrillic_i", "f\u0456le_read"},    // Cyrillic 'і'
		{"cyrillic_a", "\u0430dmin"},        // Cyrillic 'а'
	}

	for _, tc := range confusables {
		t.Run(tc.label, func(t *testing.T) {
			decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
				ToolName: tc.confusable,
				Args:     map[string]interface{}{},
				Counters: &contracts.RunCounters{},
			})
			require.NoError(t, err)
			assert.Equal(t, contracts.ActionBlock, decision.Action,
				"Unicode confusable %q should NOT match the allowlisted tool", tc.confusable)
		})
	}
}

// TestBypass_CaseVariations tests that case variations of tool names are
// rejected by the default-deny policy (exact match).
func TestBypass_CaseVariations(t *testing.T) {
	cp := allowOnlyHTTPRequest(t)
	evaluator := policy.NewEvaluator()

	caseVariations := []string{
		"HTTP_REQUEST",
		"Http_Request",
		"hTtP_rEqUeSt",
	}

	for _, variation := range caseVariations {
		t.Run(variation, func(t *testing.T) {
			decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
				ToolName: variation,
				Args:     map[string]interface{}{},
				Counters: &contracts.RunCounters{},
			})
			require.NoError(t, err)
			assert.Equal(t, contracts.ActionBlock, decision.Action,
				"Case variation %q should NOT match 'http_request'", variation)
		})
	}
}

// TestBypass_EncodingBypass tests that URL-encoded or otherwise encoded tool
// names are not matched to the allowlisted tool.
func TestBypass_EncodingBypass(t *testing.T) {
	cp := allowOnlyHTTPRequest(t)
	evaluator := policy.NewEvaluator()

	encodings := []struct {
		name    string
		encoded string
	}{
		{"url_encoded", "http%5Frequest"},
		{"double_url", "http%255Frequest"},
		{"unicode_escape", "http\\u005Frequest"},
		{"html_entity", "http&#95;request"},
	}

	for _, tc := range encodings {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
				ToolName: tc.encoded,
				Args:     map[string]interface{}{},
				Counters: &contracts.RunCounters{},
			})
			require.NoError(t, err)
			assert.Equal(t, contracts.ActionBlock, decision.Action,
				"Encoded tool name %q should be blocked", tc.encoded)
		})
	}
}

// TestBypass_NullByteInjection tests that tool names containing null bytes
// are not matched to the allowed tool.
func TestBypass_NullByteInjection(t *testing.T) {
	cp := allowOnlyHTTPRequest(t)
	evaluator := policy.NewEvaluator()

	nullPayloads := []string{
		"http_request\x00_blocked",
		"allowed_tool\x00.exe",
		"safe\x00../../../etc/passwd",
	}

	for i, payload := range nullPayloads {
		t.Run("null_byte_"+string(rune('A'+i)), func(t *testing.T) {
			decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
				ToolName: payload,
				Args:     map[string]interface{}{},
				Counters: &contracts.RunCounters{},
			})
			require.NoError(t, err)
			assert.Equal(t, contracts.ActionBlock, decision.Action,
				"Null-byte payload should be blocked")
		})
	}
}

// TestBypass_RaceConditions tests that concurrent evaluations with the same
// compiled policy do not interfere with each other.
func TestBypass_RaceConditions(t *testing.T) {
	cp := allowOnlyHTTPRequest(t)
	evaluator := policy.NewEvaluator()

	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func() {
			d, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
				ToolName: "http_request",
				Args:     map[string]interface{}{},
				Counters: &contracts.RunCounters{},
			})
			assert.NoError(t, err)
			assert.Equal(t, contracts.ActionAllow, d.Action)
			done <- true
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}

// TestBypass_AuthenticationBypass tests that the RBAC system rejects invalid roles.
func TestBypass_AuthenticationBypass(t *testing.T) {
	rbac := auth.NewRBAC()

	authBypasses := []struct {
		name string
		role string
	}{
		{"empty_string", ""},
		{"null_literal", "null"},
		{"bearer_header", "Bearer dGVzdA=="},
		{"jwt_none_alg", "eyJhbGciOiJub25lIn0"},
		{"random_string", "not_a_real_role"},
	}

	for _, tc := range authBypasses {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rbac.ValidateRole(tc.role)
			assert.Error(t, err, "role %q should not be valid", tc.role)
			assert.False(t, rbac.HasPermission(auth.Role(tc.role), auth.PermRunView))
			assert.False(t, rbac.HasPermission(auth.Role(tc.role), auth.PermPolicyDeploy))
		})
	}
}

// TestBypass_RBACEscalation tests that lower-privileged roles lack elevated permissions.
func TestBypass_RBACEscalation(t *testing.T) {
	rbac := auth.NewRBAC()

	assert.False(t, rbac.HasPermission(auth.RoleViewer, auth.PermRunCreate))
	assert.False(t, rbac.HasPermission(auth.RoleDeveloper, auth.PermPolicyApprove))
	assert.False(t, rbac.HasPermission(auth.RolePolicyAdmin, auth.PermPolicyDeploy))
	assert.False(t, rbac.HasPermission(auth.RoleApprover, auth.PermPolicyEdit))
	assert.False(t, rbac.HasPermission(auth.RoleViewer, auth.PermUserManage))
	assert.False(t, rbac.HasPermission(auth.RoleDeveloper, auth.PermUserManage))
	assert.True(t, rbac.HasPermission(auth.RoleOrgAdmin, auth.PermUserManage))
}

// TestBypass_PolicyManipulation tests that empty, whitespace-padded, or
// colliding tool names cannot subvert policy.
func TestBypass_PolicyManipulation(t *testing.T) {
	compiler := policy.NewCompiler()
	cp, err := compiler.Compile(&contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
			{Name: "shell_exec", Action: contracts.ActionBlock},
		},
	})
	require.NoError(t, err)
	evaluator := policy.NewEvaluator()

	// shell_exec should be blocked
	d, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "shell_exec", Args: map[string]interface{}{}, Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, d.Action)

	// Empty tool name → not in policy → blocked
	d, err = evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "", Args: map[string]interface{}{}, Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, d.Action)

	// Trailing space → not in policy → blocked
	d, err = evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "http_request ", Args: map[string]interface{}{}, Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, d.Action)

	// Leading spaces → not in policy → blocked
	d, err = evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: strings.Repeat(" ", 3) + "http_request",
		Args:     map[string]interface{}{}, Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, d.Action)
}
