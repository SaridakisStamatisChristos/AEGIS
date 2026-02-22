/**
 * Tool call execution
 */

import { AegisRunClient } from './client';
import { Decision } from './types';

export interface ToolCallConfig {
  client: AegisRunClient;
  runId: string;
  stepId: string;
  seqNo: number;
  toolName: string;
}

export class ToolCall {
  private client: AegisRunClient;
  private runId: string;
  private stepId: string;
  private seqNo: number;
  private toolName: string;

  public toolCallId?: string;
  public decision?: Decision;
  public result?: any;
  public error?: string;

  constructor(config: ToolCallConfig) {
    this.client = config.client;
    this.runId = config.runId;
    this.stepId = config.stepId;
    this.seqNo = config.seqNo;
    this.toolName = config.toolName;
  }

  async execute(params: {
    args: Record<string, any>;
    stateVector: Record<string, any>;
    executor?: string;
    metadata?: Record<string, any>;
  }): Promise<any> {
    const response = await this.client.executeToolCall({
      run_id: this.runId,
      step_id: this.stepId,
      tool_name: this.toolName,
      args: params.args,
      state_vector: params.stateVector,
      executor: params.executor,
      metadata: params.metadata,
    });

    this.toolCallId = response.tool_call_id;

    // Parse decision as a struct object
    if (response.decision) {
      this.decision = typeof response.decision === 'string'
        ? { action: response.decision as any, policy_rule_id: '', reason: '' }
        : response.decision;
    }

    if (response.result !== undefined) {
      this.result = response.result;
    }

    if (response.error) {
      this.error = response.error;
    }

    // Handle blocked calls
    if (this.decision && this.decision.action === 'block') {
      throw new ToolCallBlockedError(
        `Tool call blocked by policy`,
        this.decision
      );
    }

    if (this.error) {
      throw new ToolCallExecutionError(this.error);
    }

    return this.result;
  }
}

export class ToolCallBlockedError extends Error {
  constructor(message: string, public decision: Decision) {
    super(message);
    this.name = 'ToolCallBlockedError';
  }
}

export class ToolCallExecutionError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ToolCallExecutionError';
  }
}
