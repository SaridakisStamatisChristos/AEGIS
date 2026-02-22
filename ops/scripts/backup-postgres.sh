#!/usr/bin/env bash
#
# AegisRun PostgreSQL Backup & Restore Script
#
# Usage:
#   ./backup-postgres.sh <command> [options]
#
# Commands:
#   hourly      Run an hourly backup (retained 48h)
#   daily       Run a daily backup (retained 30d)
#   weekly      Run a weekly backup (retained 90d)
#   manual      Run a one-off backup (retained 90d)
#   restore     Restore from a backup file: restore <file|"latest">
#   verify      Verify the latest backup is restorable
#   list        List available backups
#   prune       Remove expired backups
#
# Environment variables:
#   DB_HOST          PostgreSQL host (default: localhost)
#   DB_PORT          PostgreSQL port (default: 5432)
#   DB_NAME          Database name (default: aegisrun)
#   DB_USER          Database user (default: aegisrun)
#   DB_PASSWORD      Database password (required)
#   BACKUP_DIR       Directory for backups (default: /var/backups/aegisrun)
#   PUSHGATEWAY_URL  Prometheus Pushgateway URL (optional; for metrics push)
#
set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────────────

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-aegisrun}"
DB_USER="${DB_USER:-aegisrun}"
DB_PASSWORD="${DB_PASSWORD:?DB_PASSWORD must be set}"

BACKUP_DIR="${BACKUP_DIR:-/var/backups/aegisrun}"
PUSHGATEWAY_URL="${PUSHGATEWAY_URL:-}"

# Retention periods (in days)
HOURLY_RETENTION=2
DAILY_RETENTION=30
WEEKLY_RETENTION=90
MANUAL_RETENTION=90

# ── Helpers ────────────────────────────────────────────────────────────────────

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
LOG_PREFIX="[aegisrun-backup]"

log()  { echo "${LOG_PREFIX} $(date -u +%H:%M:%SZ) $*"; }
err()  { echo "${LOG_PREFIX} $(date -u +%H:%M:%SZ) ERROR: $*" >&2; }
die()  { err "$@"; exit 1; }

ensure_dirs() {
    for subdir in hourly daily weekly manual; do
        mkdir -p "${BACKUP_DIR}/${subdir}"
    done
}

# ── Prometheus Metrics Push ────────────────────────────────────────────────────

push_metrics() {
    local status="$1"   # success | failure
    local size="$2"     # bytes (0 on failure)
    local duration="$3" # seconds

    if [[ -z "${PUSHGATEWAY_URL}" ]]; then
        return 0
    fi

    local success_ts=0
    if [[ "${status}" == "success" ]]; then
        success_ts="$(date +%s)"
    fi

    cat <<EOF | curl -s --max-time 5 --data-binary @- "${PUSHGATEWAY_URL}/metrics/job/aegisrun_backup/instance/${DB_HOST}" || true
# HELP aegisrun_backup_last_success_timestamp Unix timestamp of last successful backup.
# TYPE aegisrun_backup_last_success_timestamp gauge
aegisrun_backup_last_success_timestamp ${success_ts}
# HELP aegisrun_backup_size_bytes Size of the last backup in bytes.
# TYPE aegisrun_backup_size_bytes gauge
aegisrun_backup_size_bytes ${size}
# HELP aegisrun_backup_duration_seconds Duration of the last backup in seconds.
# TYPE aegisrun_backup_duration_seconds gauge
aegisrun_backup_duration_seconds ${duration}
# HELP aegisrun_backup_status Result of the last backup (1=success, 0=failure).
# TYPE aegisrun_backup_status gauge
aegisrun_backup_status $([ "${status}" == "success" ] && echo 1 || echo 0)
EOF
}

# ── Backup ─────────────────────────────────────────────────────────────────────

