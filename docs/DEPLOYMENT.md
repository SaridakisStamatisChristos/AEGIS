# AegisRun Deployment Guide

**Version**: 1.0.0  
**Last Updated**: 2026-02-03

---

## 1. Overview

AegisRun can be deployed in multiple configurations:

- **Local Development**: Docker Compose
- **Single Node**: Binary + PostgreSQL
- **Kubernetes**: Production-grade HA deployment

---

## 2. Prerequisites

### 2.1 System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU | 2 cores | 4+ cores |
| Memory | 4 GB | 8+ GB |
| Disk | 20 GB SSD | 100+ GB SSD |
| Network | 100 Mbps | 1 Gbps |

### 2.2 Software Dependencies

| Software | Version | Purpose |
|----------|---------|---------|
| Docker | 24.0+ | Container runtime |
| Docker Compose | 2.20+ | Local orchestration |
| PostgreSQL | 15+ | Primary database |
| Go | 1.23.5+ | API compilation (if building) |
| Node.js | 22+ | UI compilation (if building) |

---

## 3. Local Development (Docker Compose)

### 3.1 Quick Start

```bash
# Clone repository
git clone https://github.com/aegisrun/aegis.git
cd aegis

# Start all services
docker compose up -d

# Verify services
docker compose ps
```

### 3.2 docker-compose.yml

```yaml
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: aegis
      POSTGRES_PASSWORD: aegis_local_dev
      POSTGRES_DB: aegis
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U aegis"]
      interval: 5s
      timeout: 5s
      retries: 5

  api:
    build:
      context: ./api
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://aegis:aegis_local_dev@postgres:5432/aegis?sslmode=disable
      JWT_SECRET: local-development-secret-change-in-prod
      SIGNING_KEY_PATH: /keys/signing.key
      LOG_LEVEL: debug
      CORS_ORIGINS: http://localhost:5173
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - ./keys:/keys:ro

  ui:
    build:
      context: ./ui
      dockerfile: Dockerfile
    ports:
      - "5173:80"
    environment:
      VITE_API_URL: http://localhost:8080
    depends_on:
      - api

  verifier:
    build:
      context: ./verifier
      dockerfile: Dockerfile
    ports:
      - "8081:8081"
    environment:
      PUBLIC_KEY_PATH: /keys/signing.pub

volumes:
  postgres_data:
```

### 3.3 Generate Signing Keys

```bash
# Generate Ed25519 key pair
mkdir -p keys
openssl genpkey -algorithm Ed25519 -out keys/signing.key
openssl pkey -in keys/signing.key -pubout -out keys/signing.pub
```

### 3.4 Access Services

| Service | URL | Purpose |
|---------|-----|---------|
| API | http://localhost:8080 | REST API |
| UI | http://localhost:5173 | Dashboard |
| Verifier | http://localhost:8081 | Evidence verification |
| PostgreSQL | localhost:5432 | Database (direct access) |

---

## 4. Environment Variables

### 4.1 API Server

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `JWT_SECRET` | Yes | - | JWT signing secret (32+ chars) |
| `SIGNING_KEY_PATH` | Yes | - | Path to Ed25519 private key |
| `LOG_LEVEL` | No | `info` | Log level: debug, info, warn, error |
| `LOG_FORMAT` | No | `json` | Log format: json, text |
| `PORT` | No | `8080` | HTTP server port |
| `CORS_ORIGINS` | No | `*` | Allowed CORS origins |
| `OIDC_ISSUER_URL` | Prod | - | OIDC provider issuer URL |
| `OIDC_CLIENT_ID` | Prod | - | OIDC client ID |
| `OIDC_CLIENT_SECRET` | Prod | - | OIDC client secret |
| `RATE_LIMIT_RPS` | No | `100` | Requests per second limit |
| `SHUTDOWN_TIMEOUT` | No | `30s` | Graceful shutdown timeout |

### 4.2 UI

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `VITE_API_URL` | Yes | - | API server URL |
| `VITE_OIDC_AUTHORITY` | Prod | - | OIDC authority URL |
| `VITE_OIDC_CLIENT_ID` | Prod | - | OIDC client ID |

### 4.3 Alertmanager (On-Call Escalation)

Use `ops/prometheus/alertmanager.env.example` as the baseline template for:

- `ALERTMANAGER_WEBHOOK_PRIMARY_URL`
- `ALERTMANAGER_WEBHOOK_SECONDARY_URL`
- `ALERTMANAGER_WEBHOOK_ESCALATION_URL`

Set environment-specific values through your deployment secret/config mechanism.

---

## 5. Database Setup

### 5.1 Create Database

```bash
# Connect to PostgreSQL
psql -U postgres

# Create database and user
CREATE USER aegis WITH PASSWORD 'secure_password_here';
CREATE DATABASE aegis OWNER aegis;
GRANT ALL PRIVILEGES ON DATABASE aegis TO aegis;
```

### 5.2 Run Migrations

```bash
# Using migrate CLI
migrate -path ./api/migrations -database "postgres://aegis:password@localhost:5432/aegis?sslmode=disable" up

# Or via API binary
./api migrate up
```

### 5.3 Migration Files

```
migrations/
├── 001_initial_schema.up.sql
├── 001_initial_schema.down.sql
├── 002_indexes.up.sql
├── 002_indexes.down.sql
├── 003_audit_triggers.up.sql
└── 003_audit_triggers.down.sql
```

---

## 6. Kubernetes Deployment

### 6.1 Prerequisites

- Kubernetes 1.28+
- kubectl configured
- Helm 3.12+ (optional)

### 6.2 Apply Manifests

