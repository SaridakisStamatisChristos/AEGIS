# AegisRun Postgres Backup & Restore Runbook

**Owner**: Platform / SRE team  
**Last Reviewed**: 2026-02-22  
**Review Cadence**: Quarterly (next: 2026-05-22)

---

## 1. Recovery Objectives

| Metric | Target | Notes |
|--------|--------|-------|
| **RPO** (Recovery Point Objective) | **≤ 1 hour** | Continuous WAL archival + hourly logical snapshots |
| **RTO** (Recovery Time Objective) | **≤ 30 minutes** | Measured from incident declaration to data serving |

---

## 2. Backup Strategy

### 2.1 Automated Logical Backups (Primary)

AegisRun ships a backup script at `ops/scripts/backup-postgres.sh` that uses `pg_dump` in custom format for fast parallel restore.

**Schedule** (configure via cron or Kubernetes CronJob):

```
# Hourly incremental-safe logical dumps — keep 48 copies
0 * * * *  /opt/aegisrun/ops/scripts/backup-postgres.sh hourly

# Daily full dump — keep 30 copies
0 2 * * *  /opt/aegisrun/ops/scripts/backup-postgres.sh daily

# Weekly full dump to off-site — keep 12 copies
0 3 * * 0  /opt/aegisrun/ops/scripts/backup-postgres.sh weekly
```

**Storage Targets**:
- Hourly/daily → local PVC or S3-compatible object store (`BACKUP_S3_BUCKET`)
- Weekly → separate off-site bucket in a different region/account

### 2.2 Continuous WAL Archiving (Production)

For production deployments requiring RPO < 1 hour, enable WAL archiving:

```ini
# postgresql.conf additions
archive_mode = on
archive_command = 'aws s3 cp %p s3://aegisrun-wal-archive/%f'
wal_level = replica
```

### 2.3 Kubernetes CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: aegisrun-db-backup
  namespace: aegisrun
spec:
  schedule: "0 * * * *"
  concurrencyPolicy: Forbid
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      activeDeadlineSeconds: 900
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: backup
              image: postgres:15-alpine
              command: ["/bin/sh", "/scripts/backup-postgres.sh", "hourly"]
              envFrom:
                - secretRef:
                    name: aegisrun-secrets
              env:
                - name: DB_HOST
                  value: aegisrun-postgres
                - name: BACKUP_DIR
                  value: /backups
              volumeMounts:
                - name: backup-storage
                  mountPath: /backups
                - name: scripts
                  mountPath: /scripts
          volumes:
            - name: backup-storage
              persistentVolumeClaim:
                claimName: aegisrun-backups
            - name: scripts
              configMap:
                name: aegisrun-backup-scripts
                defaultMode: 0755
```

---

## 3. Restore Procedures

### 3.1 Full Restore from Logical Backup

**When**: Total data loss, corruption, or disaster recovery.

```bash
# 1. Identify the backup to restore
ls -ltr /backups/daily/  # or: aws s3 ls s3://aegisrun-backups/daily/

# 2. Stop the API to prevent writes during restore
kubectl scale deployment aegisrun-api --replicas=0 -n aegisrun

# 3. Restore using the script
./ops/scripts/backup-postgres.sh restore /backups/daily/aegisrun_20260222_020000.dump

# 4. Verify data integrity (the script runs verification automatically)
# Manual check:
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
  SELECT 'runs' AS tbl, count(*) FROM runs
  UNION ALL SELECT 'events', count(*) FROM events
  UNION ALL SELECT 'policies', count(*) FROM policies;
"

# 5. Restart the API
kubectl scale deployment aegisrun-api --replicas=3 -n aegisrun

# 6. Run smoke tests
make smoke-test
```

### 3.2 Point-in-Time Recovery (PITR) with WAL

**When**: Need to recover to a precise moment (e.g., accidental data deletion).

```bash
# 1. Stop Postgres
kubectl scale statefulset aegisrun-postgres --replicas=0 -n aegisrun

