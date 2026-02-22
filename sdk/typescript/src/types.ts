/**
 * Type definitions for AegisRun SDK
 *
 * Matches the Go backend API /api/v1 response shapes.
 */

// --------------- Enums ---------------

export enum RunStatus {
  Running = 'running',
  Completed = 'completed',
  Failed = 'failed',
  Blocked = 'blocked',
  Cancelled = 'cancelled',
}

export enum StepStatus {
  Running = 'running',
  Completed = 'completed',
  Failed = 'failed',
}

/** Policy action values used in a Decision struct. */
export type PolicyAction = 'allow' | 'warn' | 'redact' | 'block' | 'require_approval' | 'degrade';

/** Policy lifecycle status */
export type PolicyStatus = 'draft' | 'review' | 'approved' | 'deployed' | 'deprecated';

// --------------- Request helpers ---------------

/** Structured policy reference matching the Go backend. */
export interface PolicyRef {
  policy_id: string;
  version: string;
}

/** Structured schema reference. */
export interface SchemaRef {
  schema_id: string;
  version: string;
}

// --------------- Response shapes ---------------

/** Gateway decision returned as a nested struct from the API. */
export interface Decision {
  action: PolicyAction;
  policy_rule_id: string;
  reason: string;
  approval_id?: string;
}

export interface RunCounters {
  steps: number;
  tool_calls: number;
  bytes_egressed: number;
  retries: number;
  blocks: number;
}

export interface RunResponse {
  run_id: string;
  org_id: string;
  parent_run_id?: string;
  policy_ref: PolicyRef;
  state_schema_ref?: SchemaRef;
  metadata: Record<string, any>;
  status: RunStatus;
  outcome?: Record<string, any>;
  counters: RunCounters;
  evidence_hash?: string;
  signature?: string;
  signer_key_id?: string;
  created_at: string;
  ended_at?: string;
}

export interface StepResponse {
  step_id: string;
  run_id: string;
  parent_step_id?: string;
  seq_no: number;
  name: string;
  state_vector: Record<string, any>;
  status: StepStatus;
  error?: string;
  started_at: string;
  ended_at?: string;
}

export interface EventResponse {
  event_id: string;
  run_id: string;
  seq_no: number;
  event_type: string;
  timestamp: string;
  payload: Record<string, any>;
  prev_hash?: string;
  event_hash: string;
}

export interface PolicyResponse {
  policy_id: string;
  org_id: string;
  name: string;
  version: string;
  status: string;
  created_at: string;
  approved_at?: string;
  approved_by?: string[];
  spec: Record<string, any>;
  spec_hash: string;
}

export interface ToolCallResponse {
  tool_call_id: string;
  decision: Decision;
  result?: any;
  error?: string;
}

// --------------- Backward-compat aliases ---------------

/** @deprecated Use RunResponse */
export type RunMetadata = RunResponse;
/** @deprecated Use StepResponse */
export type StepMetadata = StepResponse;
/** @deprecated Use ToolCallResponse */
export type ToolCallResult = ToolCallResponse;
