# AegisRun

**Production-Grade Agent Control Plane**

AegisRun provides hard enforcement of tool use for AI agents with policy-as-code, tamper-evident evidence ledger, and complete audit trails.

## Features

- **Hard Policy Enforcement**: Block, allow, redact, or require approval for tool calls
- **Policy-as-Code**: YAML/JSON policies with CEL-like expressions
- **Tamper-Evident Ledger**: Hash-chained event log with Ed25519 signatures
- **Offline Verification**: Export and verify evidence bundles independently
- **SDKs**: Python and TypeScript SDKs for instrumenting any agent
- **Web UI**: Run explorer, policy studio, approval workflows
- **Self-Hosted**: Full control with SSO + RBAC + multi-tenant isolation

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.23+ (for development)
- Node.js 20+ (for UI development)
- Python 3.9+ (for Python SDK)

### Running Locally

```bash
# Clone the repository
git clone https://github.com/aegisrun/aegisrun.git
cd aegisrun

# Copy environment file
cp .env.example .env

# Start all services
make docker-up

# Access the UI
open http://localhost:5173

# API available at
curl http://localhost:8080/health
```

### Running Tests

```bash
make test
```

### Building

```bash
make build
```

### Readiness Scoring

```bash
# Bash
bash ops/scripts/readiness-score.sh

# JSON output for CI/automation
bash ops/scripts/readiness-score.sh --json
```

```powershell
# Windows PowerShell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\readiness-score.ps1

# JSON output for CI/automation
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\readiness-score.ps1 -Json
```

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                      AegisRun System                         │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐      ┌──────────────┐                     │
│  │ Agent (Py/TS)│─────▶│ Tool Gateway │                     │
│  │ + SDK        │      │ (Policy Enf) │                     │
│  └──────────────┘      └──────┬───────┘                     │
│                               │                              │
│                               ▼                              │
│  ┌──────────────┐      ┌──────────────┐                     │
│  │ Web UI       │─────▶│ API Server   │                     │
│  │ (React)      │      │ (Go/Chi)     │                     │
│  └──────────────┘      └──────┬───────┘                     │
│                               │                              │
│                               ▼                              │
│                        ┌──────────────┐                     │
│                        │  PostgreSQL  │                     │
│                        │ (Ledger+Jobs)│                     │
│                        └──────────────┘                     │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         Evidence Verifier CLI (Offline)              │   │
│  └──────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

## Project Structure

```
aegisrun/
├── api/                    # Go API server
│   ├── cmd/server/         # Entry point
│   ├── internal/           # Internal packages
│   │   ├── gateway/        # Tool gateway + enforcement
│   │   ├── policy/         # Policy compiler + evaluator
│   │   ├── ledger/         # Evidence ledger + signing
│   │   ├── auth/           # OIDC + RBAC
│   │   └── store/          # Database layer
│   └── migrations/         # SQL migrations
├── sdk/
│   ├── python/             # Python SDK
│   └── typescript/         # TypeScript SDK
├── ui/                     # React frontend
├── verifier/               # Offline evidence verifier CLI
├── ops/                    # Kubernetes, Helm, Terraform
└── demo/                   # Demo scenarios
```

## SDK Usage

### Python

```python
from aegisrun import AegisRunClient, Run

client = AegisRunClient(base_url="http://localhost:8080")

run = Run(
    client=client,
    policy_ref={"policy_id": "my-policy", "version": "v1"},
    metadata={"environment": "production"}
).start()

result = run.step("fetch_data", {"task": "collect"}, lambda step: 
    step.tool_call("http_request", {"url": "https://api.example.com"})
)

run.end({"status": "success"})
```

### TypeScript

```typescript
import { AegisRunClient, Run } from '@aegisrun/sdk';

const client = new AegisRunClient({ baseUrl: 'http://localhost:8080' });

const run = await new Run({
  client,
  policyRef: { policy_id: 'my-policy', version: 'v1' }
}).start();

await run.step('fetch_data', {}, async (step) => {
  return step.toolCall('http_request', { url: 'https://api.example.com' });
});

run.end({ status: 'success' });
```

## Policy DSL

```yaml
# policy.yaml
name: production-policy
version: v1
spec:
  tools:
    - name: http_request
      action: allow
      conditions:
        - "args.url.startsWith('https://')"
      arg_schema:
        type: object
        properties:
          url: { type: string }
          method: { type: string, enum: [GET, POST] }
    
    - name: shell_exec
      action: block
      
    - name: db_query
      action: require_approval

  budgets:
    max_tool_calls: 100
    max_wall_clock_sec: 300
    max_bytes_egressed: 10485760

  egress_controls:
    domain_allowlist:
      - "*.example.com"
      - "api.github.com"
    block_private_ips: true

  redaction:
    patterns:
      - "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b"
      - "\\b\\d{4}[- ]?\\d{4}[- ]?\\d{4}[- ]?\\d{4}\\b"
    mask_strategy: redact
```

## Evidence Verification

```bash
# Export evidence bundle for a run
curl -o evidence.zip http://localhost:8080/api/v1/evidence/{run_id}/bundle

# Verify offline
./verifier/bin/aegis-verify evidence.zip

# Output:
# ✓ Event chain integrity verified (127 events)
# ✓ Policy immutability confirmed
# ✓ Ed25519 signature valid
# ✓ All redaction rules applied
```

## Development

### API Server

```bash
cd api
go run cmd/server/main.go
```

### UI

```bash
cd ui
npm install
npm run dev
```

### Database Migrations

```bash
make migrate
```

## Configuration

See [.env.example](.env.example) for all configuration options.

Key settings:

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | localhost |
| `DB_PORT` | PostgreSQL port | 5432 |
| `OIDC_ISSUER` | OIDC provider URL (or "mock") | mock |
| `LOG_LEVEL` | Logging level | info |

## Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [Contracts & Schemas](docs/CONTRACTS.md)
- [Policy DSL Reference](docs/POLICY_DSL.md)
- [Evidence Format](docs/EVIDENCE_FORMAT.md)
- [API Reference](docs/API_REFERENCE.md)
- [Deployment Guide](docs/DEPLOYMENT.md)
- [Backup & Restore Runbook](docs/BACKUP_RESTORE_RUNBOOK.md)
- [Release Checklist](docs/RELEASE_CHECKLIST.md)
- [Release Evidence Runbook](docs/RELEASE_EVIDENCE_RUNBOOK.md)

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.

## Security

For security issues, please email security@aegisrun.io.
