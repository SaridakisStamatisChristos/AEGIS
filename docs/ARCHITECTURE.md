# AegisRun System Architecture

**Version**: 1.0.0  
**Last Updated**: 2026-02-03  
**Authors**: AegisRun Architecture Team

---

## 1. Overview

AegisRun is a production-grade Agent Control Plane that provides:
- Hard enforcement of tool use (not just logging)
- Policy-as-code with approvals, versioning, and audit trail
- Tamper-evident evidence ledger + offline verifiable evidence bundles
- Run/step/tool telemetry + operational dashboards
- Replay (best-effort deterministic) + forensic timeline UI
- SDKs for Python and TypeScript to instrument any agent workflow
- Self-hosted deployment with SSO + RBAC + multi-tenant isolation

---

## 2. C4-Lite Context Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                      AegisRun System                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐      ┌──────────────┐                   │
│  │ Agent (Py/TS)│─────>│ Tool Gateway │                   │
│  │ + SDK        │      │ (Policy Enf) │                   │
│  └──────────────┘      └──────┬───────┘                   │
│                               │                            │
│                               ▼                            │
│  ┌──────────────┐      ┌──────────────┐                   │
│  │ Web UI       │─────>│ API Server   │                   │
│  │ (React)      │      │ (Go/Chi)     │                   │
│  └──────────────┘      └──────┬───────┘                   │
│                               │                            │
│                               ▼                            │
│                        ┌──────────────┐                   │
│                        │  PostgreSQL  │                   │
│                        │ (Ledger+Jobs)│                   │
│                        └──────────────┘                   │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐ │
│  │         Evidence Verifier CLI (Offline)              │ │
│  └──────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘

External: OIDC Provider, Agent Workloads
```

---

## 3. Component Architecture

### 3.1 API Server (Go)
Single-binary monolith using Chi router:
- **cmd/server/main.go** - Entry point, configuration, dependency injection
- **internal/server/** - HTTP handlers, middleware, routing
- **internal/gateway/** - Tool call enforcement, budget tracking
- **internal/policy/** - Policy compiler, CEL parser, evaluator
- **internal/ledger/** - Event hashing, signing, bundling
- **internal/auth/** - OIDC integration, RBAC
- **internal/store/** - PostgreSQL repositories
- **internal/redaction/** - PII/secrets masking
- **internal/telemetry/** - Metrics, structured logging

### 3.2 Database (PostgreSQL)
Single database with these core tables:
- `organizations` - Multi-tenant isolation
- `users` - OIDC subjects + roles
- `policies` - Versioned policy specs
- `approvals` - Policy approval workflow
- `runs` - Agent execution sessions
- `steps` - Logical work units within runs
- `tool_calls` - Individual tool invocations
- `events` - Append-only ledger with hash chaining
- `signing_keys` - Ed25519 key pairs
- `audit_log` - Admin action trail

### 3.3 SDKs (Python + TypeScript)
Identical API surface:
```python
run = AegisRun.start_run(metadata, policy_ref, state_schema_ref)
run.step(name, state_vector, fn)  # Wrapper
run.tool_call(tool_name, args, executor_fn)  # Routes through gateway
```

Features:
- Automatic event emission (start/stop timestamps, spans, errors)
- Offline mode buffer (queue locally if server unavailable)
- Retry classification

### 3.4 Web UI (React)
Single-page application with:
- Run Explorer (filters, timeline view)
- Policy Studio (YAML editor, version history)
- Approvals (queue, audit trail)
- Evidence (export, verification status)

### 3.5 Evidence Verifier (CLI)
Offline tool for verifying evidence bundles:
```bash
aegis-verify bundle.zip
```
Checks: hash chain integrity, signatures, policy immutability, approvals.

---

## 4. Data Flow

### 4.1 Tool Call Flow
```
SDK → Gateway → Policy Evaluator → Tool Executor → Response
  │       │           │                │              │
  │       │           │                │              │
  └───────┴───────────┴────────────────┴──────────────┘
              │                    │
              ▼                    ▼
          Event Ledger        Run Counters
