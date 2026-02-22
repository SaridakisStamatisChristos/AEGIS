package integration

import (
	"testing"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Compiler tests
// ---------------------------------------------------------------------------

func TestPolicy_CompileValidSpec(t *testing.T) {
	compiler := policy.NewCompiler()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
			{Name: "exec", Action: contracts.ActionBlock},
		},
		Budgets: contracts.Budgets{
			MaxToolCalls: ptr(100),
		},
	}

	compiled, err := compiler.Compile(spec)
	require.NoError(t, err)
	require.NotNil(t, compiled)
	assert.Len(t, compiled.ToolPolicies, 2)
	assert.Contains(t, compiled.ToolPolicies, "http_request")
	assert.Contains(t, compiled.ToolPolicies, "exec")
}

func TestPolicy_CompileWithArgSchema(t *testing.T) {
	compiler := policy.NewCompiler()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{
				Name:   "http_request",
				Action: contracts.ActionAllow,
				ArgSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"url":    map[string]interface{}{"type": "string"},
						"method": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"url"},
				},
			},
		},
	}

	compiled, err := compiler.Compile(spec)
	require.NoError(t, err)
	require.NotNil(t, compiled.ToolPolicies["http_request"].ArgSchema)
}

func TestPolicy_CompileInvalidArgSchema(t *testing.T) {
	compiler := policy.NewCompiler()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{
				Name:   "broken",
				Action: contracts.ActionAllow,
				ArgSchema: map[string]interface{}{
					"type": "invalid_type_name",
				},
			},
		},
	}

	_, err := compiler.Compile(spec)
	assert.Error(t, err)
}

func TestPolicy_CompileWithConditions(t *testing.T) {
	compiler := policy.NewCompiler()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{
				Name:       "http_request",
				Action:     contracts.ActionAllow,
				Conditions: []string{`args.url != ""`},
			},
		},
	}

	compiled, err := compiler.Compile(spec)
	require.NoError(t, err)
	assert.Len(t, compiled.ToolPolicies["http_request"].Conditions, 1)
}

func TestPolicy_CompileInvalidCondition(t *testing.T) {
	compiler := policy.NewCompiler()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{
				Name:       "http_request",
				Action:     contracts.ActionAllow,
				Conditions: []string{"|||invalid|||"},
			},
		},
	}

	_, err := compiler.Compile(spec)
	assert.Error(t, err, "invalid CEL expression should fail compilation")
}

func TestPolicy_CompileEgressControls(t *testing.T) {
	compiler := policy.NewCompiler()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
		EgressControls: &contracts.EgressControls{
			DomainAllowlist: []string{"api.example.com", "*.trusted.io"},
			BlockPrivateIPs: true,
		},
	}

	compiled, err := compiler.Compile(spec)
	require.NoError(t, err)
	require.NotNil(t, compiled.EgressControls)
	assert.True(t, compiled.EgressControls.BlockPrivateIPs)
}

func TestPolicy_CompileRedactionPatterns(t *testing.T) {
	compiler := policy.NewCompiler()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
		Redaction: &contracts.RedactionConfig{
			Patterns:     []string{`sk-[a-zA-Z0-9]{48}`, `\b\d{3}-\d{2}-\d{4}\b`},
			MaskStrategy: contracts.MaskRedact,
		},
	}

	compiled, err := compiler.Compile(spec)
	require.NoError(t, err)
	assert.Len(t, compiled.RedactionPatterns, 2)
}

func TestPolicy_CompileInvalidRedactionPattern(t *testing.T) {
	compiler := policy.NewCompiler()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "tool1", Action: contracts.ActionAllow},
		},
		Redaction: &contracts.RedactionConfig{
			Patterns:     []string{`[invalid`},
			MaskStrategy: contracts.MaskRedact,
		},
	}

	_, err := compiler.Compile(spec)
	assert.Error(t, err, "invalid regex should fail compilation")
}

// ---------------------------------------------------------------------------
// Evaluator tests
// ---------------------------------------------------------------------------

