package policy

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/xeipuuv/gojsonschema"
)

// Evaluator evaluates policies at runtime
type Evaluator struct{}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

type EvaluationContext struct {
	ToolName    string
	Args        map[string]interface{}
	StateVector map[string]interface{}
	Metadata    map[string]interface{}
	Counters    *contracts.RunCounters
	RunMetadata map[string]interface{}
}

// Evaluate determines the policy action for a tool call
func (e *Evaluator) Evaluate(policy *CompiledPolicy, ctx *EvaluationContext) (*contracts.Decision, error) {
	// 1. Check if tool is defined in policy
	toolPolicy, exists := policy.ToolPolicies[ctx.ToolName]
	if !exists {
		// Default deny for undefined tools
		return &contracts.Decision{
			Action:       contracts.ActionBlock,
			PolicyRuleID: "default.undefined_tool",
			Reason:       fmt.Sprintf("Tool '%s' not defined in policy", ctx.ToolName),
		}, nil
	}

	// 2. Validate args against schema
	if toolPolicy.ArgSchema != nil {
		result, err := toolPolicy.ArgSchema.Validate(gojsonschema.NewGoLoader(ctx.Args))
		if err != nil {
			return nil, fmt.Errorf("schema validation error: %w", err)
		}
		if !result.Valid() {
			errors := make([]string, len(result.Errors()))
			for i, e := range result.Errors() {
				errors[i] = e.String()
			}
			return &contracts.Decision{
				Action:       contracts.ActionBlock,
				PolicyRuleID: fmt.Sprintf("tool.%s.arg_schema", ctx.ToolName),
				Reason:       fmt.Sprintf("Argument validation failed: %s", strings.Join(errors, "; ")),
			}, nil
		}
	}

	// 3. Evaluate conditions
	if len(toolPolicy.Conditions) > 0 {
		conditionCtx := e.buildConditionContext(ctx)
		for i, cond := range toolPolicy.Conditions {
			passed, err := cond.Evaluator(conditionCtx)
			if err != nil {
				return nil, fmt.Errorf("condition %d evaluation error: %w", i, err)
			}
			if !passed {
				return &contracts.Decision{
					Action:       contracts.ActionBlock,
					PolicyRuleID: fmt.Sprintf("tool.%s.condition.%d", ctx.ToolName, i),
					Reason:       fmt.Sprintf("Condition failed: %s", cond.Expression),
				}, nil
			}
		}
	}

	// 4. Check budgets
	if budgetViolation := e.checkBudgets(policy.Budgets, ctx.Counters); budgetViolation != nil {
		return budgetViolation, nil
	}

	// 5. Check egress controls (for tools that make network requests)
	if policy.EgressControls != nil {
		if egressViolation := e.checkEgressControls(policy.EgressControls, ctx.Args); egressViolation != nil {
			return egressViolation, nil
		}
	}

	// 6. Return the tool's configured action
	return &contracts.Decision{
		Action:       toolPolicy.Action,
		PolicyRuleID: fmt.Sprintf("tool.%s", ctx.ToolName),
		Reason:       fmt.Sprintf("Policy action: %s", toolPolicy.Action),
	}, nil
}

func (e *Evaluator) buildConditionContext(ctx *EvaluationContext) map[string]interface{} {
	return map[string]interface{}{
		"tool_name":    ctx.ToolName,
		"args":         ctx.Args,
		"state":        ctx.StateVector,
		"metadata":     ctx.Metadata,
		"counters":     ctx.Counters,
		"run_metadata": ctx.RunMetadata,
	}
}

