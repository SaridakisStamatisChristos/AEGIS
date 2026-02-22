package policy

import (
	"fmt"
	"regexp"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/xeipuuv/gojsonschema"
)

// Compiler validates and compiles policy specs
type Compiler struct {
	celParser *CELParser
}

func NewCompiler() *Compiler {
	return &Compiler{
		celParser: NewCELParser(),
	}
}

// Compile validates a policy spec and prepares it for runtime evaluation
func (c *Compiler) Compile(spec *contracts.PolicySpec) (*CompiledPolicy, error) {
	cp := &CompiledPolicy{
		ToolPolicies:   make(map[string]*CompiledToolPolicy),
		Budgets:        spec.Budgets,
		EgressControls: spec.EgressControls,
		Redaction:      spec.Redaction,
	}

	// Compile each tool policy
	for _, tp := range spec.Tools {
		compiled, err := c.compileToolPolicy(&tp)
		if err != nil {
			return nil, fmt.Errorf("compile tool policy %s: %w", tp.Name, err)
		}
		cp.ToolPolicies[tp.Name] = compiled
	}

	// Compile egress controls
	if spec.EgressControls != nil {
		if err := c.compileEgressControls(spec.EgressControls); err != nil {
			return nil, fmt.Errorf("compile egress controls: %w", err)
		}
	}

	// Compile redaction patterns
	if spec.Redaction != nil {
		patterns, err := c.compileRedactionPatterns(spec.Redaction)
		if err != nil {
			return nil, fmt.Errorf("compile redaction patterns: %w", err)
		}
		cp.RedactionPatterns = patterns
	}

	return cp, nil
}

func (c *Compiler) compileToolPolicy(tp *contracts.ToolPolicy) (*CompiledToolPolicy, error) {
	ctp := &CompiledToolPolicy{
		Name:   tp.Name,
		Action: tp.Action,
	}

	// Compile arg schema
	if tp.ArgSchema != nil {
		schema, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(tp.ArgSchema))
		if err != nil {
			return nil, fmt.Errorf("invalid arg_schema: %w", err)
		}
		ctp.ArgSchema = schema
	}

	// Compile output schema
	if tp.OutputSchema != nil {
		schema, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(tp.OutputSchema))
		if err != nil {
			return nil, fmt.Errorf("invalid output_schema: %w", err)
		}
		ctp.OutputSchema = schema
	}

	// Compile conditions (CEL expressions)
	if len(tp.Conditions) > 0 {
		conditions := make([]*CompiledCondition, len(tp.Conditions))
		for i, condStr := range tp.Conditions {
			cond, err := c.celParser.Parse(condStr)
			if err != nil {
				return nil, fmt.Errorf("invalid condition %d: %w", i, err)
			}
			conditions[i] = cond
		}
		ctp.Conditions = conditions
	}

	return ctp, nil
}

func (c *Compiler) compileEgressControls(ec *contracts.EgressControls) error {
	// Validate domain allowlist
	for _, domain := range ec.DomainAllowlist {
		if !isValidDomain(domain) {
			return fmt.Errorf("invalid domain in allowlist: %s", domain)
		}
	}

	// Validate domain denylist
	for _, domain := range ec.DomainDenylist {
		if !isValidDomain(domain) {
			return fmt.Errorf("invalid domain in denylist: %s", domain)
		}
	}

	return nil
}

func (c *Compiler) compileRedactionPatterns(rc *contracts.RedactionConfig) ([]*regexp.Regexp, error) {
	patterns := make([]*regexp.Regexp, len(rc.Patterns))
	for i, patternStr := range rc.Patterns {
		re, err := regexp.Compile(patternStr)
		if err != nil {
			return nil, fmt.Errorf("invalid redaction pattern %d: %w", i, err)
		}
		patterns[i] = re
	}
	return patterns, nil
}

// domainRegex validates FQDN domains, optionally with a wildcard prefix.
// Compiled once at package level to avoid per-call overhead.
var domainRegex = regexp.MustCompile(`^(\*\.)?[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?))*$`)

func isValidDomain(domain string) bool {
	return domainRegex.MatchString(domain)
}

// CompiledPolicy is the runtime-ready form of a policy
type CompiledPolicy struct {
	ToolPolicies      map[string]*CompiledToolPolicy
	Budgets           contracts.Budgets
	EgressControls    *contracts.EgressControls
	Redaction         *contracts.RedactionConfig
	RedactionPatterns []*regexp.Regexp
}

type CompiledToolPolicy struct {
	Name         string
	Action       contracts.PolicyAction
	ArgSchema    *gojsonschema.Schema
	OutputSchema *gojsonschema.Schema
	Conditions   []*CompiledCondition
}

type CompiledCondition struct {
	Expression string
	Evaluator  func(map[string]interface{}) (bool, error)
}