func compileAndEval(t *testing.T, spec *contracts.PolicySpec, ctx *policy.EvaluationContext) *contracts.Decision {
	t.Helper()
	compiler := policy.NewCompiler()
	compiled, err := compiler.Compile(spec)
	require.NoError(t, err)

	evaluator := policy.NewEvaluator()
	decision, err := evaluator.Evaluate(compiled, ctx)
	require.NoError(t, err)
	return decision
}

func TestPolicy_DefaultDenyUndefinedTool(t *testing.T) {
	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "allowed_tool", Action: contracts.ActionAllow},
		},
	}

	d := compileAndEval(t, spec, &policy.EvaluationContext{
		ToolName: "undefined_tool",
		Args:     map[string]interface{}{},
		Counters: &contracts.RunCounters{},
	})

	assert.Equal(t, contracts.ActionBlock, d.Action)
	assert.Contains(t, d.Reason, "not defined in policy")
}

func TestPolicy_AllowDefinedTool(t *testing.T) {
	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "my_tool", Action: contracts.ActionAllow},
		},
	}

	d := compileAndEval(t, spec, &policy.EvaluationContext{
		ToolName: "my_tool",
		Args:     map[string]interface{}{},
		Counters: &contracts.RunCounters{},
	})

	assert.Equal(t, contracts.ActionAllow, d.Action)
}

func TestPolicy_ArgSchemaValidation(t *testing.T) {
	spec := &contracts.PolicySpec{
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
			},
		},
	}

	t.Run("valid_args", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{"url": "https://example.com"},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionAllow, d.Action)
	})

	t.Run("missing_required_arg", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
		assert.Contains(t, d.Reason, "url")
	})

	t.Run("wrong_type", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{"url": 12345},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
	})
}

func TestPolicy_BudgetEnforcement(t *testing.T) {
	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
		Budgets: contracts.Budgets{
			MaxToolCalls:     ptr(5),
			MaxRetries:       ptr(3),
			MaxBytesEgressed: ptr(1024),
		},
	}

	t.Run("within_budget", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{},
			Counters: &contracts.RunCounters{ToolCalls: 4, Retries: 2, BytesEgressed: 500},
		})
		assert.Equal(t, contracts.ActionAllow, d.Action)
	})

	t.Run("exceed_tool_calls", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{},
			Counters: &contracts.RunCounters{ToolCalls: 5},
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
		assert.Contains(t, d.PolicyRuleID, "budget")
	})

	t.Run("exceed_retries", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{},
			Counters: &contracts.RunCounters{Retries: 3},
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
	})

	t.Run("exceed_bytes_egressed", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{},
			Counters: &contracts.RunCounters{BytesEgressed: 1024},
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
	})
}

func TestPolicy_EgressDomainAllowlist(t *testing.T) {
	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
		EgressControls: &contracts.EgressControls{
			DomainAllowlist: []string{"api.example.com", "*.trusted.io"},
		},
	}

	t.Run("allowed_exact", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{"url": "https://api.example.com/v1"},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionAllow, d.Action)
	})

	t.Run("allowed_wildcard", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{"url": "https://sub.trusted.io/path"},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionAllow, d.Action)
	})

	t.Run("blocked_not_in_allowlist", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{"url": "https://evil.com/steal"},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
		assert.Contains(t, d.Reason, "not in allowlist")
	})
}

func TestPolicy_EgressDomainDenylist(t *testing.T) {
	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
		EgressControls: &contracts.EgressControls{
			DomainDenylist: []string{"evil.com", "*.malware.net"},
		},
	}

	t.Run("allowed_not_in_denylist", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{"url": "https://safe.example.com"},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionAllow, d.Action)
	})

	t.Run("blocked_exact_deny", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{"url": "https://evil.com/steal"},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
	})

	t.Run("blocked_wildcard_deny", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{"url": "https://sub.malware.net/dl"},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
	})
}

