#!/usr/bin/env bash
# AegisRun Database Initialization Script
# Initializes PostgreSQL with schema and required extensions

set -euo pipefail

# Configuration with defaults
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-aegisrun}"
DB_PASSWORD="${DB_PASSWORD:-aegisrun}"
DB_NAME="${DB_NAME:-aegisrun}"
DB_SSL_MODE="${DB_SSL_MODE:-disable}"
MIGRATIONS_PATH="${MIGRATIONS_PATH:-../api/migrations}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check for required tools
check_dependencies() {
    log_info "Checking dependencies..."
    
    if ! command -v psql &> /dev/null; then
        log_error "psql is not installed. Please install PostgreSQL client."
        exit 1
    fi
    
    log_success "All dependencies found."
}

# Wait for PostgreSQL to be ready
wait_for_postgres() {
    log_info "Waiting for PostgreSQL at ${DB_HOST}:${DB_PORT}..."
    
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d postgres -c '\q' 2>/dev/null; then
            log_success "PostgreSQL is ready."
            return 0
        fi
        
        attempt=$((attempt + 1))
        log_info "Waiting for PostgreSQL... (attempt ${attempt}/${max_attempts})"
        sleep 2
    done
    
    log_error "PostgreSQL is not available after ${max_attempts} attempts."
    exit 1
}

# Create database if it doesn't exist
create_database() {
    log_info "Creating database '${DB_NAME}' if it doesn't exist..."
    
    local db_exists
    db_exists=$(PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'")
    
    if [ "$db_exists" = "1" ]; then
        log_warn "Database '${DB_NAME}' already exists."
    else
        PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d postgres -c "CREATE DATABASE ${DB_NAME};"
        log_success "Database '${DB_NAME}' created."
    fi
}

# Install required extensions
install_extensions() {
    log_info "Installing required PostgreSQL extensions..."
    
    PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" <<EOF
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
EOF
    
    log_success "Extensions installed."
}

# Run migrations
run_migrations() {
    log_info "Running database migrations from ${MIGRATIONS_PATH}..."
    
    if [ ! -d "${MIGRATIONS_PATH}" ]; then
        log_error "Migrations directory not found: ${MIGRATIONS_PATH}"
        exit 1
    fi
    
    # Find and sort migration files
    local migration_files
    migration_files=$(find "${MIGRATIONS_PATH}" -name "*.up.sql" | sort)
    
    if [ -z "$migration_files" ]; then
        log_warn "No migration files found."
        return 0
    fi
    
    # Create migrations tracking table if not exists
    PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" <<EOF
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMPTZ DEFAULT NOW()
);
EOF
    
    for migration_file in $migration_files; do
        local version
        version=$(basename "$migration_file" | sed 's/_.*//')
        
        # Check if migration already applied
        local applied
        applied=$(PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -tAc "SELECT 1 FROM schema_migrations WHERE version='${version}'")
        
        if [ "$applied" = "1" ]; then
            log_info "Migration ${version} already applied, skipping..."
            continue
        fi
        
        log_info "Applying migration: ${migration_file}"
        
        PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -f "${migration_file}"
        
        PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -c "INSERT INTO schema_migrations (version) VALUES ('${version}');"
        
        log_success "Migration ${version} applied."
    done
    
    log_success "All migrations completed."
}

# Verify schema
verify_schema() {
    log_info "Verifying database schema..."
    
    local expected_tables=(
        "organizations"
        "users"
        "policies"
        "approvals"
        "signing_keys"
        "runs"
        "steps"
        "tool_calls"
        "events"
        "audit_log"
        "jobs"
        "sessions"
    )
    
    local missing_tables=()
    
    for table in "${expected_tables[@]}"; do
        local exists
        exists=$(PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -tAc "SELECT 1 FROM information_schema.tables WHERE table_name='${table}'")
        
        if [ "$exists" != "1" ]; then
            missing_tables+=("$table")
        fi
    done
    
    if [ ${#missing_tables[@]} -gt 0 ]; then
        log_error "Missing tables: ${missing_tables[*]}"
        exit 1
    fi
    
    log_success "Schema verification passed. All ${#expected_tables[@]} tables present."
}

# Print database info
print_info() {
    log_info "Database Information:"
    echo "  Host:     ${DB_HOST}"
    echo "  Port:     ${DB_PORT}"
    echo "  Database: ${DB_NAME}"
    echo "  User:     ${DB_USER}"
    echo "  SSL Mode: ${DB_SSL_MODE}"
    echo ""
    
    log_info "Table row counts:"
    PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" <<EOF
SELECT 
    schemaname,
    relname AS table_name,
    n_live_tup AS row_count
FROM pg_stat_user_tables
ORDER BY relname;
EOF
}

# Main execution
main() {
    echo ""
    echo "============================================"
    echo "  AegisRun Database Initialization"
    echo "============================================"
    echo ""
    
    check_dependencies
    wait_for_postgres
    create_database
    install_extensions
    run_migrations
    verify_schema
    print_info
    
    echo ""
    log_success "Database initialization completed successfully!"
    echo ""
}

# Run main function
main "$@"