do_backup() {
    local tier="$1"  # hourly | daily | weekly | manual
    local dest="${BACKUP_DIR}/${tier}/${DB_NAME}_${tier}_${TIMESTAMP}.dump"

    log "Starting ${tier} backup → ${dest}"
    ensure_dirs

    local start_time
    start_time="$(date +%s)"

    if PGPASSWORD="${DB_PASSWORD}" pg_dump \
        -h "${DB_HOST}" \
        -p "${DB_PORT}" \
        -U "${DB_USER}" \
        -d "${DB_NAME}" \
        -Fc \
        --no-owner \
        --no-privileges \
        --verbose \
        -f "${dest}" 2>&1 | while IFS= read -r line; do log "  pg_dump: ${line}"; done; then

        local end_time size duration
        end_time="$(date +%s)"
        size="$(stat -c%s "${dest}" 2>/dev/null || stat -f%z "${dest}" 2>/dev/null || echo 0)"
        duration="$(( end_time - start_time ))"

        log "Backup complete: ${dest} (${size} bytes, ${duration}s)"
        push_metrics "success" "${size}" "${duration}"
    else
        local end_time duration
        end_time="$(date +%s)"
        duration="$(( end_time - start_time ))"

        err "Backup FAILED for tier=${tier}"
        push_metrics "failure" "0" "${duration}"
        return 1
    fi
}

# ── Restore ────────────────────────────────────────────────────────────────────

do_restore() {
    local target="$1"

    if [[ "${target}" == "latest" ]]; then
        # Find the most recent .dump across all tiers
        target="$(find "${BACKUP_DIR}" -name '*.dump' -type f -printf '%T@ %p\n' 2>/dev/null \
            | sort -rn | head -1 | awk '{print $2}')"
        if [[ -z "${target}" ]]; then
            die "No backup files found in ${BACKUP_DIR}"
        fi
        log "Resolved 'latest' → ${target}"
    fi

    if [[ ! -f "${target}" ]]; then
        die "Backup file not found: ${target}"
    fi

    log "╔═══════════════════════════════════════════════════════════════╗"
    log "║  WARNING: This will DROP and RECREATE the database.         ║"
    log "║  Database: ${DB_NAME} on ${DB_HOST}:${DB_PORT}              ║"
    log "║  Backup:   ${target}                                        ║"
    log "╚═══════════════════════════════════════════════════════════════╝"

    # Allow non-interactive use (e.g., from CronJob or drill scripts)
    if [[ -t 0 ]]; then
        read -r -p "Type 'yes' to proceed: " confirm
        if [[ "${confirm}" != "yes" ]]; then
            die "Restore aborted by user"
        fi
    fi

    log "Step 1/4: Terminating active connections…"
    PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d postgres -c \
        "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${DB_NAME}' AND pid <> pg_backend_pid();" \
        || log "  (no active connections to terminate)"

    log "Step 2/4: Dropping database…"
    PGPASSWORD="${DB_PASSWORD}" dropdb -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" --if-exists "${DB_NAME}"

    log "Step 3/4: Creating empty database…"
    PGPASSWORD="${DB_PASSWORD}" createdb -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" "${DB_NAME}"

    log "Step 4/4: Restoring from ${target}…"
    if PGPASSWORD="${DB_PASSWORD}" pg_restore \
        -h "${DB_HOST}" \
        -p "${DB_PORT}" \
        -U "${DB_USER}" \
        -d "${DB_NAME}" \
        --no-owner \
        --no-privileges \
        --verbose \
        "${target}" 2>&1 | while IFS= read -r line; do log "  pg_restore: ${line}"; done; then

        log "✓ Restore completed successfully"
    else
        # pg_restore returns non-zero on warnings (e.g., "already exists") which are often OK
        log "⚠ pg_restore exited with warnings — verify data manually"
    fi

    # Quick sanity check
    local row_count
    row_count="$(PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" \
        -tAc "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public';" 2>/dev/null || echo "?")"
    log "Post-restore: ${row_count} tables in public schema"
}

# ── Verify ─────────────────────────────────────────────────────────────────────

