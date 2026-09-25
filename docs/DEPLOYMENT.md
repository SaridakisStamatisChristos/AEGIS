# AegisRun Deployment Guide

**Application baseline:** v1.0.1  
**Reviewed:** 2026-09-25

## 1. Deployment modes

- **Development/local evaluation:** Docker Compose.
- **Production-oriented deployment:** Kubernetes manifests under `ops/k8s/`.

The root `docker-compose.yml` is development-only. Production startup rejects several development defaults.

## 2. Requirements

### Local Docker path

- Docker 24+
- Docker Compose 2.20+

### Source build path

- Go 1.25+; CI/release uses 1.27.1
- Node.js 24+ recommended; CI/release uses 24.21.0
- Python 3.9+ for the Python SDK
- PostgreSQL 15+ if not using Compose

### Production Kubernetes path

A conforming Kubernetes cluster plus persistent/external PostgreSQL, ingress/TLS, OIDC, Secrets, backup storage, and monitoring/alert receivers.

## 3. Local development

```bash
git clone https://github.com/SaridakisStamatisChristos/AEGIS.git
cd AEGIS
git checkout v1.0.1

cp .env.example .env
docker compose up --build -d

curl -f http://localhost:8080/health
curl -f http://localhost:8080/ready
```

Health:

```json
{
  "status": "ok",
  "version": "1.0.1"
}
```

Readiness:

```json
{
  "status": "ready",
  "checks": {
    "database": "healthy"
  }
}
```

## 4. Production configuration guardrails

When `APP_ENV=production`, configure at least:

```text
APP_ENV=production

DB_HOST=<host>
DB_PORT=5432
DB_USER=<user>
DB_PASSWORD=<non-default-secret>
DB_NAME=<database>
DB_SSL_MODE=require

OIDC_ISSUER=https://<issuer>
OIDC_CLIENT_ID=<client-id>
OIDC_CLIENT_SECRET=<secret>
OIDC_AUDIENCE=<audience>

CORS_ALLOW_ORIGIN=https://<ui-origin>
RATE_LIMIT_RPS=<positive-number>
RATE_LIMIT_BURST=<positive-integer>
```

### Client-IP trust

Safe default:

```text
CLIENT_IP_MODE=remote_addr
TRUSTED_PROXY_COUNT=0
```

Behind a known proxy chain only:

```text
CLIENT_IP_MODE=xff_trusted_proxies
TRUSTED_PROXY_COUNT=<exact-positive-hop-count>
```

## 5. Kubernetes deployment

Canonical files under `ops/k8s/` include namespace, API/UI workloads/services, HPA, PodDisruptionBudget, PostgreSQL, NetworkPolicy, ingress and ConfigMap.

Render:

```bash
kubectl kustomize ops/k8s
```

Apply only after environment-specific secrets/configuration are supplied:

```bash
kubectl apply -k ops/k8s
```

The checked-in manifests do not replace environment-specific secret management, storage-class selection, TLS/DNS, backups or identity-provider configuration.

## 6. Release images

The release workflow builds API, UI and verifier images and pushes them to GHCR under the normalized repository namespace.

Production deployment should reference immutable release digests. The deploy workflow resolves already-published API/UI release images and rewrites the canonical Kustomize image names before rollout instead of rebuilding the release.

Image visibility follows GitHub Packages settings.

## 7. Database migrations

The API image contains the pinned `golang-migrate` CLI.

Production migrations execute **before rollout** in a one-shot Kubernetes Job created from the candidate API image. The migration pod uses the API/PostgreSQL network path and deployment DB configuration.

For manual development:

```bash
migrate -path ./api/migrations \
  -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}" up
```

Never run destructive/down migrations in production without backup and rollback procedures.

## 8. Health and observability

| Endpoint | Purpose |
|---|---|
| `/health` | liveness + build version |
| `/ready` | dependency readiness |
| `/metrics` | Prometheus metrics |

OpenTelemetry export uses `OTEL_EXPORTER_OTLP_ENDPOINT`.

## 9. Security baseline

The production-oriented baseline includes non-root execution, restricted privileges, NetworkPolicy, probes, HPA/PDB controls, and an Alpine 3.24 API runtime.

## 10. Backups and recovery

Before production:

1. configure `ops/scripts/backup-postgres.sh`;
2. configure durable/off-site storage;
3. perform a restore test;
4. record RPO/RTO evidence;
5. verify rollback.

See [BACKUP_RESTORE_RUNBOOK.md](BACKUP_RESTORE_RUNBOOK.md) and [ROLLBACK_PLAYBOOK.md](ROLLBACK_PLAYBOOK.md).

## 11. Recommended release/deploy sequence

1. merge release prep with matching package versions;
2. normal CI + Security Scan green;
3. Release Gate green;
4. create SemVer tag on a commit contained in `main`;
5. run Release workflow;
6. verify preflight/security/SBOM/image build;
7. verify registry publication where configured;
8. verify GitHub Release assets;
9. resolve immutable image digests;
10. back up DB;
11. run migration Job;
12. deploy;
13. verify health/readiness/SLOs;
14. record evidence and rollback target.

## 12. v1.0.1 caveat

v1.0.1 successfully built/pushed release images, generated SBOMs/provenance and published a GitHub Release. PyPI/npm uploads did not complete because registry publishing authentication was absent.

## 13. Troubleshooting

### Production startup rejects configuration

Typical causes: mock/empty OIDC, default DB password, disabled DB TLS, wildcard/empty CORS, disabled rate limiting, or invalid client-IP trust configuration.

### /ready returns 503

Check PostgreSQL reachability, credentials, TLS, Service/NetworkPolicy and migration state.

### Migration Job fails

```bash
kubectl get jobs -n aegisrun
kubectl logs job/<migration-job> -n aegisrun
```

Do not continue rollout until the migration outcome is understood.

### Package publication fails

Treat PyPI/npm as independent distribution channels. Do not move/recreate an already-published SemVer tag simply to retry registry authentication.
