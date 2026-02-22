# AegisRun Core Contracts v1.0.0

**Status**: Normative  
**Last Updated**: 2026-02-03  
**Authors**: AegisRun Architecture Team

---

## 1. Overview

This document defines the canonical data contracts for AegisRun. All implementations (API, SDKs, verifier) MUST conform to these schemas exactly.

### 1.1 Design Principles

- **Deterministic IDs**: All IDs are ULIDv2 (lexicographically sortable, timestamp-embedded)
- **Hash Chaining**: Events form append-only Merkle-like chain
- **Immutability**: Once signed, runs cannot be altered
- **Redaction**: PII/secrets masked at ingestion, never stored raw
- **Versioning**: Semantic versioning for schema evolution

### 1.2 Type System Conventions

- Timestamps: RFC3339 with microsecond precision (UTC)
- Hashes: SHA256, hex-encoded (64 chars)
- Signatures: Ed25519, base64-encoded (88 chars)
- IDs: ULIDv2, 26 chars (e.g., `01JQZX3K2FGH9VWBCD45EFGHIJ`)

---

## 2. Core Domain Objects

### 2.1 Run

A `Run` represents a single agent execution session.

**JSON Schema**:
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "https://aegisrun.io/schemas/v1/run.json",
  "type": "object",
  "required": ["run_id", "org_id", "policy_ref", "created_at", "status"],
  "properties": {
    "run_id": {
      "type": "string",
      "pattern": "^[0-9A-HJKMNP-TV-Z]{26}$",
      "description": "ULID v2 identifier"
    },
    "org_id": {
      "type": "string",
      "pattern": "^[0-9A-HJKMNP-TV-Z]{26}$"
    },
    "parent_run_id": {
      "type": ["string", "null"],
      "pattern": "^[0-9A-HJKMNP-TV-Z]{26}$",
      "description": "For nested runs"
    },
    "policy_ref": {
      "type": "object",
      "required": ["policy_id", "version"],
      "properties": {
        "policy_id": {"type": "string"},
        "version": {"type": "string", "pattern": "^v[0-9]+$"}
      }
    },
    "state_schema_ref": {
      "type": ["object", "null"],
      "properties": {
        "schema_id": {"type": "string"},
        "version": {"type": "string"}
      }
    },
    "metadata": {
      "type": "object",
      "additionalProperties": true,
      "description": "User-provided metadata (tags, environment, etc.)"
    },
    "created_at": {"type": "string", "format": "date-time"},
    "ended_at": {"type": ["string", "null"], "format": "date-time"},
    "status": {
      "type": "string",
      "enum": ["running", "completed", "failed", "blocked", "cancelled"]
    },
    "outcome": {
      "type": ["object", "null"],
      "properties": {
        "result": {"type": ["string", "object", "null"]},
        "error": {"type": ["string", "null"]},
        "exit_reason": {"type": "string"}
      }
    },
    "counters": {
      "type": "object",
      "properties": {
        "steps": {"type": "integer"},
        "tool_calls": {"type": "integer"},
        "bytes_egressed": {"type": "integer"},
        "retries": {"type": "integer"},
        "blocks": {"type": "integer"}
      }
    },
    "evidence_hash": {
      "type": ["string", "null"],
      "pattern": "^[a-f0-9]{64}$",
      "description": "Root hash of event chain"
    },
    "signature": {
      "type": ["string", "null"],
      "description": "Ed25519 signature (base64) of run manifest"
    }
  }
}
```

---

### 2.2 Step

A `Step` is a logical unit of work within a run (e.g., "plan", "execute", "reflect").

**JSON Schema**:
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "https://aegisrun.io/schemas/v1/step.json",
  "type": "object",
  "required": ["step_id", "run_id", "seq_no", "name", "started_at"],
  "properties": {
    "step_id": {"type": "string", "pattern": "^[0-9A-HJKMNP-TV-Z]{26}$"},
    "run_id": {"type": "string", "pattern": "^[0-9A-HJKMNP-TV-Z]{26}$"},
    "parent_step_id": {"type": ["string", "null"]},
    "seq_no": {"type": "integer", "minimum": 0},
    "name": {"type": "string", "maxLength": 255},
    "state_vector": {
      "type": "object",
      "description": "Agent state snapshot at step start"
    },
    "started_at": {"type": "string", "format": "date-time"},
    "ended_at": {"type": ["string", "null"], "format": "date-time"},
    "status": {
      "type": "string",
      "enum": ["running", "completed", "failed"]
    },
    "error": {"type": ["string", "null"]}
  }
}
```