```bash
# Create namespace
kubectl apply -f ops/k8s/namespace.yaml

# Apply ConfigMap and Secrets
kubectl apply -f ops/k8s/configmap.yaml
kubectl apply -f ops/k8s/secrets.yaml

# Deploy PostgreSQL
kubectl apply -f ops/k8s/postgres-statefulset.yaml
kubectl apply -f ops/k8s/postgres-service.yaml

# Deploy API
kubectl apply -f ops/k8s/api-deployment.yaml
kubectl apply -f ops/k8s/api-service.yaml
kubectl apply -f ops/k8s/api-hpa.yaml

# Deploy UI
kubectl apply -f ops/k8s/ui-deployment.yaml
kubectl apply -f ops/k8s/ui-service.yaml

# Apply network policies
kubectl apply -f ops/k8s/network-policy.yaml

# Apply PodDisruptionBudget
kubectl apply -f ops/k8s/pdb.yaml

# Apply Ingress
kubectl apply -f ops/k8s/ingress.yaml
```

### 6.3 Using Kustomize

```bash
kubectl apply -k ops/k8s/
```

### 6.4 Key Manifests

#### API Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aegis-api
  namespace: aegis
spec:
  replicas: 3
  selector:
    matchLabels:
      app: aegis-api
  template:
    metadata:
      labels:
        app: aegis-api
    spec:
      containers:
      - name: api
        image: aegisrun/api:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: aegis-secrets
              key: database-url
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: aegis-secrets
              key: jwt-secret
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
```

#### HorizontalPodAutoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: aegis-api-hpa
  namespace: aegis
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: aegis-api
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

#### Network Policy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: aegis-api-netpol
  namespace: aegis
spec:
  podSelector:
    matchLabels:
      app: aegis-api
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: postgres
    ports:
    - protocol: TCP
      port: 5432
```

---

## 7. TLS Configuration

### 7.1 Generate Certificates (Development)

```bash
# Generate self-signed certificate
openssl req -x509 -nodes -days 365 \
  -newkey rsa:2048 \
  -keyout tls.key \
  -out tls.crt \
  -subj "/CN=aegis.local"

# Create Kubernetes secret
kubectl create secret tls aegis-tls \
  --cert=tls.crt \
  --key=tls.key \
  -n aegis
```

### 7.2 Production (cert-manager)

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: aegis-tls
  namespace: aegis
spec:
  secretName: aegis-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
  - aegis.example.com
```

---

## 8. Monitoring

### 8.1 Prometheus Metrics

API exposes metrics at `/metrics`:

```bash
# Key metrics
aegis_http_requests_total
aegis_http_request_duration_seconds
aegis_tool_calls_total
aegis_policy_evaluations_total
aegis_policy_blocks_total
aegis_active_runs
aegis_db_connections_active
```

### 8.2 ServiceMonitor (Prometheus Operator)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: aegis-api
  namespace: aegis
spec:
  selector:
    matchLabels:
      app: aegis-api
  endpoints:
  - port: http
    path: /metrics
    interval: 15s
```

### 8.3 Grafana Dashboard

Import dashboard ID: `aegisrun-overview` (available at grafana.com/dashboards)

---

## 9. Health Checks

### 9.1 Endpoints

| Endpoint | Purpose | Healthy Response |
|----------|---------|------------------|
| `/health` | Liveness | `200 OK` |
| `/ready` | Readiness | `200 OK` + DB connected |
| `/metrics` | Prometheus | Metrics output |

### 9.2 Health Response

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "checks": {
    "database": "ok",
    "signing_key": "ok"
  }
}
```

---

## 10. Backup & Recovery

### 10.1 Database Backup

```bash
# Full backup
pg_dump -h localhost -U aegis -d aegis -F c -f aegis_backup.dump

# Restore
pg_restore -h localhost -U aegis -d aegis -c aegis_backup.dump
```

### 10.2 Key Backup

```bash
# Backup signing key (store securely!)
cp keys/signing.key /secure/backup/signing.key.$(date +%Y%m%d)

# Consider using HashiCorp Vault for production key management
```

---

## 11. Troubleshooting

### 11.1 Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| API won't start | Missing DATABASE_URL | Set environment variable |
| 401 Unauthorized | Invalid JWT_SECRET | Ensure consistent across instances |
| DB connection failed | PostgreSQL not ready | Check pg_isready, increase startup delay |
| Signature verification failed | Key mismatch | Verify public/private key pair |

### 11.2 Debug Logging

```bash
# Enable debug logging
export LOG_LEVEL=debug

# View API logs
docker compose logs -f api

# Kubernetes
kubectl logs -f deployment/aegis-api -n aegis
```

### 11.3 Database Connectivity

```bash
# Test connection
psql "postgres://aegis:password@localhost:5432/aegis?sslmode=disable" -c "SELECT 1"

# Check migrations
psql "postgres://aegis:password@localhost:5432/aegis" -c "SELECT * FROM schema_migrations"
```

---

## 12. Upgrade Procedures

### 12.1 Rolling Update (Kubernetes)

```bash
# Update image tag
kubectl set image deployment/aegis-api api=aegisrun/api:v1.1.0 -n aegis

# Monitor rollout
kubectl rollout status deployment/aegis-api -n aegis

# Rollback if needed
kubectl rollout undo deployment/aegis-api -n aegis
```

### 12.2 Database Migrations

```bash
# Run migrations before deploying new API version
migrate -path ./migrations -database "$DATABASE_URL" up

# Verify
migrate -path ./migrations -database "$DATABASE_URL" version
```

---

**End of DEPLOYMENT.md**
