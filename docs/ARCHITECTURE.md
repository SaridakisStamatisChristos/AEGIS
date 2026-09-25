# AegisRun System Architecture

**Application baseline:** v1.0.1  
**Evidence bundle format:** 1.0.0  
**Reviewed:** 2026-09-25

## 1. Purpose

AegisRun is a self-hosted control plane for enforcing policy around AI-agent tool use. Its central invariant is that tool execution is routed through a gateway that evaluates policy before execution and records the resulting decision/evidence.

The platform combines:

- hard tool-use enforcement;
- versioned policy specifications;
- runtime budgets and egress controls;
- redaction and approval decisions;
- OIDC/RBAC and tenant scoping;
- tamper-evident event chains and signed run evidence;
- offline evidence verification;
- observability and production-oriented deployment controls.

## 2. System context

```text
+------------------+
| Agent workload   |
| Python / TS SDK  |
+--------+---------+
         |
         v
+---------------------------+
| AegisRun API / Tool       |
| Gateway (Go + Chi)        |
+-------+-------------------+
        |
        +----> Policy compiler/evaluator
        |
        +----> Tool executor registry
        |
        +----> PostgreSQL
        |       runs / steps / calls / events /
        |       policies / approvals / keys
        |
        +----> Prometheus + OpenTelemetry
        |
        +----> Evidence bundle export
                    |
                    v
            +------------------+
            | Offline verifier |
            +------------------+

React UI ----------> AegisRun API
OIDC provider -----> bearer-token authentication
```

## 3. Core components

### 3.1 API and gateway

The API is a Go service using Chi. The main code areas are:

- `api/cmd/server/` — bootstrap and production configuration validation;
- `api/internal/server/` — routing, middleware and HTTP handlers;
- `api/internal/gateway/` — the central tool-call enforcement path;
- `api/internal/policy/` — policy compilation/evaluation;
- `api/internal/redaction/` — redaction logic;
- `api/internal/ledger/` — event hashing, signing and evidence bundling;
- `api/internal/store/` — PostgreSQL persistence;
- `api/internal/auth/` — OIDC verification, RBAC and org isolation;
- `api/internal/telemetry/` — Prometheus/OpenTelemetry instrumentation.

The API binary version is injected at build time and is used by startup logs, telemetry service-version metadata, and the public `/health` response.

### 3.2 Gateway enforcement flow

```text
authenticated request
      |
      v
load run + referenced policy
      |
      v
compile policy
      |
      v
validate budget / args / conditions / egress
      |
      v
policy decision
  | allow/warn/redact/degrade
  | block
  | require_approval
      |
      v
persist tool-call decision + event evidence
      |
      v
execute when permitted
      |
      v
persist/redact response + counters + event evidence
```

The gateway returns different HTTP statuses according to the policy result: normal allowed/warn/redact decisions return 200, block returns 403, and require-approval returns 202.

### 3.3 PostgreSQL

PostgreSQL is the persistence layer for runs, steps, tool calls, events, policies, approvals and signing-key metadata. Event records form an append-only hash chain per run.

The production-oriented Kubernetes manifests currently deploy PostgreSQL as a StatefulSet. Operators needing managed HA PostgreSQL should replace this persistence topology without changing the application contract.

### 3.4 SDKs

AegisRun contains Python and TypeScript SDKs. They model run/step/tool-call flows and emit lifecycle evidence to the API.

For v1.0.1, the SDK source can be installed/built directly from the repository. Public PyPI/npm publication is pending registry authentication setup.

### 3.5 Web UI

The React UI provides operator-facing views for runs, policies, approvals, evidence and aggregate statistics.

### 3.6 Evidence verifier

The standalone verifier consumes exported ZIP bundles without requiring the AegisRun server.

The current bundle includes:

- `manifest.json`;
- `events.jsonl`;
- `policy_snapshot.json`;
- `run.json`;
- `public_key.pem` when a signer key is available;
- `README.txt`.

See [EVIDENCE_FORMAT.md](EVIDENCE_FORMAT.md).

## 4. Security architecture

### 4.1 Authentication and authorization

All `/api/v1/*` routes require bearer-token authentication. Authorization is permission-based through RBAC. Tenant-scoped resource handlers also enforce organization isolation.

