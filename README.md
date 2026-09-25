# AegisRun

**Production-grade control plane for AI-agent tool use**

AegisRun places a policy-enforcement gateway between an AI agent and the tools it can call. It combines policy-as-code, runtime budgets, approvals, redaction, tamper-evident evidence, offline verification, and operational controls in a self-hosted platform.

**Current application release:** `v1.0.1`  
**Evidence bundle format:** `1.0.0`  
**License:** Apache-2.0

## Why AegisRun

- **Hard tool enforcement** — allow, warn, redact, block, degrade, or return a require-approval decision before tool execution.
- **Policy-as-code** — versioned YAML/JSON policy specs with schema validation, conditions, budgets, egress controls, and redaction.
- **Tamper-evident evidence** — hash-chained events, signed run evidence, exportable bundles, and an independent verifier CLI.
- **Operational controls** — OIDC, RBAC, tenant isolation, rate limiting, Prometheus/OpenTelemetry instrumentation, health probes, HPA, network policy, backup/restore, canary and rollback tooling.
- **SDKs** — Python and TypeScript clients for instrumenting agent workflows.
- **Web UI** — run exploration, policy management, approvals, and evidence workflows.
- **Self-hosted** — Docker Compose for development and Kubernetes manifests for production-oriented deployment.

## Release status

AegisRun `v1.0.1` is published as a GitHub Release with:

- verifier binaries for Linux (amd64/arm64), macOS (amd64/arm64), and Windows (amd64);
- SHA-256 checksums;
- SPDX SBOMs for the API, UI, Python SDK, and TypeScript SDK;
- provenance attestations produced during the release pipeline;
- container images built and pushed by the release workflow.

The Python and TypeScript SDK source is usable directly from the repository. Registry publication to PyPI and npm is **not yet configured for v1.0.1**; the release jobs built and attested the packages but registry authentication was unavailable. See [Production Readiness](PRODUCTION_READINESS.md) and [Release Evidence Runbook](docs/RELEASE_EVIDENCE_RUNBOOK.md).

## Quick start

### Requirements

For the simplest local start:

- Docker 24+
- Docker Compose 2.20+

For source development:

- Go **1.25+** (the module baseline; CI/release currently uses Go 1.27.1)
- Node.js **24+** recommended (CI/release currently uses Node 24.21.0)
- Python **3.9+**

### Run locally

```bash
git clone https://github.com/SaridakisStamatisChristos/AEGIS.git
cd AEGIS
git checkout v1.0.1

cp .env.example .env
docker compose up --build -d

curl http://localhost:8080/health
curl http://localhost:8080/ready
```

The default Compose file is for **development only**. It intentionally permits development defaults such as mock OIDC and non-TLS database access. Production startup applies stricter validation and should use the Kubernetes deployment path.

### Run verification

```bash
make verify
```

For timestamped verification artifacts:

```bash
make verify-all
```

On Windows:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\verify-all.ps1
```

### Production-readiness score

```bash
bash ops/scripts/readiness-score.sh
bash ops/scripts/readiness-score.sh --json
```

The score is a structural repository check. It does **not** replace release evidence, registry publication checks, staging validation, or an operator sign-off.

## Using the release

### Offline verifier

Download the verifier binary for your platform from the `v1.0.1` GitHub Release, verify it against `checksums.txt`, then run:

```bash
./aegis-verify-linux-amd64 evidence.zip
```

### Python SDK from source

Until PyPI Trusted Publishing is configured:

```bash
python -m pip install ./sdk/python
```

### TypeScript SDK from source

Until npm publishing is configured:

```bash
cd sdk/typescript
npm ci
npm run build
```

## Architecture

```text
Agent / SDK
    |
    v
+----------------------+       +----------------------+
| AegisRun Tool Gateway| ----> | Policy Compiler /    |
| hard enforcement     |       | Evaluator            |
+----------+-----------+       +----------------------+
           |
           v
+----------------------+       +----------------------+
| Go API / Chi         | ----> | PostgreSQL           |
| OIDC + RBAC          |       | runs/events/policies |
+----------+-----------+       +----------------------+
           |
           +----> Prometheus / OpenTelemetry
           |
           +----> Evidence bundle ----> Offline verifier

