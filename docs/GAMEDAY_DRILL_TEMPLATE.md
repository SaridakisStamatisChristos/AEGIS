# AegisRun Game-Day Incident Drill Template

**Owner**: Platform / SRE team  
**Last Reviewed**: 2026-02-22  
**Frequency**: Monthly (minimum); quarterly full drill

---

## Purpose

Game-day drills validate that our runbooks, alerts, dashboards, and communication workflows actually work under realistic failure conditions. Each drill simulates a specific incident, measures response metrics, and captures improvement actions.

---

## Drill Structure

| Phase | Duration | Description |
|-------|----------|-------------|
| **Briefing** | 10 min | Announce drill, assign roles, confirm communication channels |
| **Injection** | — | Facilitator triggers the failure scenario |
| **Detection** | measured | Time until on-call acknowledges the alert |
| **Response** | measured | Time until mitigation/rollback is complete |
| **Verification** | 10 min | Confirm healthy state via smoke tests & dashboards |
| **Debrief** | 30 min | Structured retrospective (same day) |

---

## Roles

| Role | Responsibility |
|------|----------------|
| **Facilitator** | Designs scenario, injects failure, observes, keeps time |
| **Incident Commander (IC)** | Leads response, makes rollback/escalation decisions |
| **On-Call Responder** | Receives alerts, investigates, executes runbook |
| **Communications** | Posts status updates in #aegisrun-ops |
| **Observer(s)** | Takes notes on process gaps; does not intervene |

---

## Drill Scenarios

### Scenario 1: API Pod Crash Loop

