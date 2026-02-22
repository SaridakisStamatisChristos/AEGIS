# AegisRun Deploy Rollback Playbook

**Owner**: Platform / SRE team  
**Last Reviewed**: 2026-02-22  
**Review Cadence**: Quarterly

---

## 1. Overview

This playbook covers rollback procedures for failed deployments of the AegisRun platform. It addresses two scenarios:

| Scenario | Method | Target RTO |
|----------|--------|------------|
| **Image rollback** (application code regression) | Kubernetes deployment rollback | **< 5 minutes** |
| **Migration rollback** (database schema regression) | Down-migration + image rollback | **< 15 minutes** |

---

## 2. Pre-Conditions

Before any deploy reaches production:

- [ ] CI pipeline (build + test + security scan) is fully green
- [ ] Canary deployment passed health checks for ≥ 10 minutes
- [ ] Current image tag and migration version are recorded in deploy log
- [ ] Rollback has been tested in staging within the last 30 days

---

## 3. Image Rollback (No DB Changes)

**When**: Application is returning errors, latency spike, or functional regression after a deploy, but no database migration was run.

### 3.1 Kubernetes (Production)

```bash
# Step 1: Identify the current and previous revisions
kubectl rollout history deployment/aegisrun-api -n aegisrun

# Step 2: Roll back to the previous revision
kubectl rollout undo deployment/aegisrun-api -n aegisrun

# Step 3: Watch the rollout
kubectl rollout status deployment/aegisrun-api -n aegisrun --timeout=120s

# Step 4: Verify health
curl -sf https://api.aegisrun.example.com/health
curl -sf https://api.aegisrun.example.com/ready

# Step 5: Verify metrics (error rate should drop within 2 minutes)
# Check Grafana: SLO & Reliability dashboard → Error Rate panel
```

To roll back to a **specific** revision:

```bash
kubectl rollout undo deployment/aegisrun-api -n aegisrun --to-revision=<N>
```

### 3.2 UI Rollback

```bash
kubectl rollout undo deployment/aegisrun-ui -n aegisrun
kubectl rollout status deployment/aegisrun-ui -n aegisrun --timeout=120s
```

### 3.3 Docker Compose (Staging / Dev)

```bash
# Step 1: Tag the known-good image
export GOOD_TAG="v1.2.3"  # replace with last known-good tag

# Step 2: Update and restart
IMAGE_TAG=$GOOD_TAG docker compose up -d api

# Step 3: Verify
curl -sf http://localhost:8080/health && echo "OK"
```

---

## 4. Migration Rollback (DB Schema Change)

**When**: A database migration was applied as part of the deploy, and it must be reverted.

### 4.1 Pre-Flight Checks

```bash
# Identify the current migration version
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c \
  "SELECT version, dirty FROM schema_migrations;"

# Confirm the down-migration file exists
ls api/migrations/<VERSION>_*.down.sql
```

### 4.2 Rollback Steps

```bash
# Step 1: Scale down the API to stop writes
kubectl scale deployment aegisrun-api --replicas=0 -n aegisrun

# Step 2: Back up the database BEFORE rolling back
./ops/scripts/backup-postgres.sh manual

# Step 3: Apply the down-migration
# Using golang-migrate CLI:
migrate -path api/migrations/ -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}" down 1

# Or manually:
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME \
  -f api/migrations/<VERSION>_<name>.down.sql

# Step 4: Verify migration state
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c \
  "SELECT version, dirty FROM schema_migrations;"

# Step 5: Roll back the application image
kubectl rollout undo deployment/aegisrun-api -n aegisrun

# Step 6: Scale back up
kubectl scale deployment aegisrun-api --replicas=3 -n aegisrun

# Step 7: Verify
kubectl rollout status deployment/aegisrun-api -n aegisrun --timeout=120s
curl -sf https://api.aegisrun.example.com/ready
make smoke-test
```

### 4.3 Current Migrations Reference

| Version | Description | Has Down Migration |
|---------|-------------|--------------------|
| 001 | Initial schema | ✅ `001_initial_schema.down.sql` |
| 002 | Indexes | ✅ `002_indexes.down.sql` |
| 003 | Audit triggers | ✅ `003_audit_triggers.down.sql` |

> **Rule**: Every migration MUST include a `.down.sql` file. PRs without a down-migration are blocked by CI.

---

## 5. Canary Abort Procedure

**When**: A canary deployment is in progress and needs to be aborted before full rollout.

```bash
# Step 1: Pause the rollout
kubectl rollout pause deployment/aegisrun-api -n aegisrun

# Step 2: Check metrics — if error rate elevated, undo
kubectl rollout undo deployment/aegisrun-api -n aegisrun

# Step 3: Resume if it was paused but turns out healthy
# kubectl rollout resume deployment/aegisrun-api -n aegisrun
```

### Abort Thresholds

Automatically abort canary if any of these fire within 10 minutes of deploy:

| Metric | Abort Threshold |
|--------|-----------------|
| Error rate (5xx) | > 5% |
| P99 latency | > 2s |
| Readiness failures | Any pod not ready for > 60s |
| Pod restarts | > 2 restarts in canary pod |

These thresholds are encoded in the alert rules (`ops/prometheus/rules/aegisrun-alerts.yml`).

---

## 6. Post-Rollback Checklist

After any rollback, complete the following:

- [ ] Verify `/health` and `/ready` return 200
- [ ] Verify error rate is back to baseline (< 1%) on Grafana SLO dashboard
- [ ] Verify latest data created before the incident is still accessible
- [ ] Post incident message in #aegisrun-ops with:
  - What was deployed (image tag, migration version)
  - What broke (symptom + root cause if known)
  - What was rolled back to (image tag, migration version)
  - Time to detect, time to rollback
- [ ] Create follow-up issue for root cause analysis
- [ ] Update deploy log with rollback record

---

## 7. Communication Template

```
🔴 ROLLBACK EXECUTED — AegisRun API

Time: <UTC timestamp>
Environment: production
Deployed: <image-tag> (migration: <version>)
Rolled back to: <image-tag> (migration: <version>)
Reason: <brief description>
Impact: <user-facing impact>
Status: Healthy — smoke tests passing
Next steps: RCA scheduled
```

---

## 8. Escalation

| Severity | Who | When |
|----------|-----|------|
| Image rollback succeeds | On-call SRE | Notify in Slack |
| Image rollback fails | SRE + Platform lead | Page immediately |
| Migration rollback needed | SRE + DB owner | Page immediately |
| Data loss suspected | SRE + Platform lead + Engineering manager | Page immediately, invoke backup restore runbook |

---

## Related Documents

- [BACKUP_RESTORE_RUNBOOK.md](BACKUP_RESTORE_RUNBOOK.md) — Database backup and restore procedures
- [GAMEDAY_DRILL_TEMPLATE.md](GAMEDAY_DRILL_TEMPLATE.md) — Incident drill template
- `ops/prometheus/rules/aegisrun-alerts.yml` — Alert definitions