Production startup rejects unsafe/default configuration such as mock OIDC, default database credentials, disabled DB TLS, or invalid CORS/rate-limit settings.

### 4.2 Client-IP trust

AegisRun does not trust forwarding headers by default.

- `CLIENT_IP_MODE=remote_addr` uses the TCP peer address.
- `CLIENT_IP_MODE=xff_trusted_proxies` is accepted only with a positive exact `TRUSTED_PROXY_COUNT`.

The normalized client IP is shared by request logging and per-IP rate limiting, avoiding disagreement between security controls.

### 4.3 Container and Kubernetes baseline

Production-oriented manifests include:

- non-root workloads;
- read-only root filesystems where applicable;
- dropped privileges/capabilities;
- liveness/readiness probes;
- NetworkPolicy;
- HPA;
- PodDisruptionBudget.

The API runtime is Alpine 3.24 in v1.0.1.

## 5. Evidence integrity model

Each event records its previous event hash and its own hash. Completed run evidence contains an evidence/root hash and signature metadata.

The exported bundle embeds the signature fields in `manifest.json`; it does **not** use a separate `signature.json` file in the current format.

The verifier checks:

1. event-chain integrity;
2. run/policy evidence consistency;
3. Ed25519 signature validity when signing material is present.

The application release version and evidence bundle format are independent. AegisRun v1.0.1 continues to emit evidence bundle format 1.0.0.

## 6. Observability

- Prometheus metrics endpoint: `/metrics`;
- OpenTelemetry tracing;
- structured request/application logging;
- health endpoint: `/health`;
- readiness endpoint: `/ready`;
- Grafana/Prometheus operational assets under `ops/`.

## 7. Deployment architecture

### Development

`docker-compose.yml` starts PostgreSQL, the API, and UI with development-safe defaults. It is explicitly **not** the production configuration.

### Production-oriented Kubernetes

Canonical manifests live under `ops/k8s/`.

Production deployment uses already-published release image digests rather than rebuilding the release. Database migrations run in a one-shot Kubernetes Job from the candidate API image before rollout.

## 8. Release supply chain

The release path validates:

- SemVer tag shape;
- tagged commit ancestry on `main`;
- Python/TypeScript/UI version parity with the tag;
- Go vulnerability status;
- high/critical Trivy findings;
- SBOM generation.

Release image builds enable SBOM/provenance metadata and explicit GitHub attestations. The v1.0.1 GitHub Release contains verifier binaries, checksums and SBOM files.

PyPI/npm are separate distribution channels and currently require publishing authentication to be completed.

## 9. Current technology baseline

| Component | Current baseline |
|---|---|
| API language | Go module baseline 1.25.0; CI/release toolchain 1.27.1 |
| HTTP router | chi/v5 5.3.2 |
| Database | PostgreSQL 15+ |
| UI | React 18.2.x |
| UI build | Vite 7.3.x |
| CSS | Tailwind 3.4.x |
| TypeScript | 5.x |
| Release Node.js | 24.21.0 |
| Python SDK | Python 3.9+ |
| OpenTelemetry | 1.44.0 family |
| gRPC | 1.83.2 |
| API runtime image | Alpine 3.24 |
| Signing | Ed25519 |
| Event hashing | SHA-256 |

Dependency lockfiles and module manifests are authoritative if this table ever drifts.

## 10. Performance and scalability

The API is designed to be horizontally scalable; PostgreSQL remains the central persistence dependency. Performance targets in tests/runbooks should be treated as **release thresholds or engineering targets**, not universal benchmark guarantees. Real capacity depends on database sizing, policy complexity, tool latency and workload shape.

## 11. Related documentation

- [API_REFERENCE.md](API_REFERENCE.md)
- [CONTRACTS.md](CONTRACTS.md)
- [POLICY_DSL.md](POLICY_DSL.md)
- [EVIDENCE_FORMAT.md](EVIDENCE_FORMAT.md)
- [DEPLOYMENT.md](DEPLOYMENT.md)
- [RELEASE_CHECKLIST.md](RELEASE_CHECKLIST.md)
- [ROLLBACK_PLAYBOOK.md](ROLLBACK_PLAYBOOK.md)