```

### 4.2 Event Chain
```
Event 0 (run.started)
    │
    ├── event_hash = SHA256(canonical_json || "")
    │
Event 1 (step.started)
    │
    ├── event_hash = SHA256(canonical_json || prev_hash)
    │
Event N (run.ended)
    │
    └── evidence_hash = SHA256(last_hash || policy_hash || outcome)
```

---

## 5. Security Architecture

### 5.1 Authentication
- OIDC-based (Auth0, Okta, Google)
- JWT tokens in Authorization header
- Session management via database

### 5.2 Authorization (RBAC)
Roles: `viewer`, `developer`, `policy_admin`, `approver`, `org_admin`

Permissions per role:
- `viewer`: Read runs, policies, evidence
- `developer`: + Create runs, export evidence
- `policy_admin`: + Create/edit policies
- `approver`: + Approve policies
- `org_admin`: Full access including user/key management

### 5.3 Multi-Tenant Isolation
- Every table includes `org_id`
- All queries filtered by authenticated user's org
- Database-level enforcement via row-level security (optional)

### 5.4 Data Protection
- PII redacted at ingestion (emails, phones, credit cards, API keys)
- Secrets never stored in plaintext
- Private IP ranges blocked by default (SSRF protection)

---

## 6. Technology Stack

| Component | Technology | Version |
|-----------|------------|---------|
| API Server | Go | 1.23.5 |
| Router | chi/v5 | 5.0.12 |
| Database | PostgreSQL | 15+ |
| Frontend | React | 18.2.0 |
| Build | Vite | 5.0.8 |
| CSS | Tailwind | 3.3.6 |
| Python SDK | Python | 3.9+ |
| TS SDK | TypeScript | 5.2+ |
| Signing | Ed25519 | crypto/ed25519 |
| Hashing | SHA256 | crypto/sha256 |

---

## 7. Deployment Architecture

### 7.1 Docker Compose (Development)
```yaml
services:
  - postgres (database)
  - api (Go server)
  - ui (React app via nginx)
```

### 7.2 Kubernetes (Production)
```
namespace: aegisrun
├── api-deployment (2+ replicas)
├── api-service (ClusterIP)
├── ui-deployment (2+ replicas)
├── ui-service (ClusterIP)
├── postgres-statefulset (1 replica + PVC)
├── postgres-service (ClusterIP)
├── ingress (TLS termination)
└── configmap/secrets
```

---

## 8. Scalability Considerations

### 8.1 Horizontal Scaling
- API server is stateless (scale horizontally)
- Database is single point (consider read replicas)
- Event ordering uses advisory locks per run

### 8.2 Performance Targets
- Gateway latency: P99 < 50ms
- Event ingestion: 10k events/sec
- Concurrent runs: 1000/node

### 8.3 Bottlenecks
- Database writes (mitigate: batch inserts, connection pooling)
- Policy compilation (mitigate: cache compiled policies)
- Evidence bundling (mitigate: async job queue)

---

## 9. Related Documents

- [CONTRACTS.md](CONTRACTS.md) - Data schemas and API contracts
- [POLICY_DSL.md](POLICY_DSL.md) - Policy language reference
- [EVIDENCE_FORMAT.md](EVIDENCE_FORMAT.md) - Bundle format specification
- [DEPLOYMENT.md](DEPLOYMENT.md) - Deployment guide
- [ADR/001-single-binary-api.md](ADR/001-single-binary-api.md) - Monolith decision
- [ADR/002-postgres-as-queue.md](ADR/002-postgres-as-queue.md) - No external MQ
- [ADR/003-ed25519-signing.md](ADR/003-ed25519-signing.md) - Crypto choice
- [ADR/004-cel-subset.md](ADR/004-cel-subset.md) - Expression language
