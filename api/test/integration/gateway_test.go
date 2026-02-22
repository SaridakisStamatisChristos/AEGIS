package integration

import (
	"testing"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// TestGateway_ExecuteToolCall_Allowed tests that allowed tool calls pass evaluation.
func TestGateway_ExecuteToolCall_Allowed(t *testing.T) {
	compiler := policy.NewCompiler()
	cp, err := compiler.Compile(&contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
			{Name: "db_query", Action: contracts.ActionAllow},
		},
	})
	require.NoError(t, err)

	evaluator := policy.NewEvaluator()

	allowed := []string{"http_request", "db_query"}
	for _, toolName := range allowed {
		decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
			ToolName: toolName,
			Args:     map[string]interface{}{"query": "SELECT 1"},
			Counters: &contracts.RunCounters{},
		})
		require.NoError(t, err)
		assert.Equal(t, contracts.ActionAllow, decision.Action)
	}
}

// TestGateway_ExecuteToolCall_Blocked tests that blocked tool calls are rejected.
func TestGateway_ExecuteToolCall_Blocked(t *testing.T) {
	compiler := policy.NewCompiler()
	cp, err := compiler.Compile(&contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
			{Name: "shell_exec", Action: contracts.ActionBlock},
		},
	})
	require.NoError(t, err)

	evaluator := policy.NewEvaluator()

	// Explicitly blocked tool
	decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "shell_exec",
		Args:     map[string]interface{}{},
		Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, decision.Action)

	// Undefined tool → default deny
	decision, err = evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "unknown_tool",
		Args:     map[string]interface{}{},
		Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, decision.Action)
	assert.Contains(t, decision.Reason, "not defined in policy")
}

// TestGateway_ExecuteToolCall_Redacted tests that tools with redact action
// produce the correct decision.
func TestGateway_ExecuteToolCall_Redacted(t *testing.T) {
	compiler := policy.NewCompiler()
	cp, err := compiler.Compile(&contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionRedact},
		},
	})
	require.NoError(t, err)

	evaluator := policy.NewEvaluator()
	decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "http_request",
		Args:     map[string]interface{}{"url": "http://example.com"},
		Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionRedact, decision.Action)
}

// TestGateway_ExecuteToolCall_RequireApproval tests approval workflow decision.
func TestGateway_ExecuteToolCall_RequireApproval(t *testing.T) {
	compiler := policy.NewCompiler()
	cp, err := compiler.Compile(&contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "deploy", Action: contracts.ActionRequireApproval},
		},
	})
	require.NoError(t, err)

	evaluator := policy.NewEvaluator()
	decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "deploy",
		Args:     map[string]interface{}{},
		Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionRequireApproval, decision.Action)
}

// TestGateway_BudgetEnforcement tests budget limits are enforced.
func TestGateway_BudgetEnforcement(t *testing.T) {
	compiler := policy.NewCompiler()

	maxCalls := 5
	maxRetries := 3
	maxBytes := 1000
	cp, err := compiler.Compile(&contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
		Budgets: contracts.Budgets{
			MaxToolCalls:     &maxCalls,
			MaxRetries:       &maxRetries,
			MaxBytesEgressed: &maxBytes,
		},
	})
	require.NoError(t, err)
	evaluator := policy.NewEvaluator()

	// Under all budgets → allowed
	decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "http_request",
		Args:     map[string]interface{}{},
		Counters: &contracts.RunCounters{ToolCalls: 3, Retries: 1, BytesEgressed: 500},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionAllow, decision.Action)

	// Exceed tool call budget
	decision, err = evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "http_request",
		Args:     map[string]interface{}{},
		Counters: &contracts.RunCounters{ToolCalls: 5},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, decision.Action)
	assert.Contains(t, decision.Reason, "max tool calls")

	// Exceed retry budget
	decision, err = evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "http_request",
		Args:     map[string]interface{}{},
		Counters: &contracts.RunCounters{Retries: 3},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, decision.Action)
	assert.Contains(t, decision.Reason, "max retries")

	// Exceed bytes budget
	decision, err = evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "http_request",
		Args:     map[string]interface{}{},
		Counters: &contracts.RunCounters{BytesEgressed: 1000},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, decision.Action)
	assert.Contains(t, decision.Reason, "max bytes egressed")
}

