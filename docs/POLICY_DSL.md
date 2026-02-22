# AegisRun Policy DSL Reference

**Version**: 1.0.0  
**Last Updated**: 2026-02-03

---

## 1. Overview

AegisRun policies are defined in YAML and enforce constraints on agent tool usage. Policies support:

- Tool allow/deny lists
- Per-tool argument JSON Schema validation
- Output validators (JSON Schema + regex)
- Budget limits (tool calls, wall clock, retries, bytes egressed)
- Data egress controls (domain allowlist/denylist)
- Redaction rules (PII/secrets masking)
- Approval requirements for high-risk operations
- Expression-based conditions (CEL-like subset)

---

## 2. Policy Structure

```yaml
# policy.yaml
name: production-policy
version: v1

tools:
  - name: http_request
    action: allow
    arg_schema:
      type: object
      properties:
        url:
          type: string
          format: uri
        method:
          type: string
          enum: [GET, POST, PUT, DELETE]
      required: [url, method]
    output_schema:
      type: object
      properties:
        status_code:
          type: integer
    conditions:
      - 'args.method != "DELETE"'
      - 'counters.tool_calls < 100'

  - name: shell_exec
    action: block

  - name: database_query
    action: require_approval

budgets:
  max_tool_calls: 500
  max_wall_clock_sec: 3600
  max_retries: 10
  max_bytes_egressed: 10485760  # 10MB

egress_controls:
  domain_allowlist:
    - "*.example.com"
    - "api.github.com"
  domain_denylist:
    - "evil.com"
  block_private_ips: true

redaction:
  patterns:
    - '\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b'  # Email
    - '\b\d{3}-\d{2}-\d{4}\b'  # SSN
  mask_strategy: redact
```

---

## 3. Tool Rules

### 3.1 Actions

| Action | Behavior |
|--------|----------|
| `allow` | Tool call permitted; execute normally |
| `warn` | Tool call permitted; log warning |
| `redact` | Arguments/response redacted before storage |
| `block` | Tool call rejected; return denial |
| `require_approval` | Pause until human approves |
| `degrade` | Use fallback/limited functionality |

### 3.2 Argument Schema

Validate tool arguments using JSON Schema draft-07:

```yaml
tools:
  - name: send_email
    action: allow
    arg_schema:
      type: object
      properties:
        to:
          type: string
          format: email
        subject:
          type: string
          maxLength: 100
        body:
          type: string
          maxLength: 10000
      required: [to, subject, body]
      additionalProperties: false
```

### 3.3 Output Schema

Validate tool response:

```yaml
tools:
  - name: fetch_data
    action: allow
    output_schema:
      type: object
      properties:
        data:
          type: array
        count:
          type: integer
          minimum: 0
      required: [data, count]
```

### 3.4 Conditions (CEL-Subset)

Conditions are evaluated before executing the tool. If any condition returns `false`, the tool call is blocked.

**Supported operators:**
- Comparison: `==`, `!=`, `<`, `>`, `<=`, `>=`
- Logical: `&&`, `||`
- Parentheses: `(`, `)`

**Available context variables:**

| Variable | Description |
|----------|-------------|
| `tool_name` | Name of the tool being called |
| `args` | Tool arguments (object) |
| `args.<field>` | Specific argument field |
| `state.<field>` | Agent state vector field |
| `metadata.<field>` | Run metadata field |
| `counters.tool_calls` | Total tool calls in run |
| `counters.steps` | Total steps in run |
| `counters.bytes_egressed` | Total bytes egressed |
| `counters.retries` | Total retries |
| `counters.blocks` | Total blocked calls |

**Examples:**
```yaml
conditions:
  # Only allow GET and POST methods
  - 'args.method == "GET" || args.method == "POST"'
  
  # Limit file size
  - 'args.size < 1000000'
  
  # Check agent state
  - 'state.authenticated == true'
  
  # Budget check
  - 'counters.tool_calls < 100'
  
  # Metadata check
  - 'metadata.environment != "production"'
```

---

## 4. Budgets

Budgets are global limits for the entire run:

```yaml
budgets:
  max_tool_calls: 500        # Total tool invocations
  max_wall_clock_sec: 3600   # Total runtime in seconds
  max_retries: 10            # Total retry attempts
  max_bytes_egressed: 10485760  # Total outbound data
```

When a budget is exceeded, subsequent tool calls are blocked with `budget.<type>` policy rule ID.

---

## 5. Egress Controls

Control outbound network requests:

```yaml
egress_controls:
  # Only allow requests to these domains
  domain_allowlist:
    - "api.example.com"
    - "*.internal.corp"
  
  # Block requests to these domains
  domain_denylist:
    - "malware.com"
    - "*.spam.net"
  
  # Block private IP ranges (default: true)
  # Prevents SSRF attacks to 127.0.0.1, 10.x.x.x, 169.254.169.254, etc.
  block_private_ips: true
```

**Domain matching:**
- Exact match: `api.github.com`
- Wildcard: `*.example.com` (matches `api.example.com`, `www.example.com`)

**Evaluation order:**
1. Check denylist (block if match)
2. Check allowlist (block if no match when allowlist is defined)
3. Check private IP (block if private and `block_private_ips: true`)

