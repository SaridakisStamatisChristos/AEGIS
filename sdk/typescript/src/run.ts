/**
 * Run management
 */

import { ulid } from 'ulid';
import { AegisRunClient } from './client';
import { Step } from './step';
import { RunStatus, RunCounters } from './types';
import { OfflineBuffer } from './offlineBuffer';

export interface RunConfig {
  client: AegisRunClient;
  policyId: string;
  policyVersion: string;
  metadata?: Record<string, any>;
  parentRunId?: string;
  offlineMode?: boolean;
}

export class Run {
  private client: AegisRunClient;
  private policyId: string;
  private policyVersion: string;
  private metadata: Record<string, any>;
  private parentRunId?: string;
  private offlineMode: boolean;

  public runId?: string;
  public status: RunStatus = RunStatus.Running;
  public counters: RunCounters = {
    steps: 0,
    tool_calls: 0,
    bytes_egressed: 0,
    retries: 0,
    blocks: 0,
  };
  public createdAt?: Date;

  private offlineBuffer?: OfflineBuffer;
  private stepSeq = 0;

  constructor(config: RunConfig) {
    this.client = config.client;
    this.policyId = config.policyId;
    this.policyVersion = config.policyVersion;
    this.metadata = config.metadata || {};
    this.parentRunId = config.parentRunId;
    this.offlineMode = config.offlineMode || false;

    if (this.offlineMode) {
      this.offlineBuffer = new OfflineBuffer();
    }
  }

  async start(): Promise<Run> {
    try {
      const response = await this.client.createRun({
        policyId: this.policyId,
        policyVersion: this.policyVersion,
        metadata: this.metadata,
        parentRunId: this.parentRunId,
      });

      this.runId = response.run_id;
      this.createdAt = new Date(response.created_at);

      return this;
    } catch (error) {
      if (this.offlineMode) {
        // Generate local run_id
        this.runId = ulid();
        this.createdAt = new Date();
        this.offlineBuffer?.queueRunStart(this.runId, this.metadata);
        return this;
      }
      throw error;
    }
  }

  async step<T>(
    name: string,
    stateVector: Record<string, any>,
    fn: (step: Step) => Promise<T> | T
  ): Promise<T> {
    if (!this.runId) {
      throw new Error('Run not started. Call run.start() first.');
    }

    const step = new Step({
      client: this.client,
      runId: this.runId,
      seqNo: this.stepSeq,
      name,
      stateVector,
      counters: this.counters,
    });

    this.stepSeq++;

    try {
      step.start();
      const result = await fn(step);
      step.complete();
      this.counters.steps++;
      return result;
    } catch (error) {
      step.fail(error instanceof Error ? error.message : String(error));
      throw error;
    }
  }

  end(outcome?: Record<string, any>): void {
    // Known limitation: there is no dedicated "end run" API endpoint, so this
    // method only updates the local status.  The server tracks run completion
    // implicitly via tool-call events.
    if (this.offlineMode && this.offlineBuffer) {
      this.offlineBuffer.queueRunEnd(this.runId!, outcome);
    }

    this.status = RunStatus.Completed;
  }

  async flushOfflineEvents(): Promise<void> {
    if (this.offlineBuffer) {
      await this.offlineBuffer.flush(this.client);
    }
  }
}
