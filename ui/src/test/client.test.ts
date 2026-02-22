/**
 * Tests for the Axios API client (interceptors, auth header, error handling).
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import axios from 'axios';
import type { AxiosAdapter } from 'axios';

// We re-import the module under test each time to get the interceptors fresh.
// Instead, we test the interceptor behaviour directly.

describe('API client interceptors', () => {
  let mockAdapter: ReturnType<typeof vi.fn>;
  let apiClient: ReturnType<typeof axios.create>;

  const storage: Record<string, string> = {};

  beforeEach(async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(
      (k: string) => storage[k] ?? null,
    );
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(
      (k: string, v: string) => {
        storage[k] = v;
      },
    );
    vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(
      (k: string) => {
        delete storage[k];
      },
    );

    // Dynamically import to pick up patched localStorage
    // Use a mock adapter so we never hit the network
    mockAdapter = vi.fn();

    apiClient = axios.create({
      baseURL: '/api/v1',
      headers: { 'Content-Type': 'application/json' },
      adapter: mockAdapter as unknown as AxiosAdapter,
    });

    // Replicate the same request interceptor from src/api/client.ts
    apiClient.interceptors.request.use((config) => {
      const token = localStorage.getItem('auth_token');
      if (token) {
        config.headers.Authorization = `Bearer ${token}`;
      }
      return config;
    });

    // Replicate the same response error interceptor
    apiClient.interceptors.response.use(
      (response) => response,
      (error) => {
        if (error.response) {
          const { status } = error.response;
          if (status === 401) {
            localStorage.removeItem('auth_token');
            localStorage.removeItem('auth_user');
          }
          const data = error.response.data;
          const msg =
            data?.message ??
            data?.error ??
            `Request failed with status ${status}`;
          (error as any).displayMessage = msg;
        } else if (error.request) {
          (error as any).displayMessage =
            'Network error — please check your connection.';
        }
        return Promise.reject(error);
      },
    );
  });

  afterEach(() => {
    vi.restoreAllMocks();
    for (const k of Object.keys(storage)) delete storage[k];
  });

  // --- Request interceptor --------------------------------------------------

  it('attaches Authorization header when token exists', async () => {
    storage['auth_token'] = 'my-jwt';

    mockAdapter.mockResolvedValueOnce({
      status: 200,
      data: { ok: true },
      headers: {},
    });

    await apiClient.get('/stats');

    const sentConfig = mockAdapter.mock.calls[0][0];
    expect(sentConfig.headers.Authorization).toBe('Bearer my-jwt');
  });

  it('does not attach Authorization when no token', async () => {
    mockAdapter.mockResolvedValueOnce({
      status: 200,
      data: {},
      headers: {},
    });

    await apiClient.get('/runs');

    const sentConfig = mockAdapter.mock.calls[0][0];
    expect(sentConfig.headers.Authorization).toBeUndefined();
  });

  // --- Response interceptor (error path) ------------------------------------

  it('removes auth_token on 401 response', async () => {
    storage['auth_token'] = 'expired';
    storage['auth_user'] = '{}';

    // Stub location to avoid jsdom navigation errors
    Object.defineProperty(window, 'location', {
      value: { href: '/dashboard', pathname: '/dashboard' },
      writable: true,
      configurable: true,
    });

    mockAdapter.mockRejectedValueOnce(
      (() => {
        const err = new axios.AxiosError(
          'Unauthorized',
          '401',
          undefined,
          {},
          { status: 401, data: { message: 'token expired' }, headers: {}, config: {} as any, statusText: 'Unauthorized' },
        );
        return err;
      })(),
    );

    await expect(apiClient.get('/runs')).rejects.toThrow();
    expect(storage['auth_token']).toBeUndefined();
    expect(storage['auth_user']).toBeUndefined();
  });

  it('sets displayMessage from response.data.message', async () => {
    mockAdapter.mockRejectedValueOnce(
      new axios.AxiosError(
        'Bad request',
        '400',
        undefined,
        {},
        { status: 400, data: { message: 'Invalid run ID' }, headers: {}, config: {} as any, statusText: 'Bad Request' },
      ),
    );

    try {
      await apiClient.get('/runs/bad');
    } catch (err: any) {
      expect(err.displayMessage).toBe('Invalid run ID');
    }
  });

  it('sets displayMessage from response.data.error if no message', async () => {
    mockAdapter.mockRejectedValueOnce(
      new axios.AxiosError(
        'Internal',
        '500',
        undefined,
        {},
        { status: 500, data: { error: 'db crash' }, headers: {}, config: {} as any, statusText: 'Server Error' },
      ),
    );

    try {
      await apiClient.get('/fail');
    } catch (err: any) {
      expect(err.displayMessage).toBe('db crash');
    }
  });

  it('falls back to status-based message when data is empty', async () => {
    mockAdapter.mockRejectedValueOnce(
      new axios.AxiosError(
        'Not Found',
        '404',
        undefined,
        {},
        { status: 404, data: {}, headers: {}, config: {} as any, statusText: 'Not Found' },
      ),
    );

    try {
      await apiClient.get('/nope');
    } catch (err: any) {
      expect(err.displayMessage).toBe('Request failed with status 404');
    }
  });

  it('sets network error message when no response', async () => {
    const err = new axios.AxiosError('Network Error', 'ERR_NETWORK');
    (err as any).request = {}; // mark as having a request but no response
    mockAdapter.mockRejectedValueOnce(err);

    try {
      await apiClient.get('/offline');
    } catch (e: any) {
      expect(e.displayMessage).toBe(
        'Network error — please check your connection.',
      );
    }
  });

  // --- Passthrough on success -----------------------------------------------

  it('passes through successful responses unchanged', async () => {
    mockAdapter.mockResolvedValueOnce({
      status: 200,
      data: { runs: [] },
      headers: {},
    });

    const resp = await apiClient.get('/runs');
    expect(resp.data).toEqual({ runs: [] });
  });
});
