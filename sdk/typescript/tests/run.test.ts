/**
 * Tests for the Run class
 */

import { Run } from '../src/run';
import { AegisRunClient } from '../src/client';
import { RunStatus } from '../src/types';

// Mock AegisRunClient
jest.mock('../src/client');

const CORRECT_COUNTERS = { steps: 0, tool_calls: 0, bytes_egressed: 0, retries: 0, blocks: 0 };

function mockRunResponse(overrides: Record<string, any> = {}) {
  return {
    run_id: '01HQXYZ123456789ABCDEFGHIJ',
    org_id: 'org-1',
    policy_ref: { policy_id: 'test-policy', version: 'v1' },
    metadata: {},
    status: 'running',
    counters: { ...CORRECT_COUNTERS },
    created_at: '2024-01-15T10:00:00Z',
    ...overrides,
  } as any;
}

describe('Run', () => {
  let mockClient: jest.Mocked<AegisRunClient>;

  beforeEach(() => {
    jest.clearAllMocks();
    mockClient = new AegisRunClient() as jest.Mocked<AegisRunClient>;
  });

  describe('constructor', () => {
    it('should create a run with required config', () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });

      expect(run.status).toBe(RunStatus.Running);
      expect(run.counters.steps).toBe(0);
      expect(run.counters.tool_calls).toBe(0);
    });

    it('should create a run with optional metadata', () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
        metadata: { agent: 'test-agent', environment: 'test' },
      });

      expect(run).toBeDefined();
    });

    it('should create a run with offline mode', () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
        offlineMode: true,
      });

      expect(run).toBeDefined();
    });
  });

  describe('start()', () => {
    it('should start a run and set runId', async () => {
      const resp = mockRunResponse();
      mockClient.createRun.mockResolvedValue(resp);

      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });

      await run.start();

      expect(run.runId).toBe(resp.run_id);
      expect(run.createdAt).toBeInstanceOf(Date);
      expect(mockClient.createRun).toHaveBeenCalledWith({
        policyId: 'test-policy',
        policyVersion: 'v1',
        metadata: {},
        parentRunId: undefined,
      });
    });

    it('should pass metadata when starting run', async () => {
      mockClient.createRun.mockResolvedValue(mockRunResponse());

      const metadata = { agent: 'test-agent', task: 'test-task' };
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
        metadata,
      });

      await run.start();

      expect(mockClient.createRun).toHaveBeenCalledWith({
        policyId: 'test-policy',
        policyVersion: 'v1',
        metadata,
        parentRunId: undefined,
      });
    });

    it('should return self for chaining', async () => {
      mockClient.createRun.mockResolvedValue(mockRunResponse());

      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });

      const result = await run.start();
      expect(result).toBe(run);
    });

    it('should generate local runId in offline mode on server error', async () => {
      mockClient.createRun.mockRejectedValue(new Error('Network error'));

      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
        offlineMode: true,
      });

      await run.start();

      expect(run.runId).toBeDefined();
      expect(run.runId).toMatch(/^[0-9A-HJKMNP-TV-Z]{26}$/); // ULID format
    });

    it('should throw error when not in offline mode and server fails', async () => {
      mockClient.createRun.mockRejectedValue(new Error('Network error'));

      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
        offlineMode: false,
      });

      await expect(run.start()).rejects.toThrow('Network error');
    });
  });

  describe('step()', () => {
    beforeEach(() => {
      mockClient.createRun.mockResolvedValue(mockRunResponse());
    });

    it('should throw if run not started', async () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });

      await expect(
        run.step('test-step', {}, async () => 'result')
      ).rejects.toThrow('Run not started');
    });

    it('should execute step function and return result', async () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });
      await run.start();
      mockClient.submitEvent.mockClear();

      const result = await run.step('test-step', { key: 'value' }, async () => {
        return 'test-result';
      });

      expect(result).toBe('test-result');
      expect(run.counters.steps).toBe(1);
      expect(mockClient.submitEvent).toHaveBeenCalledWith(
        expect.objectContaining({
          runId: expect.any(String),
          eventType: 'step.started',
          payload: expect.objectContaining({
            name: 'test-step',
          }),
          timestamp: expect.any(String),
        })
      );
      expect(mockClient.submitEvent).toHaveBeenCalledWith(
        expect.objectContaining({
          runId: expect.any(String),
          eventType: 'step.ended',
          payload: expect.objectContaining({
            status: 'completed',
          }),
          timestamp: expect.any(String),
        })
      );
    });

    it('should increment step counter for each step', async () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });
      await run.start();

      await run.step('step-1', {}, async () => 'result-1');
      await run.step('step-2', {}, async () => 'result-2');
      await run.step('step-3', {}, async () => 'result-3');

      expect(run.counters.steps).toBe(3);
    });

    it('should propagate errors from step function', async () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });
      await run.start();
      mockClient.submitEvent.mockClear();

      await expect(
        run.step('failing-step', {}, async () => {
          throw new Error('Step failed');
        })
      ).rejects.toThrow('Step failed');

      expect(mockClient.submitEvent).toHaveBeenCalledWith(
        expect.objectContaining({
          eventType: 'step.ended',
          payload: expect.objectContaining({
            status: 'failed',
            error: 'Step failed',
          }),
        })
      );
    });

    it('should pass step to function for tool calls', async () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });
      await run.start();

      await run.step('step-with-context', { data: 'test' }, async (step) => {
        expect(step).toBeDefined();
        expect(step.stepId).toBeDefined();
        return 'done';
      });
    });

    it('should support synchronous step functions', async () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });
      await run.start();

      // Even though step() is async, the callback can be sync
      const result = await run.step('sync-step', {}, (step) => {
        return 'sync-result';
      });

      expect(result).toBe('sync-result');
    });
  });

  describe('end()', () => {
    beforeEach(() => {
      mockClient.createRun.mockResolvedValue(mockRunResponse());
    });

    it('should update status to completed', async () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });
      await run.start();

      run.end();

      expect(run.status).toBe(RunStatus.Completed);
    });

    it('should accept optional outcome', async () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });
      await run.start();

      run.end({ status: 'success', result: { data: 'test' } });

      expect(run.status).toBe(RunStatus.Completed);
    });
  });

  describe('counters', () => {
    beforeEach(() => {
      mockClient.createRun.mockResolvedValue(mockRunResponse());
    });

    it('should initialize counters to zero', () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });

      expect(run.counters).toEqual({
        steps: 0,
        tool_calls: 0,
        bytes_egressed: 0,
        retries: 0,
        blocks: 0,
      });
    });

    it('should track step execution', async () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
      });
      await run.start();

      await run.step('step-1', {}, async () => 'result');

      expect(run.counters.steps).toBe(1);
    });
  });

  describe('offline mode', () => {
    it('should buffer events when offline', async () => {
      mockClient.createRun.mockRejectedValue(new Error('Network error'));

      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
        offlineMode: true,
      });

      await run.start();

      expect(run.runId).toBeDefined();
      expect(run.createdAt).toBeDefined();
    });

    it('should have flushOfflineEvents method', async () => {
      const run = new Run({
        client: mockClient,
        policyId: 'test-policy',
        policyVersion: 'v1',
        offlineMode: true,
      });

      expect(typeof run.flushOfflineEvents).toBe('function');
    });
  });
});