# 2. Restore base backup
pg_basebackup -D /var/lib/postgresql/data/pgdata -Fp -Xs -P

# 3. Configure recovery target
cat > /var/lib/postgresql/data/pgdata/recovery.conf <<EOF
restore_command = 'aws s3 cp s3://aegisrun-wal-archive/%f %p'
recovery_target_time = '2026-02-22 14:30:00 UTC'
recovery_target_action = 'promote'
EOF

# 4. Start Postgres — it will replay WAL to the target time
kubectl scale statefulset aegisrun-postgres --replicas=1 -n aegisrun

# 5. Verify and restart API
```

### 3.3 Single-Table Restore

**When**: Only one table is corrupted or data needs selective recovery.

```bash
# 1. Restore to a temporary database
PGPASSWORD=$DB_PASSWORD pg_restore -h $DB_HOST -U $DB_USER \
  -d aegisrun_restore_tmp --create --clean \
  /backups/daily/aegisrun_20260222_020000.dump

# 2. Copy the needed table
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
  BEGIN;
  TRUNCATE TABLE events;
  INSERT INTO events SELECT * FROM dblink(
    'dbname=aegisrun_restore_tmp',
    'SELECT * FROM events'
  ) AS t(/* column list */);
  COMMIT;
"

# 3. Drop temporary database
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d postgres -c \
  "DROP DATABASE aegisrun_restore_tmp;"
```

---

## 4. Verification Cadence

| Verification | Frequency | Owner | Method |
|-------------|-----------|-------|--------|
| Backup job runs successfully | Daily (automated) | Monitoring | Prometheus alert: `aegisrun_backup_last_success_timestamp` |
| Restore to staging | **Weekly** | SRE on-call | Restore latest daily backup → run `make smoke-test` |
| Full DR rehearsal | **Monthly** | SRE team | Game-day drill (see `GAMEDAY_DRILL_TEMPLATE.md`) |
| Backup integrity check | Weekly (automated) | CI | `pg_restore --list` on latest backup |

### Automated Verification Script

The backup script includes a `verify` command:

```bash
# Verify latest backup is restorable without actually restoring
./ops/scripts/backup-postgres.sh verify /backups/daily/aegisrun_20260222_020000.dump
```

---

## 5. Monitoring & Alerts

The following Prometheus alerts fire when backups are unhealthy:

| Alert | Condition | Severity |
|-------|-----------|----------|
| `AegisRunBackupMissing` | No successful backup in 2 hours | **critical** |
| `AegisRunBackupFailed` | Backup job exit code ≠ 0 | **critical** |
| `AegisRunBackupSizeDrop` | Backup size dropped >50% vs. previous | **warning** |

---

## 6. Retention Policy

| Tier | Retention | Storage |
|------|-----------|---------|
| Hourly | 48 hours | Local PVC / primary S3 bucket |
| Daily | 30 days | Primary S3 bucket |
| Weekly | 12 weeks | Off-site S3 bucket (separate region) |

Old backups are pruned automatically by `backup-postgres.sh`.

---

## 7. Migration Rollback

When a database migration causes issues, see the dedicated [ROLLBACK_PLAYBOOK.md](ROLLBACK_PLAYBOOK.md) for migration-specific rollback procedures. Each migration in `api/migrations/` includes a corresponding `.down.sql` file.

---

## Appendix: Quick Reference

```bash
# Create an immediate backup
./ops/scripts/backup-postgres.sh manual

# List available backups
./ops/scripts/backup-postgres.sh list

# Restore latest daily backup
./ops/scripts/backup-postgres.sh restore latest

# Verify a backup file
./ops/scripts/backup-postgres.sh verify /path/to/backup.dump

# Check backup age (seconds since last success)
curl -s http://localhost:9090/api/v1/query?query=aegisrun_backup_last_success_timestamp | jq .
```