do_verify() {
    local latest
    latest="$(find "${BACKUP_DIR}" -name '*.dump' -type f -printf '%T@ %p\n' 2>/dev/null \
        | sort -rn | head -1 | awk '{print $2}')"

    if [[ -z "${latest}" ]]; then
        die "No backup files found in ${BACKUP_DIR}"
    fi

    log "Verifying backup: ${latest}"

    # Test that pg_restore can read the TOC without errors
    if PGPASSWORD="${DB_PASSWORD}" pg_restore --list "${latest}" > /dev/null 2>&1; then
        log "✓ Backup TOC is valid"
    else
        err "✗ Backup TOC is corrupted or unreadable"
        return 1
    fi

    # Check file is non-trivially small (< 1KB likely means empty/failed)
    local size
    size="$(stat -c%s "${latest}" 2>/dev/null || stat -f%z "${latest}" 2>/dev/null || echo 0)"
    if (( size < 1024 )); then
        err "✗ Backup is suspiciously small (${size} bytes)"
        return 1
    fi

    log "✓ Verification passed (${size} bytes, TOC readable)"
}

# ── List ───────────────────────────────────────────────────────────────────────

do_list() {
    log "Available backups in ${BACKUP_DIR}:"
    echo ""
    printf "%-10s  %-12s  %s\n" "TIER" "SIZE" "FILE"
    printf "%-10s  %-12s  %s\n" "----" "----" "----"

    for tier in hourly daily weekly manual; do
        local tier_dir="${BACKUP_DIR}/${tier}"
        if [[ -d "${tier_dir}" ]]; then
            find "${tier_dir}" -name '*.dump' -type f -printf "%T@ ${tier} %s %p\n" 2>/dev/null \
                | sort -rn \
                | while read -r _ t s f; do
                    local human_size
                    if (( s > 1073741824 )); then
                        human_size="$(awk "BEGIN{printf \"%.1fG\", ${s}/1073741824}")"
                    elif (( s > 1048576 )); then
                        human_size="$(awk "BEGIN{printf \"%.1fM\", ${s}/1048576}")"
                    elif (( s > 1024 )); then
                        human_size="$(awk "BEGIN{printf \"%.1fK\", ${s}/1024}")"
                    else
                        human_size="${s}B"
                    fi
                    printf "%-10s  %-12s  %s\n" "${t}" "${human_size}" "${f}"
                done
        fi
    done
    echo ""
}

# ── Prune ──────────────────────────────────────────────────────────────────────

do_prune() {
    log "Pruning expired backups…"
    ensure_dirs

    local total_removed=0

    prune_tier() {
        local tier="$1"
        local retention_days="$2"
        local tier_dir="${BACKUP_DIR}/${tier}"
        local count

        count="$(find "${tier_dir}" -name '*.dump' -type f -mtime "+${retention_days}" 2>/dev/null | wc -l)"
        if (( count > 0 )); then
            log "  Removing ${count} expired ${tier} backup(s) (> ${retention_days}d old)"
            find "${tier_dir}" -name '*.dump' -type f -mtime "+${retention_days}" -delete
            total_removed=$(( total_removed + count ))
        fi
    }

    prune_tier "hourly"  "${HOURLY_RETENTION}"
    prune_tier "daily"   "${DAILY_RETENTION}"
    prune_tier "weekly"  "${WEEKLY_RETENTION}"
    prune_tier "manual"  "${MANUAL_RETENTION}"

    log "Pruning complete: removed ${total_removed} file(s)"
}

# ── Main ───────────────────────────────────────────────────────────────────────

main() {
    local command="${1:-help}"
    shift || true

    case "${command}" in
        hourly)   do_backup hourly  && do_prune ;;
        daily)    do_backup daily   && do_prune ;;
        weekly)   do_backup weekly  && do_prune ;;
        manual)   do_backup manual ;;
        restore)  do_restore "${1:?Usage: $0 restore <file|latest>}" ;;
        verify)   do_verify ;;
        list)     do_list ;;
        prune)    do_prune ;;
        help|--help|-h)
            head -n 17 "$0" | tail -n +3 | sed 's/^# \?//'
            ;;
        *)
            die "Unknown command: ${command}. Run '$0 help' for usage."
            ;;
    esac
}

main "$@"
