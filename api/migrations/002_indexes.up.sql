-- Additional performance indexes

-- Composite index for run filtering (common query pattern)
CREATE INDEX idx_runs_org_status_created ON runs(org_id, status, created_at DESC);

-- Policy lookup optimization
CREATE INDEX idx_policies_org_name_deployed ON policies(org_id, name) 
    WHERE status = 'deployed';

-- Event chain traversal optimization
CREATE INDEX idx_events_chain_traversal ON events(run_id, seq_no DESC, prev_hash);

-- Tool call analytics
CREATE INDEX idx_tool_calls_analytics ON tool_calls(tool_name, requested_at DESC)
    INCLUDE (decision);

-- Budget tracking (frequent counter updates)
CREATE INDEX idx_runs_counters ON runs USING GIN(counters);

-- Partial index for active runs only
CREATE INDEX idx_runs_active ON runs(org_id, created_at DESC)
    WHERE status = 'running';

-- Step hierarchy traversal
CREATE INDEX idx_steps_hierarchy ON steps(run_id, parent_step_id, seq_no);

-- Approval workflow optimization
CREATE INDEX idx_approvals_pending ON approvals(policy_id, created_at DESC)
    WHERE decision = 'pending';

-- Job queue worker optimization
CREATE INDEX idx_jobs_dequeue ON jobs(job_type, created_at)
    WHERE status = 'pending' AND retry_count < max_retries;

-- Audit trail search
CREATE INDEX idx_audit_search ON audit_log(org_id, action, timestamp DESC);