---

## 6. Redaction

Automatically mask sensitive data:

```yaml
redaction:
  patterns:
    # Email addresses
    - '\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b'
    
    # Credit card numbers
    - '\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b'
    
    # Social Security Numbers
    - '\b\d{3}-\d{2}-\d{4}\b'
    
    # AWS keys
    - 'AKIA[0-9A-Z]{16}'
    
    # Custom API key pattern
    - 'sk_live_[A-Za-z0-9]{32}'
  
  mask_strategy: redact  # hash | redact | truncate
```

**Mask strategies:**

| Strategy | Example Input | Example Output |
|----------|--------------|----------------|
| `hash` | `user@example.com` | `[REDACTED:EMAIL:a1b2c3d4]` |
| `redact` | `user@example.com` | `[REDACTED:EMAIL]` |
| `truncate` | `user@example.com` | `user****[REDACTED:EMAIL]` |

**Built-in patterns (always applied):**
- Email addresses
- US/International phone numbers
- Credit card numbers
- Social Security Numbers
- Generic API key patterns
- AWS access keys
- JWT tokens

---

## 7. Policy Lifecycle

```
Draft → Review → Approved → Deployed → Deprecated
```

| Status | Editable | Usable by Runs |
|--------|----------|----------------|
| `draft` | Yes | No |
| `review` | No | No |
| `approved` | No | Yes |
| `deployed` | No | Yes (active default) |
| `deprecated` | No | Existing runs only |

### 7.1 Version Immutability

Once a policy version is `approved`, its spec cannot be modified. Changes require creating a new version.

### 7.2 Two-Person Approval

For high-risk policies, require approval from 2+ approvers:

```yaml
# Configured at org level
approval_requirements:
  min_approvers: 2
  required_roles:
    - policy_admin
    - security_admin
```

---

## 8. Default Tool Policy

Tools not explicitly defined in the policy are **blocked by default**:

```
Tool 'unknown_tool' not defined in policy
Policy rule: default.undefined_tool
Action: block
```

To allow undefined tools (not recommended):

```yaml
default_action: warn  # allow | warn | block (default)
```

---

## 9. Policy Validation

Policies are validated at compile time:

1. **Schema validation**: YAML/JSON structure matches PolicySpec schema
2. **Tool name uniqueness**: No duplicate tool names
3. **Condition parsing**: All conditions must be valid CEL expressions
4. **Schema compilation**: All JSON Schemas must be valid draft-07
5. **Pattern compilation**: All regex patterns must compile
6. **Budget sanity**: All budget values must be non-negative

**Validation errors:**
```
Policy validation failed:
- Line 15: Duplicate tool name 'http_request'
- Line 23: Invalid condition syntax: unexpected token 'AND'
- Line 35: Invalid regex pattern: unclosed group
```

---

## 10. Examples

### 10.1 Restrictive Production Policy

```yaml
name: production-lockdown
version: v1

tools:
  - name: http_request
    action: allow
    arg_schema:
      type: object
      properties:
        url:
          type: string
          format: uri
        method:
          type: string
          enum: [GET, POST]
      required: [url, method]
    conditions:
      - 'args.method == "GET" || args.method == "POST"'

  - name: file_read
    action: allow
    arg_schema:
      type: object
      properties:
        path:
          type: string
          pattern: '^/data/.*$'

  - name: shell_exec
    action: block

  - name: database_write
    action: require_approval

budgets:
  max_tool_calls: 100
  max_wall_clock_sec: 300
  max_bytes_egressed: 1048576

egress_controls:
  domain_allowlist:
    - "api.internal.company.com"
  block_private_ips: true

redaction:
  mask_strategy: hash
```

### 10.2 Permissive Development Policy

```yaml
name: development-permissive
version: v1

tools:
  - name: http_request
    action: warn
  - name: file_read
    action: allow
  - name: file_write
    action: warn
  - name: shell_exec
    action: warn

default_action: warn

budgets:
  max_tool_calls: 10000
  max_wall_clock_sec: 86400

egress_controls:
  block_private_ips: false

redaction:
  mask_strategy: truncate
```

---

## 11. Error Responses

When a tool call is blocked, the gateway returns:

```json
{
  "tool_call_id": "01JQZX3K2FGH9VWBCD45EFGHIJ",
  "decision": {
    "action": "block",
    "policy_rule_id": "tool.http_request.condition.0",
    "reason": "Condition failed: args.method == \"GET\" || args.method == \"POST\""
  },
  "response": null,
  "error": "Tool call blocked by policy"
}
```

---

## 12. Best Practices

1. **Start restrictive**: Begin with a minimal allowlist and expand as needed
2. **Version everything**: Create new versions for any policy changes
3. **Test policies**: Use staging environment before deploying to production
4. **Review conditions**: Ensure conditions don't create security bypasses
5. **Monitor blocks**: Alert on high block rates (may indicate bugs or attacks)
6. **Audit approvals**: Regularly review who approved what policies
7. **Rotate patterns**: Update redaction patterns as new PII types emerge

---

**End of POLICY_DSL.md**
