package policy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CELParser implements a minimal CEL-like expression language
// Supports: field access, comparisons, logical operators, string operations
// Does NOT support: functions, complex types, macros
type CELParser struct{}

func NewCELParser() *CELParser {
	return &CELParser{}
}

// Parse compiles a CEL-like expression into an evaluator
func (p *CELParser) Parse(expr string) (*CompiledCondition, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty expression")
	}

	evaluator, err := p.buildEvaluator(expr)
	if err != nil {
		return nil, err
	}

	return &CompiledCondition{
		Expression: expr,
		Evaluator:  evaluator,
	}, nil
}

func (p *CELParser) buildEvaluator(expr string) (func(map[string]interface{}) (bool, error), error) {
	// Parse the expression into tokens
	tokens, err := p.tokenize(expr)
	if err != nil {
		return nil, err
	}

	// Build evaluator
	return func(ctx map[string]interface{}) (bool, error) {
		return p.evaluate(tokens, ctx)
	}, nil
}

type TokenType int

const (
	TokenField TokenType = iota
	TokenString
	TokenNumber
	TokenOperator
	TokenParen
	TokenLogical
)

type Token struct {
	Type  TokenType
	Value string
}

func (p *CELParser) tokenize(expr string) ([]Token, error) {
	var tokens []Token

	// Simple regex-based tokenizer
	// Supports: field.access, "strings", numbers, ==, !=, <, >, <=, >=, &&, ||, (), !
	patterns := []struct {
		regex   *regexp.Regexp
		tokType TokenType
	}{
		{regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_\.]*`), TokenField},
		{regexp.MustCompile(`^"([^"]*)"`), TokenString},
		{regexp.MustCompile(`^[0-9]+(\.[0-9]+)?`), TokenNumber},
		{regexp.MustCompile(`^(==|!=|<=|>=|<|>)`), TokenOperator},
		{regexp.MustCompile(`^(&&|\|\|)`), TokenLogical},
		{regexp.MustCompile(`^[()]`), TokenParen},
	}

	i := 0
	for i < len(expr) {
		// Skip whitespace
		if expr[i] == ' ' || expr[i] == '\t' {
			i++
			continue
		}

		matched := false
		for _, p := range patterns {
			if loc := p.regex.FindStringIndex(expr[i:]); loc != nil && loc[0] == 0 {
				token := expr[i : i+loc[1]]
				tokens = append(tokens, Token{Type: p.tokType, Value: token})
				i += loc[1]
				matched = true
				break
			}
		}

		if !matched {
			return nil, fmt.Errorf("unexpected character at position %d: %c", i, expr[i])
		}
	}

	return tokens, nil
}

func (p *CELParser) evaluate(tokens []Token, ctx map[string]interface{}) (bool, error) {
	if len(tokens) == 0 {
		return false, fmt.Errorf("empty expression")
	}

	// Simple recursive descent parser
	return p.parseLogicalOr(tokens, ctx, &tokenIndex{idx: 0})
}

type tokenIndex struct {
	idx int
}

func (p *CELParser) parseLogicalOr(tokens []Token, ctx map[string]interface{}, ti *tokenIndex) (bool, error) {
	left, err := p.parseLogicalAnd(tokens, ctx, ti)
	if err != nil {
		return false, err
	}

	for ti.idx < len(tokens) && tokens[ti.idx].Value == "||" {
		ti.idx++ // consume ||
		right, err := p.parseLogicalAnd(tokens, ctx, ti)
		if err != nil {
			return false, err
		}
		left = left || right
	}

	return left, nil
}

func (p *CELParser) parseLogicalAnd(tokens []Token, ctx map[string]interface{}, ti *tokenIndex) (bool, error) {
	left, err := p.parseComparison(tokens, ctx, ti)
	if err != nil {
		return false, err
	}

	for ti.idx < len(tokens) && tokens[ti.idx].Value == "&&" {
		ti.idx++ // consume &&
		right, err := p.parseComparison(tokens, ctx, ti)
		if err != nil {
			return false, err
		}
		left = left && right
	}

	return left, nil
}

