import axios from 'axios';
import { AegisRunClient } from '../src/client';

jest.mock('axios');

describe('API↔SDK contract: client routes and envelopes', () => {
  const mockedAxios = axios as jest.Mocked<typeof axios>;

  const mockHttp = {
    get: jest.fn(),
    post: jest.fn(),
    put: jest.fn(),
    delete: jest.fn(),
  };

  beforeEach(() => {
    jest.clearAllMocks();
    mockedAxios.create.mockReturnValue(mockHttp as any);
  });

  it('uses run lifecycle routes and payload shape for create/get', async () => {
    const created = {
      run_id: '01HQXYZ123456789ABCDEFGHIJ',
      org_id: 'org-1',
      policy_ref: { policy_id: 'policy-a', version: 'v1' },
      metadata: {},
      status: 'running',
      counters: { steps: 0, tool_calls: 0, bytes_egressed: 0, retries: 0, blocks: 0 },
      created_at: '2026-02-22T00:00:00Z',
    };

    mockHttp.post.mockResolvedValueOnce({ data: created });
    mockHttp.get.mockResolvedValueOnce({ data: created });

    const client = new AegisRunClient({ baseUrl: 'http://localhost:8080', apiToken: 'token' });

    const createResp = await client.createRun({
      policyId: 'policy-a',
      policyVersion: 'v1',
      metadata: { source: 'contract-test' },
    });

    expect(mockHttp.post).toHaveBeenCalledWith('/api/v1/runs/', {
      policy_ref: { policy_id: 'policy-a', version: 'v1' },
      metadata: { source: 'contract-test' },
    });
    expect(createResp.run_id).toBe(created.run_id);

    const getResp = await client.getRun(created.run_id);
    expect(mockHttp.get).toHaveBeenCalledWith(`/api/v1/runs/${created.run_id}/`);
    expect(getResp.run_id).toBe(created.run_id);
  });

  it('unwraps list envelope contracts for runs/steps/events', async () => {
    const run = {
      run_id: '01HQXYZ123456789ABCDEFGHIJ',
      org_id: 'org-1',
      policy_ref: { policy_id: 'policy-a', version: 'v1' },
      metadata: {},
      status: 'running',
      counters: { steps: 0, tool_calls: 0, bytes_egressed: 0, retries: 0, blocks: 0 },
      created_at: '2026-02-22T00:00:00Z',
    };
    const step = {
      step_id: '01STEP123456789ABCDEFGHIJK',
      run_id: run.run_id,
      seq_no: 1,
      name: 'step-1',
      state_vector: {},
      status: 'completed',
      started_at: '2026-02-22T00:00:01Z',
    };
    const event = {
      event_id: '01EVT123456789ABCDEFGHIJKL',
      run_id: run.run_id,
      seq_no: 1,
      event_type: 'run.started',
      timestamp: '2026-02-22T00:00:00Z',
      payload: {},
      event_hash: 'abc123',
    };

    mockHttp.get
      .mockResolvedValueOnce({ data: { runs: [run] } })
      .mockResolvedValueOnce({ data: { steps: [step] } })
      .mockResolvedValueOnce({ data: { events: [event] } });

    const client = new AegisRunClient();

    const runs = await client.listRuns({ limit: 10, offset: 0 });
    expect(mockHttp.get).toHaveBeenNthCalledWith(1, '/api/v1/runs/', {
      params: { limit: 10, offset: 0 },
    });
    expect(runs).toHaveLength(1);
    expect(runs[0].run_id).toBe(run.run_id);

    const steps = await client.listSteps(run.run_id);
    expect(mockHttp.get).toHaveBeenNthCalledWith(2, `/api/v1/runs/${run.run_id}/steps`);
    expect(steps).toHaveLength(1);
    expect(steps[0].step_id).toBe(step.step_id);

    const events = await client.listEvents(run.run_id);
    expect(mockHttp.get).toHaveBeenNthCalledWith(3, `/api/v1/runs/${run.run_id}/events`);
    expect(events).toHaveLength(1);
    expect(events[0].event_id).toBe(event.event_id);
  });

  it('uses gateway execute route and preserves decision payload contract', async () => {
    const gatewayResp = {
      tool_call_id: '01TC123456789ABCDEFGHIJKL',
      decision: {
        action: 'block',
        policy_rule_id: 'egress.domain_allowlist',
        reason: 'domain not allowlisted',
      },
      error: 'Blocked by policy: domain not allowlisted',
    };

    mockHttp.post.mockResolvedValueOnce({ data: gatewayResp });

    const client = new AegisRunClient();
    const resp = await client.executeToolCall({
      run_id: '01RUN123456789ABCDEFGHIJK',
      step_id: '01STEP123456789ABCDEFGHIJK',
      tool_name: 'http_request',
      args: { url: 'https://blocked.example' },
      state_vector: { seq: 1 },
    });

    expect(mockHttp.post).toHaveBeenCalledWith('/api/v1/gateway/execute', {
      run_id: '01RUN123456789ABCDEFGHIJK',
      step_id: '01STEP123456789ABCDEFGHIJK',
      tool_name: 'http_request',
      args: { url: 'https://blocked.example' },
      state_vector: { seq: 1 },
      executor: 'builtin',
      metadata: {},
    });

    expect(resp.tool_call_id).toBe(gatewayResp.tool_call_id);
    expect(resp.decision.action).toBe('block');
    expect(resp.decision.policy_rule_id).toBe('egress.domain_allowlist');
  });
});
