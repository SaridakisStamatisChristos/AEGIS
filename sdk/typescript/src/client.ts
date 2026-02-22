/**
 * AegisRun API client
 *
 * All endpoints live under /api/v1.
 */

import axios, { AxiosInstance } from 'axios';
import {
  RunResponse,
  StepResponse,
  EventResponse,
  PolicyResponse,
  ToolCallResponse,
} from './types';

export interface ClientConfig {
  baseUrl?: string;
  apiToken?: string;
}

export class AegisRunClient {
  private client: AxiosInstance;

  constructor(config: ClientConfig = {}) {
    const baseUrl = config.baseUrl || 'http://localhost:8080';

    this.client = axios.create({
      baseURL: baseUrl,
      headers: config.apiToken
        ? { Authorization: `Bearer ${config.apiToken}` }
        : {},
    });
  }

  // ─── Runs ──────────────────────────────────────────────

  async createRun(params: {
    policyId: string;
    policyVersion: string;
    metadata?: Record<string, any>;
    parentRunId?: string;
    stateSchemaRef?: { schema_id: string; version: string };
  }): Promise<RunResponse> {
    const body: Record<string, any> = {
      policy_ref: {
        policy_id: params.policyId,
        version: params.policyVersion,
      },
      metadata: params.metadata || {},
    };
    if (params.parentRunId) body.parent_run_id = params.parentRunId;
    if (params.stateSchemaRef) body.state_schema_ref = params.stateSchemaRef;
    const response = await this.client.post('/api/v1/runs/', body);
    return response.data;
  }

  async getRun(runId: string): Promise<RunResponse> {
    const response = await this.client.get(`/api/v1/runs/${runId}/`);
    return response.data;
  }

  async listRuns(params?: {
    status?: string;
    limit?: number;
    offset?: number;
  }): Promise<RunResponse[]> {
    const response = await this.client.get('/api/v1/runs/', { params });
    if (Array.isArray(response.data)) {
      return response.data;
    }
    return response.data?.runs ?? [];
  }

  async listSteps(runId: string): Promise<StepResponse[]> {
    const response = await this.client.get(`/api/v1/runs/${runId}/steps`);
    if (Array.isArray(response.data)) {
      return response.data;
    }
    return response.data?.steps ?? [];
  }

  async listEvents(runId: string): Promise<EventResponse[]> {
    const response = await this.client.get(`/api/v1/runs/${runId}/events`);
    if (Array.isArray(response.data)) {
      return response.data;
    }
    return response.data?.events ?? [];
  }

  /**
   * Submit a hash-chained event to a run's ledger.
   *
   * @param runId     - The run to append the event to.
   * @param eventType - One of run.started, run.ended, step.started,
   *                    step.ended, state.updated.
   * @param payload   - Arbitrary JSON payload for the event.
   * @param timestamp - Optional ISO-8601 timestamp; server uses now() if omitted.
   */
  async submitEvent(params: {
    runId: string;
    eventType: string;
    payload?: Record<string, any>;
    timestamp?: string;
  }): Promise<EventResponse> {
    const body: Record<string, any> = { event_type: params.eventType };
    if (params.payload) body.payload = params.payload;
    if (params.timestamp) body.timestamp = params.timestamp;
    const response = await this.client.post(
      `/api/v1/runs/${params.runId}/events`,
      body
    );
    return response.data;
  }

  // ─── Gateway ───────────────────────────────────────────

  async executeToolCall(params: {
    run_id: string;
    step_id: string;
    tool_name: string;
    args: Record<string, any>;
    state_vector: Record<string, any>;
    metadata?: Record<string, any>;
    executor?: string;
  }): Promise<ToolCallResponse> {
    const response = await this.client.post('/api/v1/gateway/execute', {
      ...params,
      executor: params.executor || 'builtin',
      metadata: params.metadata || {},
    });
    return response.data;
  }

  // ─── Policies ──────────────────────────────────────────

  async listPolicies(params?: {
    status?: string;
  }): Promise<PolicyResponse[]> {
    const response = await this.client.get('/api/v1/policies/', { params });
    return response.data;
  }

  async createPolicy(body: {
    name: string;
    spec: Record<string, any>;
  }): Promise<PolicyResponse> {
    const response = await this.client.post('/api/v1/policies/', body);
    return response.data;
  }

  async getPolicy(
    policyId: string,
    version?: string
  ): Promise<PolicyResponse> {
    const response = await this.client.get(`/api/v1/policies/${policyId}/`, {
      params: version !== undefined ? { version } : undefined,
    });
    return response.data;
  }

  async updatePolicy(
    policyId: string,
    body: { spec: Record<string, any> }
  ): Promise<PolicyResponse> {
    const response = await this.client.put(
      `/api/v1/policies/${policyId}/`,
      body
    );
    return response.data;
  }

  async deletePolicy(policyId: string): Promise<void> {
    await this.client.delete(`/api/v1/policies/${policyId}/`);
  }

  async activatePolicy(policyId: string): Promise<PolicyResponse> {
    const response = await this.client.post(
      `/api/v1/policies/${policyId}/activate`
    );
    return response.data;
  }

  async deactivatePolicy(policyId: string): Promise<PolicyResponse> {
    const response = await this.client.post(
      `/api/v1/policies/${policyId}/deactivate`
    );
    return response.data;
  }

  // ─── Approvals ─────────────────────────────────────────

  async listApprovals(params?: {
    policy_id?: string;
    version?: string;
  }): Promise<any[]> {
    const response = await this.client.get('/api/v1/approvals/', { params });
    return response.data;
  }

  async getApproval(approvalId: string): Promise<any> {
    const response = await this.client.get(
      `/api/v1/approvals/${approvalId}`
    );
    return response.data;
  }

  async approvePolicy(
    policyId: string,
    version: string,
    comment?: string
  ): Promise<any> {
    const response = await this.client.post(
      `/api/v1/approvals/policies/${policyId}/approve`,
      comment !== undefined ? { comment } : {},
      { params: { version } }
    );
    return response.data;
  }

  async rejectPolicy(
    policyId: string,
    version: string,
    comment: string
  ): Promise<any> {
    const response = await this.client.post(
      `/api/v1/approvals/policies/${policyId}/reject`,
      { comment },
      { params: { version } }
    );
    return response.data;
  }

  // ─── Evidence ──────────────────────────────────────────

  async exportEvidence(runId: string): Promise<Blob> {
    const response = await this.client.get(
      `/api/v1/evidence/runs/${runId}/bundle`,
      { responseType: 'blob' }
    );
    return response.data;
  }

  async verifyEvidence(runId: string): Promise<any> {
    const response = await this.client.post('/api/v1/evidence/verify', {
      run_id: runId,
    });
    return response.data;
  }
}
