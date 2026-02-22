/**
 * Offline event buffering
 */

import * as fs from 'fs';
import * as path from 'path';
import { AegisRunClient } from './client';

const fsPromises = fs.promises;

interface BufferedEvent {
  type: string;
  run_id: string;
  timestamp: string;
  [key: string]: any;
}

export class OfflineBuffer {
  private bufferDir: string;
  private events: BufferedEvent[] = [];

  constructor(bufferDir: string = '.aegisrun_buffer') {
    this.bufferDir = bufferDir;
    
    if (!fs.existsSync(this.bufferDir)) {
      fs.mkdirSync(this.bufferDir, { recursive: true });
    }
  }

  queueRunStart(runId: string, metadata: Record<string, any>): void {
    this.events.push({
      type: 'run.started',
      run_id: runId,
      metadata,
      timestamp: new Date().toISOString(),
    });
    this.persist();
  }

  queueRunEnd(runId: string, outcome?: Record<string, any>): void {
    this.events.push({
      type: 'run.ended',
      run_id: runId,
      outcome,
      payload: outcome ? { outcome } : {},
      timestamp: new Date().toISOString(),
    });
    this.persist();
  }

  /**
   * Flush buffered events to the server.
   *
   * - `run.started` events are replayed via `client.createRun()`.
   * - All other event types are submitted via `client.submitEvent()`.
   *
   * Events that fail to send are retained so the caller can retry later.
   */
  async flush(client: AegisRunClient): Promise<void> {
    if (this.events.length === 0) {
      return;
    }

    const remaining: BufferedEvent[] = [];

    for (const event of this.events) {
      try {
        if (event.type === 'run.started') {
          const meta = (event.metadata || {}) as Record<string, any>;
          await client.createRun({
            policyId: meta.policy_id || '',
            policyVersion: meta.policy_version || '',
            metadata: meta,
          });
        } else {
          await client.submitEvent({
            runId: event.run_id,
            eventType: event.type,
            payload: (event as any).payload || {},
            timestamp: event.timestamp,
          });
        }
      } catch {
        // Keep failed events so they can be retried on the next flush.
        remaining.push(event);
      }
    }

    this.events = remaining;
    await this.persist();
  }

  private async persist(): Promise<void> {
    const bufferFile = path.join(this.bufferDir, 'events.json');
    await fsPromises.writeFile(bufferFile, JSON.stringify(this.events, null, 2));
  }
}