---

### 2.3 ToolCall

A `ToolCall` represents a single invocation of a tool through the gateway.

**JSON Schema**:
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "https://aegisrun.io/schemas/v1/toolcall.json",
  "type": "object",
  "required": ["tool_call_id", "run_id", "step_id", "seq_no", "tool_name", "requested_at"],
  "properties": {
    "tool_call_id": {"type": "string", "pattern": "^[0-9A-HJKMNP-TV-Z]{26}$"},
    "run_id": {"type": "string", "pattern": "^[0-9A-HJKMNP-TV-Z]{26}$"},
    "step_id": {"type": "string", "pattern": "^[0-9A-HJKMNP-TV-Z]{26}$"},
    "seq_no": {"type": "integer", "minimum": 0},
    "tool_name": {"type": "string", "maxLength": 255},
    "args": {
      "type": "object",
      "description": "Tool input arguments"
    },
    "args_redacted": {
      "type": "boolean",
      "description": "True if args contain redacted fields"
    },
    "requested_at": {"type": "string", "format": "date-time"},
    "responded_at": {"type": ["string", "null"], "format": "date-time"},
    "decision": {
      "type": "object",
      "required": ["action", "policy_rule_id"],
      "properties": {
        "action": {
          "type": "string",
          "enum": ["allow", "warn", "redact", "block", "require_approval", "degrade"]
        },
        "policy_rule_id": {"type": "string"},
        "reason": {"type": "string"},
        "approval_id": {"type": ["string", "null"]}
      }
    },
    "response": {
      "type": ["object", "null"],
      "properties": {
        "result": {},
        "error": {"type": ["string", "null"]},
        "duration_ms": {"type": "number"}
      }
    },
    "response_redacted": {"type": "boolean"},
    "metadata": {
      "type": "object",
      "properties": {
        "executor": {"type": "string"},
        "retry_count": {"type": "integer"}
      }
    }
  }
}
```

---

### 2.4 Event (Ledger Entry)

All runtime events are persisted in an append-only log with hash chaining.

**JSON Schema**:
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "https://aegisrun.io/schemas/v1/event.json",
  "type": "object",
  "required": ["event_id", "run_id", "seq_no", "event_type", "timestamp", "prev_hash", "event_hash"],
  "properties": {
    "event_id": {"type": "string", "pattern": "^[0-9A-HJKMNP-TV-Z]{26}$"},
    "run_id": {"type": "string", "pattern": "^[0-9A-HJKMNP-TV-Z]{26}$"},
    "seq_no": {"type": "integer", "minimum": 0},
    "event_type": {
      "type": "string",
      "enum": [
        "run.started",
        "run.ended",
        "step.started",
        "step.ended",
        "tool.requested",
        "tool.decided",
        "tool.responded",
        "decision.overridden",
        "state.updated"
      ]
    },
    "timestamp": {"type": "string", "format": "date-time"},
    "payload": {
      "type": "object",
      "description": "Event-specific data"
    },
    "prev_hash": {
      "type": ["string", "null"],
      "pattern": "^[a-f0-9]{64}$",
      "description": "Hash of previous event (null for first event)"
    },
    "event_hash": {
      "type": "string",
      "pattern": "^[a-f0-9]{64}$",
      "description": "SHA256(canonical_json(event sans event_hash) || prev_hash)"
    }
  }
}
```

---

### 2.5 Policy

A `Policy` defines runtime constraints and controls.