React UI ----> Go API
```

See [System Architecture](docs/ARCHITECTURE.md) for component and security details.

## Repository layout

```text
AEGIS/
├── api/                  Go control-plane API and policy gateway
├── verifier/             Offline evidence verifier
├── sdk/
│   ├── python/
│   └── typescript/
├── ui/                   React frontend
├── ops/                  Kubernetes, monitoring and operational scripts
├── tests/                E2E and load tests
├── docs/                 Technical and operational documentation
└── artifacts/            Historical verification/drill/release evidence
```

## Policy example

A policy is created through the API as a named document whose `spec` contains the enforcement rules:

```json
{
  "name": "production-policy",
  "spec": {
    "tools": [
      {
        "name": "http_request",
        "action": "allow",
        "arg_schema": {
          "type": "object",
          "properties": {
            "url": { "type": "string" },
            "method": { "type": "string", "enum": ["GET", "POST"] }
          }
        },
        "conditions": ["args.url.startsWith('https://')"]
      },
      {
        "name": "shell_exec",
        "action": "block"
      }
    ],
    "budgets": {
      "max_tool_calls": 100,
      "max_wall_clock_sec": 300,
      "max_bytes_egressed": 10485760
    },
    "egress_controls": {
      "domain_allowlist": ["api.github.com"],
      "block_private_ips": true
    }
  }
}
```

See [Policy DSL Reference](docs/POLICY_DSL.md) for the full policy-spec model. **Current limitation:** a runtime `require_approval` tool decision is recorded/returned, but pending tool-call approval and resume execution are not yet implemented; the existing `/approvals` API is for policy-version approval.

## Evidence bundles

A current evidence bundle contains:

```text
manifest.json
events.jsonl
policy_snapshot.json
run.json
public_key.pem      # when a signing key is available
README.txt
```

The verifier checks the event hash chain, policy snapshot integrity, and signature information using the included public key. The bundle format remains version `1.0.0` in AegisRun `v1.0.1`.

See [Evidence Bundle Format](docs/EVIDENCE_FORMAT.md).

## Security and production behavior

Production configuration validates, among other things:

- non-mock OIDC configuration;
- non-default database credentials;
- database TLS mode;
- explicit CORS origin;
- enabled rate limiting;
- explicit client-IP trust configuration.

Forwarding headers are ignored by default. Deployments behind trusted proxies may opt into `CLIENT_IP_MODE=xff_trusted_proxies` with an exact positive `TRUSTED_PROXY_COUNT`.

The security pipeline includes dependency/vulnerability scanning, CodeQL, secret scanning, license checks, SBOM generation, and fail-closed high/critical release checks.

## Documentation

- [API Reference](docs/API_REFERENCE.md)
- [System Architecture](docs/ARCHITECTURE.md)
- [Core Contracts](docs/CONTRACTS.md)
- [Policy DSL Reference](docs/POLICY_DSL.md)
- [Evidence Bundle Format](docs/EVIDENCE_FORMAT.md)
- [Deployment Guide](docs/DEPLOYMENT.md)
- [Production Readiness](PRODUCTION_READINESS.md)
- [Production Roadmap](PRODUCTION_ROADMAP.md)
- [Release Checklist](docs/RELEASE_CHECKLIST.md)
- [Release Evidence Runbook](docs/RELEASE_EVIDENCE_RUNBOOK.md)
- [Backup & Restore Runbook](docs/BACKUP_RESTORE_RUNBOOK.md)
- [Rollback Playbook](docs/ROLLBACK_PLAYBOOK.md)
- [On-Call Escalation Policy](docs/ONCALL_ESCALATION_POLICY.md)
- [Game-Day Drill Template](docs/GAMEDAY_DRILL_TEMPLATE.md)

## Development

```bash
# API
cd api
go test ./...
go run ./cmd/server

# Verifier
cd ../verifier
go test ./...

# UI
cd ../ui
npm ci
npm run test -- --run
npm run build

# TypeScript SDK
cd ../sdk/typescript
npm ci
npm test
npm run build
```

## License

Apache-2.0. See [LICENSE](LICENSE).