describe('Run integration scenarios', () => {
  let mockClient: jest.Mocked<AegisRunClient>;

  beforeEach(() => {
    jest.clearAllMocks();
    mockClient = new AegisRunClient() as jest.Mocked<AegisRunClient>;
    mockClient.createRun.mockResolvedValue({
      run_id: '01HQXYZ123456789ABCDEFGHIJ',
      org_id: 'org-1',
      policy_ref: { policy_id: 'agent-policy', version: 'v1' },
      metadata: {},
      status: 'running',
      counters: { steps: 0, tool_calls: 0, bytes_egressed: 0, retries: 0, blocks: 0 },
      created_at: '2024-01-15T10:00:00Z',
    } as any);
  });

  it('should support typical agent workflow', async () => {
    const run = new Run({
      client: mockClient,
      policyId: 'agent-policy',
      policyVersion: 'v1',
      metadata: { agent: 'test-agent' },
    });

    await run.start();

    // Step 1: Planning
    const plan = await run.step('planning', { phase: 'init' }, async () => {
      return { tasks: ['task1', 'task2'] };
    });

    // Step 2: Execution
    const results = await run.step(
      'execution',
      { phase: 'execute', plan },
      async () => {
        return { completed: true };
      }
    );

    // Step 3: Reporting
    await run.step(
      'reporting',
      { phase: 'report', results },
      async () => {
        return { report: 'success' };
      }
    );

    run.end({ status: 'success' });

    expect(run.status).toBe(RunStatus.Completed);
    expect(run.counters.steps).toBe(3);
  });

  it('should handle step failures gracefully', async () => {
    const run = new Run({
      client: mockClient,
      policyId: 'agent-policy',
      policyVersion: 'v1',
    });

    await run.start();

    // Successful step
    await run.step('step-1', {}, async () => 'ok');

    // Failing step
    try {
      await run.step('step-2', {}, async () => {
        throw new Error('Something went wrong');
      });
    } catch (e) {
      // Expected
    }

    // Should be able to continue with more steps
    await run.step('step-3', {}, async () => 'recovered');

    expect(run.counters.steps).toBe(2); // Only successful steps counted
  });
});