func (e *Evaluator) checkBudgets(budgets contracts.Budgets, counters *contracts.RunCounters) *contracts.Decision {
	if budgets.MaxToolCalls != nil && counters.ToolCalls >= *budgets.MaxToolCalls {
		return &contracts.Decision{
			Action:       contracts.ActionBlock,
			PolicyRuleID: "budget.max_tool_calls",
			Reason:       fmt.Sprintf("Exceeded max tool calls: %d", *budgets.MaxToolCalls),
		}
	}

	if budgets.MaxRetries != nil && counters.Retries >= *budgets.MaxRetries {
		return &contracts.Decision{
			Action:       contracts.ActionBlock,
			PolicyRuleID: "budget.max_retries",
			Reason:       fmt.Sprintf("Exceeded max retries: %d", *budgets.MaxRetries),
		}
	}

	if budgets.MaxBytesEgressed != nil && counters.BytesEgressed >= *budgets.MaxBytesEgressed {
		return &contracts.Decision{
			Action:       contracts.ActionBlock,
			PolicyRuleID: "budget.max_bytes_egressed",
			Reason:       fmt.Sprintf("Exceeded max bytes egressed: %d", *budgets.MaxBytesEgressed),
		}
	}

	// Check wall clock budget (if start time in metadata)
	if budgets.MaxWallClockSec != nil {
		// Implementation note: This would require start_time in metadata
		// For now, we skip this check in the evaluator
	}

	return nil
}

func (e *Evaluator) checkEgressControls(ec *contracts.EgressControls, args map[string]interface{}) *contracts.Decision {
	// Extract URL from args (common patterns: "url", "endpoint", "target")
	urlFields := []string{"url", "endpoint", "target", "destination", "host"}
	var targetURL string

	for _, field := range urlFields {
		if val, ok := args[field]; ok {
			if s, ok := val.(string); ok {
				targetURL = s
				break
			}
		}
	}

	if targetURL == "" {
		return nil // No URL to check
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return &contracts.Decision{
			Action:       contracts.ActionBlock,
			PolicyRuleID: "egress.invalid_url",
			Reason:       fmt.Sprintf("Invalid URL: %s", err.Error()),
		}
	}

	// Check if it's an IP address (block by default if BlockPrivateIPs is true)
	if ec.BlockPrivateIPs {
		if ip := net.ParseIP(parsedURL.Hostname()); ip != nil {
			if isPrivateIP(ip) {
				return &contracts.Decision{
					Action:       contracts.ActionBlock,
					PolicyRuleID: "egress.private_ip",
					Reason:       fmt.Sprintf("Private IP access blocked: %s", parsedURL.Hostname()),
				}
			}
		}
	}

	// Check denylist
	if len(ec.DomainDenylist) > 0 {
		for _, denied := range ec.DomainDenylist {
			if matchesDomain(parsedURL.Hostname(), denied) {
				return &contracts.Decision{
					Action:       contracts.ActionBlock,
					PolicyRuleID: "egress.domain_denylist",
					Reason:       fmt.Sprintf("Domain in denylist: %s", parsedURL.Hostname()),
				}
			}
		}
	}

	// Check allowlist (if defined, only allowed domains pass)
	if len(ec.DomainAllowlist) > 0 {
		allowed := false
		for _, permitted := range ec.DomainAllowlist {
			if matchesDomain(parsedURL.Hostname(), permitted) {
				allowed = true
				break
			}
		}
		if !allowed {
			return &contracts.Decision{
				Action:       contracts.ActionBlock,
				PolicyRuleID: "egress.domain_not_allowed",
				Reason:       fmt.Sprintf("Domain not in allowlist: %s", parsedURL.Hostname()),
			}
		}
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return true
	}
	// Check for metadata endpoints (AWS, GCP, Azure)
	if ip.String() == "169.254.169.254" {
		return true
	}
	return false
}

func matchesDomain(hostname string, pattern string) bool {
	// Support wildcard patterns like *.example.com
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // Remove *
		return strings.HasSuffix(hostname, suffix) || hostname == pattern[2:]
	}
	return hostname == pattern
}

// ValidateOutput validates tool response against output schema
func (e *Evaluator) ValidateOutput(toolPolicy *CompiledToolPolicy, output interface{}) error {
	if toolPolicy.OutputSchema == nil {
		return nil
	}

	result, err := toolPolicy.OutputSchema.Validate(gojsonschema.NewGoLoader(output))
	if err != nil {
		return fmt.Errorf("output schema validation error: %w", err)
	}

	if !result.Valid() {
		errors := make([]string, len(result.Errors()))
		for i, e := range result.Errors() {
			errors[i] = e.String()
		}
		return fmt.Errorf("output validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}