func TestPolicy_PrivateIPBlocking(t *testing.T) {
	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
		EgressControls: &contracts.EgressControls{
			BlockPrivateIPs: true,
		},
	}

	privateIPs := []string{
		"http://127.0.0.1/evil",
		"http://10.0.0.1/internal",
		"http://192.168.1.1/local",
		"http://172.16.0.1/internal",
		"http://169.254.169.254/latest/meta-data",
	}

	for _, ip := range privateIPs {
		t.Run(ip, func(t *testing.T) {
			d := compileAndEval(t, spec, &policy.EvaluationContext{
				ToolName: "http_request",
				Args:     map[string]interface{}{"url": ip},
				Counters: &contracts.RunCounters{},
			})
			assert.Equal(t, contracts.ActionBlock, d.Action, "should block private IP: %s", ip)
		})
	}
}

func TestPolicy_OutputValidation(t *testing.T) {
	compiler := policy.NewCompiler()
	evaluator := policy.NewEvaluator()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{
				Name:   "lookup",
				Action: contracts.ActionAllow,
				OutputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"result": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"result"},
				},
			},
		},
	}

	compiled, err := compiler.Compile(spec)
	require.NoError(t, err)

	t.Run("valid_output", func(t *testing.T) {
		tp := compiled.ToolPolicies["lookup"]
		err := evaluator.ValidateOutput(tp, map[string]interface{}{"result": "hello"})
		assert.NoError(t, err)
	})

	t.Run("invalid_output", func(t *testing.T) {
		tp := compiled.ToolPolicies["lookup"]
		err := evaluator.ValidateOutput(tp, map[string]interface{}{"wrongfield": 42})
		assert.Error(t, err)
	})
}

func TestPolicy_ArgSchemaStrictEdges(t *testing.T) {
	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{
				Name:   "http_request",
				Action: contracts.ActionAllow,
				ArgSchema: map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"url": map[string]interface{}{"type": "string", "format": "uri"},
						"headers": map[string]interface{}{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]interface{}{
								"Authorization": map[string]interface{}{"type": "string"},
							},
							"required": []interface{}{"Authorization"},
						},
						"retry": map[string]interface{}{
							"oneOf": []interface{}{
								map[string]interface{}{"type": "integer", "minimum": float64(0)},
								map[string]interface{}{"type": "string", "enum": []interface{}{"auto"}},
							},
						},
					},
					"required": []interface{}{"url", "headers"},
				},
			},
		},
	}

	t.Run("valid_nested_and_oneof_integer", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args: map[string]interface{}{
				"url": "https://api.example.com/v1",
				"headers": map[string]interface{}{
					"Authorization": "Bearer token",
				},
				"retry": float64(2),
			},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionAllow, d.Action)
	})

	t.Run("rejects_additional_top_level_property", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args: map[string]interface{}{
				"url": "https://api.example.com/v1",
				"headers": map[string]interface{}{
					"Authorization": "Bearer token",
				},
				"unexpected": "boom",
			},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
		assert.Contains(t, d.PolicyRuleID, "arg_schema")
		assert.Contains(t, d.Reason, "Additional property")
	})

	t.Run("rejects_missing_nested_required_field", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args: map[string]interface{}{
				"url":     "https://api.example.com/v1",
				"headers": map[string]interface{}{},
			},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
		assert.Contains(t, d.PolicyRuleID, "arg_schema")
		assert.Contains(t, d.Reason, "Authorization")
	})

	t.Run("rejects_oneof_mismatch", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args: map[string]interface{}{
				"url": "https://api.example.com/v1",
				"headers": map[string]interface{}{
					"Authorization": "Bearer token",
				},
				"retry": "manual",
			},
			Counters: &contracts.RunCounters{},
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
		assert.Contains(t, d.PolicyRuleID, "arg_schema")
	})
}

