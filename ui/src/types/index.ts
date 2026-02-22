// ============================================
// Run Types (matches Go RunResponse)
// ============================================

export interface Run {
  run_id: string;
  org_id: string;
  parent_run_id?: string;
  policy_ref: PolicyRef;
  state_schema_ref?: SchemaRef;
  metadata: Record<string, any>;
  created_at: string;
  ended_at?: string;
  status: RunStatus;
  outcome?: RunOutcome;
  counters: RunCounters;
  evidence_hash?: string;
  signature?: string;
}

export interface PolicyRef {
  policy_id: string;
  version: string;
}

export interface SchemaRef {
  schema_id: string;
  version: string;
}

export type RunStatus = 'running' | 'completed' | 'failed' | 'blocked' | 'cancelled';

export interface RunOutcome {
  result?: any;
  error?: string;
  exit_reason: string;
}

export interface RunCounters {
  steps: number;
  tool_calls: number;
  bytes_egressed: number;
  retries: number;
  blocks: number;
}

// ============================================
// Step Types (matches Go StepResponse)
// ============================================

export interface Step {
  step_id: string;
  run_id: string;
  parent_step_id?: string;
  seq_no: number;
  name: string;
  state_vector: Record<string, any>;
  started_at: string;
  ended_at?: string;
  status: StepStatus;
  error?: string;
}

export type StepStatus = 'running' | 'completed' | 'failed';

// ============================================
// Event Types (matches Go EventResponse)
// ============================================

export interface Event {
  event_id: string;
  run_id: string;
  seq_no: number;
  event_type: string;
  timestamp: string;
  payload: Record<string, any>;
  prev_hash?: string;
  event_hash: string;
}

// ============================================
// Tool Call Types (matches Go ToolCall)
// ============================================

export interface ToolCall {
  tool_call_id: string;
  run_id: string;
  step_id: string;
  seq_no: number;
  tool_name: string;
  args: Record<string, any>;
  args_redacted: boolean;
  requested_at: string;
  responded_at?: string;
  decision: Decision;
  response?: ToolResponse;
  response_redacted: boolean;
  metadata: ToolCallMetadata;
}

export interface Decision {
  action: PolicyAction;
  policy_rule_id: string;
  reason: string;
  approval_id?: string;
}

export type PolicyAction = 'allow' | 'warn' | 'redact' | 'block' | 'require_approval' | 'degrade';

export interface ToolResponse {
  result?: any;
  error?: string;
  duration_ms: number;
}

export interface ToolCallMetadata {
  executor: string;
  retry_count: number;
}

// ============================================
// Policy Types (matches Go PolicyResponse)
// ============================================

export interface Policy {
  policy_id: string;
  org_id: string;
  name: string;
  version: string;
  status: PolicyStatus;
  created_at: string;
  approved_at?: string;
  approved_by?: string[];
  spec: PolicySpec;
  spec_hash: string;
}

export type PolicyStatus = 'draft' | 'review' | 'approved' | 'deployed' | 'deprecated';

export interface PolicySpec {
  tools: ToolPolicy[];
  budgets: Budgets;
  egress_controls?: EgressControls;
  redaction?: RedactionConfig;
}

export interface ToolPolicy {
  name: string;
  action: PolicyAction;
  arg_schema?: Record<string, any>;
  output_schema?: Record<string, any>;
  conditions?: string[];
}

export interface Budgets {
  max_tool_calls?: number;
  max_wall_clock_sec?: number;
  max_retries?: number;
  max_bytes_egressed?: number;
}

export interface EgressControls {
  domain_allowlist?: string[];
  domain_denylist?: string[];
  block_private_ips: boolean;
}

export interface RedactionConfig {
  patterns: string[];
  mask_strategy: 'hash' | 'redact' | 'truncate';
}

// ============================================
// Approval Types (matches Go ApprovalResponse)
// ============================================

export interface Approval {
  approval_id: string;
  policy_id: string;
  version: string;
  approver_id: string;
  decision: ApprovalDecision;
  comment?: string;
  created_at: string;
}

export type ApprovalDecision = 'approved' | 'rejected';

// UI-only status type (includes 'pending' for policies awaiting review)
export type ApprovalStatus = 'pending' | 'approved' | 'rejected';

// ============================================
// Evidence Types (matches Go verify response)
// ============================================

export interface VerifyResponse {
  run_id: string;
  chain_valid: boolean;
  message?: string;
}