**JSON Schema**:
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "https://aegisrun.io/schemas/v1/policy.json",
  "type": "object",
  "required": ["policy_id", "org_id", "name", "version", "status", "created_at"],
  "properties": {
    "policy_id": {"type": "string", "pattern": "^[0-9A-HJKMNP-TV-Z]{26}$"},
    "org_id": {"type": "string", "pattern": "^[0-9A-HJKMNP-TV-Z]{26}$"},
    "name": {"type": "string", "maxLength": 255},
    "version": {"type": "string", "pattern": "^v[0-9]+$"},
    "status": {
      "type": "string",
      "enum": ["draft", "review", "approved", "deployed", "deprecated"]
    },
    "created_at": {"type": "string", "format": "date-time"},
    "approved_at": {"type": ["string", "null"], "format": "date-time"},
    "approved_by": {"type": ["array", "null"], "items": {"type": "string"}},
    "spec": {
      "type": "object",
      "required": ["tools", "budgets"],
      "properties": {
        "tools": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["name", "action"],
            "properties": {
              "name": {"type": "string"},
              "action": {"type": "string", "enum": ["allow", "warn", "block", "require_approval"]},
              "arg_schema": {"type": "object"},
              "output_schema": {"type": "object"},
              "conditions": {"type": "array", "items": {"type": "string"}}
            }
          }
        },
        "budgets": {
          "type": "object",
          "properties": {
            "max_tool_calls": {"type": "integer"},
            "max_wall_clock_sec": {"type": "integer"},
            "max_retries": {"type": "integer"},
            "max_bytes_egressed": {"type": "integer"}
          }
        },
        "egress_controls": {
          "type": "object",
          "properties": {
            "domain_allowlist": {"type": "array", "items": {"type": "string"}},
            "domain_denylist": {"type": "array", "items": {"type": "string"}},
            "block_private_ips": {"type": "boolean", "default": true}
          }
        },
        "redaction": {
          "type": "object",
          "properties": {
            "patterns": {"type": "array", "items": {"type": "string"}},
            "mask_strategy": {"type": "string", "enum": ["hash", "redact", "truncate"]}
          }
        }
      }
    },
    "spec_hash": {
      "type": "string",
      "pattern": "^[a-f0-9]{64}$",
      "description": "SHA256(canonical_json(spec))"
    }
  }
}
```

---

## 3. Deterministic ID Scheme

### 3.1 ULID v2 Generation
- Use `github.com/oklog/ulid/v2` (Go) or `ulid` (Python/TS)
- Timestamp component: Unix milliseconds
- Randomness: Cryptographically secure RNG

### 3.2 ID Namespaces
- Runs: `run_*`
- Steps: `step_*`
- ToolCalls: `tc_*`
- Policies: `pol_*`
- Events: `evt_*`

---

## 4. Hashing Scheme

### 4.1 Canonical JSON
Before hashing, JSON is canonicalized per RFC 8785:
- Keys sorted lexicographically
- No whitespace
- Unicode normalization (NFC)

Implementation: Use `github.com/gibson042/canonicaljson-go`

### 4.2 Event Hash Calculation
```
event_hash = SHA256(canonical_json(event_without_hash_field) || prev_hash)
```

First event in chain: `prev_hash = null` → use empty string for concat.

### 4.3 Run Evidence Hash
```
evidence_hash = SHA256(last_event_hash || policy_spec_hash || run_outcome_canonical)
```

---

## 5. Signature Scheme

### 5.1 Ed25519 Signing
- Key generation: `crypto/ed25519` (Go) or `PyNaCl` (Python)
- Sign: `signature = ed25519.sign(private_key, evidence_hash)`
- Verify: `ed25519.verify(public_key, signature, evidence_hash)`

### 5.2 Key Management
- Keys stored in database with `key_id` (ULID)
- Rotation: New key generated, old key marked `deprecated` (not deleted)
- Bundles include `signer_key_id` reference

---

## 6. Event Ordering Rules

### 6.1 Sequence Numbers
- Per-run monotonic counter
- First event: `seq_no = 0`
- Gaps not allowed

### 6.2 Timestamp Guarantees
- Events within same run: `timestamp[n] <= timestamp[n+1]`
- Clock skew tolerance: 5 seconds
- If wall-clock reverses, event rejected

### 6.3 Concurrency
- Single writer per run (database-enforced via advisory locks)
- Tool calls can be concurrent, but events serialized

---

## 7. Redaction Rules

### 7.1 What is Redacted
- Email addresses
- Phone numbers (US/International formats)
- Credit card numbers
- API keys/tokens (heuristic patterns)
- SSN patterns
- Custom regex patterns from policy

### 7.2 When Redaction Occurs
- At ingestion (before database write)
- Original never logged/stored
- Redacted fields marked with `[REDACTED:<TYPE>]`

### 7.3 Masking Strategies
- `hash`: SHA256(value) truncated to 8 chars
- `redact`: Replace with `[REDACTED]`
- `truncate`: Show first 4 chars + `****`

---

## 8. Policy Evaluation Guarantees

### 8.1 Policy Version Immutability
- Once `approved`, policy spec becomes immutable
- Edits create new version
- Run always references exact `policy_id:version` pair

### 8.2 "Policy in Effect at Time T"
Query: Given `run_id`, determine policy version used.
```sql
SELECT policy_id, version FROM runs WHERE run_id = ?
```
Guarantee: Policy spec at that version cannot change retroactively.

### 8.3 Evaluation Determinism
For same `(policy, tool_call)` pair:
- Same inputs → same decision
- No hidden state
- No wall-clock dependency (except budgets)

---

## 9. Schema Versioning

### 9.1 Breaking Changes
Require major version bump (e.g., `v1` → `v2`).

### 9.2 Backward Compatibility
API must support `N` and `N-1` schema versions simultaneously for 6 months.

### 9.3 Migration Path
Old runs/events never migrated. Queries handle multi-version data.

---

## 10. Consistency Checklist

Before emission, verify:
- [ ] All Go structs match JSON schemas field-for-field
- [ ] SDK event payloads match `Event.payload` structure
- [ ] Verifier CLI uses identical hashing/signature logic
- [ ] UI displays all fields defined in contracts
- [ ] Database migrations enforce all `required` fields as `NOT NULL`
- [ ] Policy compiler validates against `PolicySpec` schema
- [ ] Evidence bundle format includes all mandatory files

---

## 11. Reference Implementations

### 11.1 Canonical JSON (Go)
```go
import "github.com/gibson042/canonicaljson-go"