func TestPolicy_OutputSchemaStrictEdges(t *testing.T) {
	compiler := policy.NewCompiler()
	evaluator := policy.NewEvaluator()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{
				Name:   "lookup",
				Action: contracts.ActionAllow,
				OutputSchema: map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"status": map[string]interface{}{"type": "string", "enum": []interface{}{"ok", "error"}},
						"payload": map[string]interface{}{
							"oneOf": []interface{}{
								map[string]interface{}{
									"type":                 "object",
									"additionalProperties": false,
									"properties": map[string]interface{}{
										"value": map[string]interface{}{"type": "string"},
									},
									"required": []interface{}{"value"},
								},
								map[string]interface{}{
									"type":                 "object",
									"additionalProperties": false,
									"properties": map[string]interface{}{
										"code": map[string]interface{}{"type": "integer"},
									},
									"required": []interface{}{"code"},
								},
							},
						},
					},
					"required": []interface{}{"status", "payload"},
				},
			},
		},
	}

	compiled, err := compiler.Compile(spec)
	require.NoError(t, err)
	tp := compiled.ToolPolicies["lookup"]

	t.Run("accepts_valid_oneof_branch", func(t *testing.T) {
		err := evaluator.ValidateOutput(tp, map[string]interface{}{
			"status": "ok",
			"payload": map[string]interface{}{
				"value": "hello",
			},
		})
		assert.NoError(t, err)
	})

	t.Run("rejects_additional_output_property", func(t *testing.T) {
		err := evaluator.ValidateOutput(tp, map[string]interface{}{
			"status": "ok",
			"payload": map[string]interface{}{
				"value": "hello",
			},
			"extra": true,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Additional property")
	})

	t.Run("rejects_payload_oneof_mismatch", func(t *testing.T) {
		err := evaluator.ValidateOutput(tp, map[string]interface{}{
			"status": "ok",
			"payload": map[string]interface{}{
				"value": 123,
			},
		})
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Validator tests
// ---------------------------------------------------------------------------

func TestPolicy_ValidatorAcceptsValidSpec(t *testing.T) {
	v := policy.NewValidator()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
		Budgets: contracts.Budgets{
			MaxToolCalls: ptr(50),
		},
	}

	err := v.Validate(spec)
	assert.NoError(t, err)
}

func TestPolicy_ValidatorRejectsEmptyTools(t *testing.T) {
	v := policy.NewValidator()
	spec := &contracts.PolicySpec{Tools: []contracts.ToolPolicy{}}
	err := v.Validate(spec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one tool")
}

func TestPolicy_ValidatorRejectsDuplicateTools(t *testing.T) {
	v := policy.NewValidator()
	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "duplicate", Action: contracts.ActionAllow},
			{Name: "duplicate", Action: contracts.ActionBlock},
		},
	}
	err := v.Validate(spec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate tool name")
}

func TestPolicy_ValidatorRejectsNegativeBudgets(t *testing.T) {
	v := policy.NewValidator()

	t.Run("negative_tool_calls", func(t *testing.T) {
		spec := &contracts.PolicySpec{
			Tools:   []contracts.ToolPolicy{{Name: "t", Action: contracts.ActionAllow}},
			Budgets: contracts.Budgets{MaxToolCalls: ptr(-1)},
		}
		assert.Error(t, v.Validate(spec))
	})

	t.Run("negative_retries", func(t *testing.T) {
		spec := &contracts.PolicySpec{
			Tools:   []contracts.ToolPolicy{{Name: "t", Action: contracts.ActionAllow}},
			Budgets: contracts.Budgets{MaxRetries: ptr(-5)},
		}
		assert.Error(t, v.Validate(spec))
	})

	t.Run("negative_bytes", func(t *testing.T) {
		spec := &contracts.PolicySpec{
			Tools:   []contracts.ToolPolicy{{Name: "t", Action: contracts.ActionAllow}},
			Budgets: contracts.Budgets{MaxBytesEgressed: ptr(-1)},
		}
		assert.Error(t, v.Validate(spec))
	})
}

func TestPolicy_SpecHashDeterminism(t *testing.T) {
	v := policy.NewValidator()

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
			{Name: "read_file", Action: contracts.ActionBlock},
		},
		Budgets: contracts.Budgets{MaxToolCalls: ptr(10)},
	}

	hash1, err := v.ComputeSpecHash(spec)
	require.NoError(t, err)

	hash2, err := v.ComputeSpecHash(spec)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "same spec should always produce the same hash")
	assert.Len(t, hash1, 64, "SHA-256 hex should be 64 chars")
}

