/**
 * Step execution management
 */

import { ulid } from 'ulid';
import { AegisRunClient } from './client';
import { ToolCall, ToolCallBlockedError } from './toolCall';
import { StepStatus, RunCounters } from './types';

export interface StepConfig {
  client: AegisRunClient;
  runId: string;
  seqNo: number;
  name: string;
  stateVector: Record<string, any>;
  counters?: RunCounters;
}

export class Step {
  private client: AegisRunClient;
  private runId: string;
  private seqNo: number;
  private name: string;
  private stateVector: Record<string, any>;

  public stepId: string;
  public status: StepStatus = StepStatus.Running;
  public startedAt?: Date;
  public endedAt?: Date;

  private toolCallSeq = 0;
  private counters?: RunCounters;

  constructor(config: StepConfig) {
    this.client = config.client;
    this.runId = config.runId;
    this.seqNo = config.seqNo;
    this.name = config.name;
    this.stateVector = config.stateVector;
    this.counters = config.counters;

    this.stepId = ulid();
  }

  start(): void {
    this.startedAt = new Date();
    this.emitEvent('step.started', {
      step_id: this.stepId,
      name: this.name,
      seq_no: this.seqNo,
    }, this.startedAt);
  }

  complete(): void {
    this.status = StepStatus.Completed;
    this.endedAt = new Date();
    this.emitEvent('step.ended', {
      step_id: this.stepId,
      status: 'completed',
      seq_no: this.seqNo,
    }, this.endedAt);
  }

  fail(error: string): void {
    this.status = StepStatus.Failed;
    this.endedAt = new Date();
    this.emitEvent('step.ended', {
      step_id: this.stepId,
      status: 'failed',
      seq_no: this.seqNo,
      error,
    }, this.endedAt);
  }

  private emitEvent(
    eventType: string,
    payload: Record<string, any>,
    timestamp: Date
  ): void {
    const maybePromise = this.client.submitEvent({
      runId: this.runId,
      eventType,
      payload,
      timestamp: timestamp.toISOString(),
    });

    Promise.resolve(maybePromise).catch(() => {
      // Best-effort lifecycle telemetry; do not fail step execution.
    });
  }

  async toolCall(
    toolName: string,
    args: Record<string, any>,
    executor: string = 'builtin'
  ): Promise<any> {
    const toolCall = new ToolCall({
      client: this.client,
      runId: this.runId,
      stepId: this.stepId,
      seqNo: this.toolCallSeq,
      toolName,
    });

    this.toolCallSeq++;

    try {
      const result = await toolCall.execute({
        args,
        stateVector: this.stateVector,
        executor,
      });
      if (this.counters) {
        this.counters.tool_calls++;
      }
      return result;
    } catch (error) {
      if (error instanceof ToolCallBlockedError && this.counters) {
        this.counters.blocks++;
      }
      throw error;
    }
  }
}
