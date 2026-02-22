# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-02-21

### Added

#### Core Platform
- **Tool Gateway** - Centralized gateway for all tool calls with hard policy enforcement
- **Policy Engine** - YAML/JSON policy compiler with CEL-like expression language
- **Evidence Ledger** - Tamper-evident, hash-chained event log for complete audit trails
- **OIDC Authentication** - OpenID Connect support with local mock provider for development
- **RBAC Authorization** - Role-based access control (viewer, developer, policy_admin, approver, org_admin)
- **Multi-tenant Isolation** - Organization-level data isolation with org_id enforcement

#### Policy Features
- Tool allow/deny lists with per-tool argument schema validation
- Output validators using JSONSchema and regex patterns
- Budget controls: max tool calls, max wall clock time, max retries, max bytes egressed
- Data egress controls with domain allowlist/denylist
- Automatic blocking of private IP ranges and metadata endpoints
- PII redaction patterns for email, phone, credit card, API keys, SSN
- Configurable masking strategies: hash, redact, truncate
- Approval workflows for high-risk tools
- Policy lifecycle: Draft → Review → Approved → Deployed
- Two-person approval option for high-risk policies
- Policy actions: ALLOW, WARN, REDACT, BLOCK, REQUIRE_APPROVAL, DEGRADE_MODE

#### Evidence & Verification
- Hash-chained events using SHA256 with canonical JSON
- Ed25519 digital signatures for run manifests
- Offline evidence bundle export (ZIP format)
- Standalone verifier CLI for offline verification
- Chain integrity validation
- Signature verification
- Policy immutability checks
- Redaction compliance verification

#### SDKs
- **Python SDK** (`aegisrun`)
  - `AegisRunClient` for API communication
  - `Run`, `Step`, `ToolCall` abstractions
  - Automatic event emission with timestamps and spans
  - Offline mode buffer for network interruptions
  - Type hints and async support
- **TypeScript SDK** (`@aegisrun/sdk`)
  - Full TypeScript support with strict typing
  - Promise-based API
  - Browser and Node.js compatibility
  - Automatic retry handling

#### Web UI
- **Run Explorer** - Filter by environment, tag, policy, time range, outcome
- **Run Timeline** - Visualize steps, tool calls, decisions, redactions, blocks
- **Policy Studio** - YAML editor with schema validation and linter
- **Version History** - Policy diff viewer
- **Approval Dashboard** - Approve/deny with comments and audit trail
- **Evidence Export** - Download bundles with verification status

#### Built-in Tool Executors
- `http_request` - Restricted HTTP client with egress controls
- `local_file_read` - Sandboxed file access with path allowlist
- `shell_exec` - Disabled by default, requires explicit policy approval
- Generic plugin interface for external tools via HTTP callbacks

#### Security
- PII minimization with redaction at ingestion
- Secrets detection before persistence
- SSRF prevention (blocks private IPs, metadata endpoints)
- Input validation on all endpoints
- Rate limiting on gateway endpoints
- Signed evidence keys with rotation plan

#### Observability
- Structured logging with correlation IDs
- Prometheus metrics endpoint
- OpenTelemetry trace support
- Run/step/tool call telemetry

#### Infrastructure
- Single-binary Go API server
- PostgreSQL for persistence and job queue (no external MQ required)
- Docker Compose for local development
- Kubernetes manifests with Kustomize
- Database migrations with version control

### Documentation
- [ARCHITECTURE.md](docs/ARCHITECTURE.md) - System architecture and C4 diagrams
- [CONTRACTS.md](docs/CONTRACTS.md) - Core data contracts and schemas
- [POLICY_DSL.md](docs/POLICY_DSL.md) - Policy language reference
- [EVIDENCE_FORMAT.md](docs/EVIDENCE_FORMAT.md) - Evidence bundle specification
- [DEPLOYMENT.md](docs/DEPLOYMENT.md) - Deployment guide
- [API_REFERENCE.md](docs/API_REFERENCE.md) - REST API documentation
- Architecture Decision Records (ADRs) for key design choices

[1.0.0]: https://github.com/aegisrun/aegisrun/releases/tag/v1.0.0
