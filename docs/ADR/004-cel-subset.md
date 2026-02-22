# ADR-004: CEL Subset for Policy Expressions

**Status**: Accepted  
**Date**: 2026-02-03  
**Authors**: AegisRun Team

---

## Context

AegisRun policies need conditional expressions to enable dynamic rules:

```yaml
tools:
  - name: http_request
    action: allow
    conditions:
      - 'args.method == "GET"'
      - 'counters.tool_calls < 100'
```

We needed to choose an expression language that is:
- Safe to evaluate (no side effects, no infinite loops)
- Fast to parse and execute
- Familiar to developers
- Auditable (clear semantics)

## Decision

We implement a **strict subset of CEL** (Common Expression Language) for policy conditions.

## Rationale

### Why CEL-Based

1. **Security by Design**
   - No loops → no infinite execution
   - No variable assignment → no state mutation
   - No function definitions → limited attack surface
   - Sandboxed evaluation environment

2. **Google-Backed Standard**
   - Used in: Firebase Security Rules, Kubernetes RBAC, Envoy
   - Well-documented semantics
   - Growing ecosystem

3. **Familiar Syntax**
   - C-like operators (`&&`, `||`, `==`, `!=`)
   - Dot notation for field access (`args.url`)
   - No learning curve for most developers

4. **Predictable Performance**
   - O(1) evaluation for most expressions
   - No recursion or unbounded iteration
   - Sub-millisecond execution guaranteed

### Supported Subset

#### Operators

| Category | Operators |
|----------|-----------|
| Comparison | `==`, `!=`, `<`, `>`, `<=`, `>=` |
| Logical | `&&`, `||` |
| Grouping | `(`, `)` |

#### Variables

| Variable | Type | Description |
|----------|------|-------------|
| `tool_name` | string | Current tool being called |
| `args` | object | Tool arguments |
| `args.<field>` | any | Specific argument field |
| `state` | object | Agent state vector |
| `state.<field>` | any | Specific state field |
| `metadata` | object | Run metadata |
| `metadata.<field>` | any | Specific metadata field |
| `counters.tool_calls` | int | Total tool calls in run |
| `counters.steps` | int | Total steps in run |
| `counters.bytes_egressed` | int | Total bytes sent |
| `counters.retries` | int | Total retries |

#### Literals

- Strings: `"hello"`, `'world'`
- Numbers: `42`, `3.14`
- Booleans: `true`, `false`

### What We Explicitly Exclude

1. **No loops**: `for`, `while`, list comprehensions
2. **No functions**: `len()`, `contains()`, custom functions
3. **No assignment**: `x = 5`, `let`, `var`
4. **No regex**: `matches()`, `~` operator
5. **No lists/maps**: `[1, 2, 3]`, `{"key": "value"}`
6. **No macros**: `has()`, `all()`, `exists()`

## Implementation

### Parser (Recursive Descent)

```go
func (p *Parser) parseExpression() (Expr, error) {
    return p.parseOr()
}

func (p *Parser) parseOr() (Expr, error) {
    left, err := p.parseAnd()
    if err != nil {
        return nil, err
    }
    for p.match("||") {
        right, err := p.parseAnd()
        if err != nil {
            return nil, err
        }
        left = &BinaryExpr{Op: "||", Left: left, Right: right}
    }
    return left, nil
}
```

### Evaluator

```go
func (e *Evaluator) Eval(expr Expr, ctx Context) (bool, error) {
    switch node := expr.(type) {
    case *BinaryExpr:
        left, _ := e.Eval(node.Left, ctx)
        right, _ := e.Eval(node.Right, ctx)
        switch node.Op {
        case "&&":
            return left && right, nil
        case "||":
            return left || right, nil
        case "==":
            return e.equals(node.Left, node.Right, ctx)
        // ...
        }
    case *FieldAccess:
        return e.lookupField(node.Path, ctx)
    case *Literal:
        return node.Value, nil
    }
}
```

### Compilation Step

Expressions are parsed and validated at policy creation time:

```go
func ValidateConditions(conditions []string) error {
    parser := NewParser()
    for i, cond := range conditions {
        ast, err := parser.Parse(cond)
        if err != nil {
            return fmt.Errorf("condition %d: %w", i, err)
        }
        if err := validateAST(ast); err != nil {
            return fmt.Errorf("condition %d: %w", i, err)
        }
    }
    return nil
}
```

## Consequences

### Positive

- Expressions cannot cause denial-of-service
- Policy evaluation is predictable and fast (<1ms)
- Conditions are auditable (clear intent)
- Familiar syntax reduces errors

### Negative

- Cannot express complex conditions (regex, list operations)
- Must add features explicitly as needs arise
- Not full CEL compatibility (may confuse CEL experts)

### Mitigations

- Document exact supported syntax clearly
- Provide clear error messages for unsupported features
- Consider adding features incrementally (e.g., `contains()`)
- Validate at policy creation, not evaluation time

## Future Considerations

### Potential Additions

1. **String functions**: `startsWith()`, `endsWith()`, `contains()`
2. **Type checking**: `type(x) == "string"`
3. **Null handling**: `args.optional ?? "default"`

Each addition must be:
- Side-effect free
- Bounded execution time
- Clearly documented

### Migration Path

If we need full CEL:
1. Use google/cel-go library
2. Configure strict environment (no I/O functions)
3. Set timeout on evaluation
4. Keep existing subset expressions valid

## Alternatives Considered

### JavaScript (V8/QuickJS)

Rejected because:
- Turing complete (infinite loop risk)
- Large attack surface
- Heavy runtime (V8) or limited features (QuickJS)

### Lua

Rejected because:
- Less familiar to most developers
- Would need sandboxing
- Turing complete

### JSON Logic

Rejected because:
- Verbose syntax: `{"==":[{"var":"args.method"},"GET"]}`
- Less readable for complex conditions
- Not widely known

### Full CEL

Rejected (for now) because:
- Includes features we don't need/want
- `has()`, `exists()` add complexity
- Can adopt later if needed

## Related Decisions

- [ADR-001: Single Binary API](001-single-binary-api.md)
- [ADR-003: Ed25519 Signing](003-ed25519-signing.md)

---

**End of ADR-004**