// TestGateway_SSRFPrevention tests SSRF protection via egress controls.
func TestGateway_SSRFPrevention(t *testing.T) {
	compiler := policy.NewCompiler()
	cp, err := compiler.Compile(&contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
		EgressControls: &contracts.EgressControls{
			BlockPrivateIPs: true,
		},
	})
	require.NoError(t, err)

	evaluator := policy.NewEvaluator()

	blockedURLs := []struct {
		label string
		url   string
	}{
		{"aws_metadata", "http://169.254.169.254/latest/meta-data/"},
		{"loopback", "http://127.0.0.1:8080/admin"},
		{"class_a", "http://10.0.0.1/internal"},
		{"class_c", "http://192.168.1.1/"},
		{"class_b", "http://172.16.0.1/"},
	}

	for _, tc := range blockedURLs {
		t.Run(tc.label, func(t *testing.T) {
			decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
				ToolName: "http_request",
				Args:     map[string]interface{}{"url": tc.url},
				Counters: &contracts.RunCounters{},
			})
			require.NoError(t, err)
			assert.Equal(t, contracts.ActionBlock, decision.Action,
				"URL %q should be blocked as private IP", tc.url)
		})
	}

	// Public URL should be allowed
	decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "http_request",
		Args:     map[string]interface{}{"url": "https://api.github.com/repos"},
		Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionAllow, decision.Action)
}

// TestGateway_ConcurrentCalls tests that concurrent evaluations are safe.
func TestGateway_ConcurrentCalls(t *testing.T) {
	compiler := policy.NewCompiler()
	cp, err := compiler.Compile(&contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
	})
	require.NoError(t, err)

	evaluator := policy.NewEvaluator()

	done := make(chan struct{}, 100)
	for i := 0; i < 100; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			d, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
				ToolName: "http_request",
				Args:     map[string]interface{}{"url": "http://example.com"},
				Counters: &contracts.RunCounters{},
			})
			assert.NoError(t, err)
			assert.Equal(t, contracts.ActionAllow, d.Action)
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}

// TestGateway_PolicyEvaluation tests evaluation order: tool lookup → schema →
// conditions → budgets → egress → action.
func TestGateway_PolicyEvaluation(t *testing.T) {
	compiler := policy.NewCompiler()
	maxCalls := 100
	cp, err := compiler.Compile(&contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{
				Name:   "http_request",
				Action: contracts.ActionAllow,
				ArgSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"url": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"url"},
				},
				Conditions: []string{`args.url != "forbidden"`},
			},
		},
		Budgets: contracts.Budgets{MaxToolCalls: &maxCalls},
		EgressControls: &contracts.EgressControls{
			BlockPrivateIPs: true,
		},
	})
	require.NoError(t, err)

	evaluator := policy.NewEvaluator()

	// 1. Schema violation → blocked
	decision, err := evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "http_request",
		Args:     map[string]interface{}{}, // missing required 'url'
		Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, decision.Action)
	assert.Contains(t, decision.PolicyRuleID, "arg_schema")

	// 2. Condition failure → blocked
	decision, err = evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "http_request",
		Args:     map[string]interface{}{"url": "forbidden"},
		Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, decision.Action)
	assert.Contains(t, decision.PolicyRuleID, "condition")

	// 3. Egress violation → blocked
	decision, err = evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "http_request",
		Args:     map[string]interface{}{"url": "http://127.0.0.1/admin"},
		Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionBlock, decision.Action)
	assert.Contains(t, decision.PolicyRuleID, "egress")

	// 4. All checks pass → allowed
	decision, err = evaluator.Evaluate(cp, &policy.EvaluationContext{
		ToolName: "http_request",
		Args:     map[string]interface{}{"url": "https://api.github.com"},
		Counters: &contracts.RunCounters{},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.ActionAllow, decision.Action)
}