func TestPolicy_SpecHashChangesOnModification(t *testing.T) {
	v := policy.NewValidator()

	spec1 := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
	}

	spec2 := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionBlock},
		},
	}

	hash1, err := v.ComputeSpecHash(spec1)
	require.NoError(t, err)
	hash2, err := v.ComputeSpecHash(spec2)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "different specs should produce different hashes")
}

func TestPolicy_VersionValidation(t *testing.T) {
	v := policy.NewValidator()

	assert.NoError(t, v.ValidateVersion("v1"))
	assert.NoError(t, v.ValidateVersion("v42"))
	assert.Error(t, v.ValidateVersion("1"))
	assert.Error(t, v.ValidateVersion(""))
	assert.Error(t, v.ValidateVersion("x"))
}

// ---------------------------------------------------------------------------
// Evaluation order tests
// ---------------------------------------------------------------------------

func TestPolicy_EvaluationOrder(t *testing.T) {
	// The evaluator checks: 1. tool lookup, 2. schema, 3. conditions,
	// 4. budgets, 5. egress, 6. action.
	// Verify that earlier checks short-circuit.

	spec := &contracts.PolicySpec{
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
			},
		},
		Budgets: contracts.Budgets{
			MaxToolCalls: ptr(0), // already at limit
		},
		EgressControls: &contracts.EgressControls{
			BlockPrivateIPs: true,
		},
	}

	t.Run("schema_blocks_before_budget", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{}, // missing required 'url'
			Counters: &contracts.RunCounters{ToolCalls: 0},
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
		assert.Contains(t, d.PolicyRuleID, "arg_schema", "schema should block before budget check")
	})

	t.Run("budget_blocks_before_egress", func(t *testing.T) {
		d := compileAndEval(t, spec, &policy.EvaluationContext{
			ToolName: "http_request",
			Args:     map[string]interface{}{"url": "http://127.0.0.1"},
			Counters: &contracts.RunCounters{ToolCalls: 0}, // at budget limit
		})
		assert.Equal(t, contracts.ActionBlock, d.Action)
		assert.Contains(t, d.PolicyRuleID, "budget", "budget should block before egress check")
	})
}

// ---------------------------------------------------------------------------
// Multi-action tests
// ---------------------------------------------------------------------------

func TestPolicy_MultipleToolActions(t *testing.T) {
	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "safe_tool", Action: contracts.ActionAllow},
			{Name: "risky_tool", Action: contracts.ActionRequireApproval},
			{Name: "blocked_tool", Action: contracts.ActionBlock},
			{Name: "sensitive_tool", Action: contracts.ActionRedact},
			{Name: "warn_tool", Action: contracts.ActionWarn},
		},
	}

	tests := []struct {
		tool           string
		expectedAction contracts.PolicyAction
	}{
		{"safe_tool", contracts.ActionAllow},
		{"risky_tool", contracts.ActionRequireApproval},
		{"blocked_tool", contracts.ActionBlock},
		{"sensitive_tool", contracts.ActionRedact},
		{"warn_tool", contracts.ActionWarn},
		{"unknown", contracts.ActionBlock}, // default deny
	}

	for _, tc := range tests {
		t.Run(tc.tool, func(t *testing.T) {
			d := compileAndEval(t, spec, &policy.EvaluationContext{
				ToolName: tc.tool,
				Args:     map[string]interface{}{},
				Counters: &contracts.RunCounters{},
			})
			assert.Equal(t, tc.expectedAction, d.Action)
		})
	}
}