func CanonicalHash(v interface{}) (string, error) {
    canonical, err := canonicaljson.Marshal(v)
    if err != nil {
        return "", err
    }
    hash := sha256.Sum256(canonical)
    return hex.EncodeToString(hash[:]), nil
}
```

### 11.2 Event Hash Chaining (Go)
```go
func ComputeEventHash(event *Event) string {
    eventCopy := *event
    eventCopy.EventHash = "" // exclude from hash
    
    canonical, _ := canonicaljson.Marshal(eventCopy)
    
    prevHashBytes := []byte{}
    if event.PrevHash != nil {
        prevHashBytes, _ = hex.DecodeString(*event.PrevHash)
    }
    
    toHash := append(canonical, prevHashBytes...)
    hash := sha256.Sum256(toHash)
    return hex.EncodeToString(hash[:])
}
```

### 11.3 Run Signature (Go)
```go
func SignRun(run *Run, privateKey ed25519.PrivateKey) (string, error) {
    evidenceHash, _ := hex.DecodeString(*run.EvidenceHash)
    signature := ed25519.Sign(privateKey, evidenceHash)
    return base64.StdEncoding.EncodeToString(signature), nil
}
```

---

## 12. Conformance Testing

Each implementation (API, Python SDK, TS SDK, Verifier) MUST pass:
- Schema validation suite (100+ test cases)
- Hash test vectors (exact match)
- Signature test vectors (exact match)
- Round-trip serialization (no data loss)

---

**End of CONTRACTS.md**
