-- AegisRun Initial Schema
-- Version: 1.0.0
-- Postgres 15+

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Organizations (multi-tenant isolation)
CREATE TABLE organizations (
    org_id VARCHAR(26) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_orgs_slug ON organizations(slug);

-- Users
CREATE TABLE users (
    user_id VARCHAR(26) PRIMARY KEY,
    org_id VARCHAR(26) NOT NULL REFERENCES organizations(org_id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    oidc_subject VARCHAR(512) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}'::jsonb,
    UNIQUE(org_id, email),
    UNIQUE(oidc_subject)
);

CREATE INDEX idx_users_org ON users(org_id);
CREATE INDEX idx_users_oidc ON users(oidc_subject);

-- Roles: viewer, developer, policy_admin, approver, org_admin
COMMENT ON COLUMN users.role IS 'RBAC role: viewer|developer|policy_admin|approver|org_admin';

-- Policies
CREATE TABLE policies (
    policy_id VARCHAR(26) PRIMARY KEY,
    org_id VARCHAR(26) NOT NULL REFERENCES organizations(org_id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(10) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    spec JSONB NOT NULL,
    spec_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(26) NOT NULL REFERENCES users(user_id),
    approved_at TIMESTAMPTZ,
    approved_by JSONB,
    deployed_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}'::jsonb,
    UNIQUE(org_id, name, version)
);

CREATE INDEX idx_policies_org ON policies(org_id);
CREATE INDEX idx_policies_status ON policies(status);
CREATE INDEX idx_policies_version ON policies(org_id, name, version);

COMMENT ON COLUMN policies.status IS 'draft|review|approved|deployed|deprecated';

-- Approvals
CREATE TABLE approvals (
    approval_id VARCHAR(26) PRIMARY KEY,
    policy_id VARCHAR(26) NOT NULL REFERENCES policies(policy_id) ON DELETE CASCADE,
    version VARCHAR(10) NOT NULL,
    approver_id VARCHAR(26) NOT NULL REFERENCES users(user_id),
    decision VARCHAR(20) NOT NULL,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_approvals_policy ON approvals(policy_id, version);
CREATE INDEX idx_approvals_approver ON approvals(approver_id);

COMMENT ON COLUMN approvals.decision IS 'approved|rejected';

-- Signing Keys
CREATE TABLE signing_keys (
    key_id VARCHAR(26) PRIMARY KEY,
    org_id VARCHAR(26) NOT NULL REFERENCES organizations(org_id) ON DELETE CASCADE,
    public_key BYTEA NOT NULL,
    private_key BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_keys_org ON signing_keys(org_id);
CREATE INDEX idx_keys_status ON signing_keys(org_id, status);

COMMENT ON COLUMN signing_keys.status IS 'active|deprecated';

-- Runs
CREATE TABLE runs (
    run_id VARCHAR(26) PRIMARY KEY,
    org_id VARCHAR(26) NOT NULL REFERENCES organizations(org_id) ON DELETE CASCADE,
    parent_run_id VARCHAR(26) REFERENCES runs(run_id),
    policy_id VARCHAR(26) NOT NULL REFERENCES policies(policy_id),
    policy_version VARCHAR(10) NOT NULL,
    state_schema_id VARCHAR(100),
    state_schema_version VARCHAR(10),
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'running',
    outcome JSONB,
    counters JSONB NOT NULL DEFAULT '{"steps":0,"tool_calls":0,"bytes_egressed":0,"retries":0,"blocks":0}'::jsonb,
    evidence_hash VARCHAR(64),
    signature TEXT,
    signer_key_id VARCHAR(26) REFERENCES signing_keys(key_id)
);

CREATE INDEX idx_runs_org ON runs(org_id);
CREATE INDEX idx_runs_status ON runs(status);
CREATE INDEX idx_runs_created ON runs(created_at DESC);
CREATE INDEX idx_runs_policy ON runs(policy_id, policy_version);
CREATE INDEX idx_runs_parent ON runs(parent_run_id);
CREATE INDEX idx_runs_metadata ON runs USING GIN(metadata);

COMMENT ON COLUMN runs.status IS 'running|completed|failed|blocked|cancelled';

-- Steps
CREATE TABLE steps (
    step_id VARCHAR(26) PRIMARY KEY,
    run_id VARCHAR(26) NOT NULL REFERENCES runs(run_id) ON DELETE CASCADE,
    parent_step_id VARCHAR(26) REFERENCES steps(step_id),
    seq_no INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    state_vector JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'running',
    error TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    UNIQUE(run_id, seq_no)
);

CREATE INDEX idx_steps_run ON steps(run_id, seq_no);
CREATE INDEX idx_steps_parent ON steps(parent_step_id);

COMMENT ON COLUMN steps.status IS 'running|completed|failed';

-- Tool Calls
CREATE TABLE tool_calls (
    tool_call_id VARCHAR(26) PRIMARY KEY,
    run_id VARCHAR(26) NOT NULL REFERENCES runs(run_id) ON DELETE CASCADE,
    step_id VARCHAR(26) NOT NULL REFERENCES steps(step_id) ON DELETE CASCADE,
    seq_no INTEGER NOT NULL,
    tool_name VARCHAR(255) NOT NULL,
    args JSONB NOT NULL,
    args_redacted BOOLEAN NOT NULL DEFAULT FALSE,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    responded_at TIMESTAMPTZ,
    decision JSONB NOT NULL,
    response JSONB,
    response_redacted BOOLEAN NOT NULL DEFAULT FALSE,
    metadata JSONB DEFAULT '{}'::jsonb,
    UNIQUE(run_id, seq_no)
);

CREATE INDEX idx_tool_calls_run ON tool_calls(run_id, seq_no);
CREATE INDEX idx_tool_calls_step ON tool_calls(step_id);
CREATE INDEX idx_tool_calls_tool ON tool_calls(tool_name);
CREATE INDEX idx_tool_calls_decision ON tool_calls USING GIN(decision);

-- Events (append-only ledger)
CREATE TABLE events (
    event_id VARCHAR(26) PRIMARY KEY,
    run_id VARCHAR(26) NOT NULL REFERENCES runs(run_id) ON DELETE CASCADE,
    seq_no INTEGER NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    payload JSONB NOT NULL,
    prev_hash VARCHAR(64),
    event_hash VARCHAR(64) NOT NULL UNIQUE,
    UNIQUE(run_id, seq_no)
);

CREATE INDEX idx_events_run ON events(run_id, seq_no);
CREATE INDEX idx_events_type ON events(event_type);
CREATE INDEX idx_events_timestamp ON events(timestamp DESC);
CREATE INDEX idx_events_hash_chain ON events(run_id, prev_hash);

COMMENT ON TABLE events IS 'Append-only tamper-evident event log with hash chaining';

-- Audit Log (for admin actions)
CREATE TABLE audit_log (
    audit_id VARCHAR(26) PRIMARY KEY,
    org_id VARCHAR(26) NOT NULL REFERENCES organizations(org_id) ON DELETE CASCADE,
    user_id VARCHAR(26) NOT NULL REFERENCES users(user_id),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(26),
    changes JSONB,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip_address INET,
    user_agent TEXT
);

CREATE INDEX idx_audit_org ON audit_log(org_id);
CREATE INDEX idx_audit_user ON audit_log(user_id);
CREATE INDEX idx_audit_timestamp ON audit_log(timestamp DESC);
CREATE INDEX idx_audit_resource ON audit_log(resource_type, resource_id);

-- Job Queue (Postgres-based)
CREATE TABLE jobs (
    job_id VARCHAR(26) PRIMARY KEY,
    org_id VARCHAR(26) NOT NULL REFERENCES organizations(org_id) ON DELETE CASCADE,
    job_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3
);

CREATE INDEX idx_jobs_status ON jobs(status, created_at);
CREATE INDEX idx_jobs_type ON jobs(job_type);

COMMENT ON COLUMN jobs.status IS 'pending|running|completed|failed';

-- Sessions (OIDC session management)
CREATE TABLE sessions (
    session_id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    data JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- Updated_at triggers
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER organizations_updated_at BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Advisory locks for run event serialization
COMMENT ON TABLE runs IS 'Use pg_advisory_lock(hashtext(run_id)) to serialize event writes per run';