func (p *CELParser) parseComparison(tokens []Token, ctx map[string]interface{}, ti *tokenIndex) (bool, error) {
	if ti.idx >= len(tokens) {
		return false, fmt.Errorf("unexpected end of expression")
	}

	// Handle parentheses
	if tokens[ti.idx].Type == TokenParen && tokens[ti.idx].Value == "(" {
		ti.idx++ // consume (
		result, err := p.parseLogicalOr(tokens, ctx, ti)
		if err != nil {
			return false, err
		}
		if ti.idx >= len(tokens) || tokens[ti.idx].Value != ")" {
			return false, fmt.Errorf("missing closing parenthesis")
		}
		ti.idx++ // consume )
		return result, nil
	}

	// Parse left operand
	left, err := p.getValue(tokens[ti.idx], ctx)
	if err != nil {
		return false, err
	}
	ti.idx++

	// Check for operator
	if ti.idx >= len(tokens) || tokens[ti.idx].Type != TokenOperator {
		// Boolean field access (e.g., "enabled")
		if b, ok := left.(bool); ok {
			return b, nil
		}
		return false, fmt.Errorf("expected operator")
	}

	operator := tokens[ti.idx].Value
	ti.idx++

	// Parse right operand
	if ti.idx >= len(tokens) {
		return false, fmt.Errorf("missing right operand")
	}
	right, err := p.getValue(tokens[ti.idx], ctx)
	if err != nil {
		return false, err
	}
	ti.idx++

	// Evaluate comparison
	return p.compare(left, operator, right)
}

func (p *CELParser) getValue(token Token, ctx map[string]interface{}) (interface{}, error) {
	switch token.Type {
	case TokenField:
		return p.getField(token.Value, ctx)
	case TokenString:
		// Remove quotes
		return token.Value[1 : len(token.Value)-1], nil
	case TokenNumber:
		if strings.Contains(token.Value, ".") {
			return strconv.ParseFloat(token.Value, 64)
		}
		return strconv.ParseInt(token.Value, 10, 64)
	default:
		return nil, fmt.Errorf("unexpected token type: %v", token.Type)
	}
}

func (p *CELParser) getField(path string, ctx map[string]interface{}) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := ctx

	for i, part := range parts {
		val, ok := current[part]
		if !ok {
			return nil, fmt.Errorf("field not found: %s", path)
		}

		if i == len(parts)-1 {
			return val, nil
		}

		// Navigate deeper
		if m, ok := val.(map[string]interface{}); ok {
			current = m
		} else {
			return nil, fmt.Errorf("cannot navigate through non-map: %s", part)
		}
	}

	return current, nil
}

func (p *CELParser) compare(left interface{}, op string, right interface{}) (bool, error) {
	switch op {
	case "==":
		return p.equals(left, right), nil
	case "!=":
		return !p.equals(left, right), nil
	case "<":
		return p.lessThan(left, right)
	case "<=":
		return p.lessThanOrEqual(left, right)
	case ">":
		return p.greaterThan(left, right)
	case ">=":
		return p.greaterThanOrEqual(left, right)
	default:
		return false, fmt.Errorf("unknown operator: %s", op)
	}
}

func (p *CELParser) equals(a, b interface{}) bool {
	// Type-aware equality
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case int64:
		bv, ok := b.(int64)
		return ok && av == bv
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	default:
		return false
	}
}

func (p *CELParser) lessThan(a, b interface{}) (bool, error) {
	an, aok := toNumber(a)
	bn, bok := toNumber(b)
	if !aok || !bok {
		return false, fmt.Errorf("cannot compare non-numeric values with <")
	}
	return an < bn, nil
}

func (p *CELParser) lessThanOrEqual(a, b interface{}) (bool, error) {
	an, aok := toNumber(a)
	bn, bok := toNumber(b)
	if !aok || !bok {
		return false, fmt.Errorf("cannot compare non-numeric values with <=")
	}
	return an <= bn, nil
}

func (p *CELParser) greaterThan(a, b interface{}) (bool, error) {
	an, aok := toNumber(a)
	bn, bok := toNumber(b)
	if !aok || !bok {
		return false, fmt.Errorf("cannot compare non-numeric values with >")
	}
	return an > bn, nil
}

func (p *CELParser) greaterThanOrEqual(a, b interface{}) (bool, error) {
	an, aok := toNumber(a)
	bn, bok := toNumber(b)
	if !aok || !bok {
		return false, fmt.Errorf("cannot compare non-numeric values with >=")
	}
	return an >= bn, nil
}

func toNumber(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}