**Objective**: Validate Kubernetes self-healing + alert detection  
**Failure Mode**: OOM kill or misconfigured readiness probe  
**Injection Method**:
```bash
# Patch the API deployment with an impossible memory limit
kubectl patch deployment aegisrun-api -n aegisrun -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"api","resources":{"limits":{"memory":"10Mi"}}}]}}}}'
```
**Expected Alerts**: `AegisRunAPIDown`, `AegisRunPodRestarting`  
**Success Criteria**:
- [ ] Alert fires within 2 minutes
- [ ] On-call acknowledges within 5 minutes
- [ ] IC decides to rollback within 3 minutes of acknowledgement
- [ ] Service restored within 10 minutes total
**Runbook**: [ROLLBACK_PLAYBOOK.md — Image Rollback](ROLLBACK_PLAYBOOK.md#3-image-rollback-no-db-changes)

---

### Scenario 2: Database Connection Exhaustion

**Objective**: Validate DB alert and connection pool runbook  
**Failure Mode**: Connection pool saturated (> 90%)  
**Injection Method**:
```bash
# Create many idle connections from a test pod
kubectl run dbstress --image=postgres:15-alpine --rm -it -- bash -c '
  for i in $(seq 1 80); do
    psql "postgres://${DB_USER}:${DB_PASSWORD}@postgres:5432/${DB_NAME}" -c "SELECT pg_sleep(600)" &
  done
  wait
'
```
**Expected Alerts**: `AegisRunDBConnectionPoolHigh`, `AegisRunDBQueryLatencyHigh`  
**Success Criteria**:
- [ ] Alert fires within 3 minutes
- [ ] Responder identifies connection source within 5 minutes
- [ ] Connections reclaimed within 10 minutes
- [ ] No data loss
**Runbook**: [BACKUP_RESTORE_RUNBOOK.md](BACKUP_RESTORE_RUNBOOK.md) (if data affected)

---

### Scenario 3: Failed Deploy with Bad Migration

**Objective**: Validate migration rollback playbook end-to-end  
**Failure Mode**: Migration applies but makes API fail  
**Injection Method** (staging only):
```bash
# Apply a deliberately broken migration
cat > /tmp/999_break_things.up.sql << 'EOF'
ALTER TABLE runs DROP COLUMN status;
EOF

migrate -path /tmp/ -database "$DATABASE_URL" up 1
```
**Expected Alerts**: `AegisRunHighErrorRate`, `AegisRunAPIDown`  
**Success Criteria**:
- [ ] Alert fires within 2 minutes
- [ ] IC invokes migration rollback playbook within 5 minutes
- [ ] Database rolled back cleanly
- [ ] API restored within 15 minutes total
- [ ] Pre-incident data verified intact
**Runbook**: [ROLLBACK_PLAYBOOK.md — Migration Rollback](ROLLBACK_PLAYBOOK.md#4-migration-rollback-db-schema-change)

---

### Scenario 4: Backup Restore Verification

**Objective**: Validate RTO target (< 30 min) for full Postgres restore  
**Failure Mode**: Simulated data loss  
**Injection Method** (staging only):
```bash
# 1. Take a fresh backup
./ops/scripts/backup-postgres.sh manual

# 2. Insert marker data
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c \
  "INSERT INTO runs (id, status) VALUES ('drill-marker-$(date +%s)', 'completed');"

# 3. Wipe the database (STAGING ONLY!)
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c \
  "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

# 4. Start timer — begin restore
./ops/scripts/backup-postgres.sh restore latest
```
**Expected Outcome**: Full restore completes within RTO  
**Success Criteria**:
- [ ] Restore completes in under 30 minutes
- [ ] All data up to the backup point is present
- [ ] Post-backup marker data is NOT present (confirms RPO boundary)
- [ ] Application reconnects and serves traffic
**Runbook**: [BACKUP_RESTORE_RUNBOOK.md — Full Restore](BACKUP_RESTORE_RUNBOOK.md)

---

### Scenario 5: Elevated Error Rate from Gateway Timeout

**Objective**: Validate error budget alert and escalation path  
**Failure Mode**: External tool/gateway returning timeouts  
**Injection Method**:
```bash
# Use a network policy to block egress from API pods temporarily
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: drill-block-egress
  namespace: aegisrun
spec:
  podSelector:
    matchLabels:
      app: aegisrun-api
  policyTypes:
  - Egress
  egress: []  # block all egress
EOF
```
**Expected Alerts**: `AegisRunGatewayErrorRate`, `AegisRunErrorBudgetBurning`  
**Success Criteria**:
- [ ] Gateway error rate alert fires within 5 minutes
- [ ] Responder identifies blocked egress within 10 minutes
- [ ] Network policy removed, traffic restored
- [ ] Error budget consumption quantified
**Cleanup**:
```bash
kubectl delete networkpolicy drill-block-egress -n aegisrun
```

---

## Drill Record Template

Copy and fill in for each drill:

```markdown
## Drill Record

**Date**: YYYY-MM-DD
**Scenario**: <number and title>
**Environment**: staging / production-canary
**Participants**:
- Facilitator: <name>
- IC: <name>
- On-Call: <name>
- Communications: <name>
- Observers: <names>

### Timeline

| Time (UTC) | Event |
|------------|-------|
| HH:MM | Failure injected |
| HH:MM | Alert fired |
| HH:MM | On-call acknowledged |
| HH:MM | IC assumed command |
| HH:MM | Mitigation started |
| HH:MM | Service restored |
| HH:MM | Verification complete |

### Metrics

| Metric | Target | Actual |
|--------|--------|--------|
| Time to detect | < 2 min | |
| Time to acknowledge | < 5 min | |
| Time to mitigate | varies | |
| Total downtime | varies | |

### Success Criteria Results

- [ ] Alert fired on time
- [ ] Correct runbook followed
- [ ] Communication updates sent
- [ ] Service restored within target
- [ ] No unintended side effects

### Findings

| # | Finding | Severity | Action Item | Owner | Due |
|---|---------|----------|-------------|-------|----|
| 1 | | | | | |
| 2 | | | | | |

### Debrief Notes

**What went well**:
-

**What could improve**:
-

**Runbook updates needed**:
-
```

---

## Scheduling

| Drill | Frequency | Next Scheduled | Environment |
|-------|-----------|----------------|-------------|
| API Pod Crash Loop | Monthly | — | Production (non-destructive) |
| DB Connection Exhaustion | Monthly | — | Staging |
| Failed Migration | Quarterly | — | Staging only |
| Backup Restore | Quarterly | — | Staging only |
| Gateway Timeout | Monthly | — | Staging |

---

## Related Documents

- [ROLLBACK_PLAYBOOK.md](ROLLBACK_PLAYBOOK.md) — Deploy rollback procedures
- [BACKUP_RESTORE_RUNBOOK.md](BACKUP_RESTORE_RUNBOOK.md) — Backup and restore runbook
- `ops/prometheus/rules/aegisrun-alerts.yml` — Alert rule definitions
- `ops/grafana/dashboards/slo-reliability-dashboard.json` — SLO monitoring dashboard
