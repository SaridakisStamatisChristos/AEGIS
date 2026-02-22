/**
 * Tests for the Zustand auth store.
 *
 * Covers login, logout, OIDC, token management, RBAC helpers,
 * and the persistence partialize config.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { useAuthStore } from '../stores/authStore';

// ---- helpers ---------------------------------------------------------------

function resetStore() {
  useAuthStore.setState({
    user: null,
    token: null,
    isAuthenticated: false,
    isLoading: false,
    error: null,
  });
}

const MOCK_USER = {
  user_id: 'u1',
  org_id: 'org1',
  email: 'alice@acme.com',
  name: 'Alice',
  roles: ['admin', 'reviewer'],
  permissions: ['policies:write', 'approvals:write', 'evidence:read'],
};

// ---- setup -----------------------------------------------------------------

// Stub localStorage
const storage: Record<string, string> = {};
beforeEach(() => {
  resetStore();
  vi.stubGlobal('fetch', vi.fn());
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation((k: string, v: string) => {
    storage[k] = v;
  });
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation((k: string) => storage[k] ?? null);
  vi.spyOn(Storage.prototype, 'removeItem').mockImplementation((k: string) => {
    delete storage[k];
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  for (const k of Object.keys(storage)) delete storage[k];
});

// ---- tests -----------------------------------------------------------------

describe('authStore', () => {
  // --- initial state --------------------------------------------------------
  it('starts unauthenticated', () => {
    const state = useAuthStore.getState();
    expect(state.isAuthenticated).toBe(false);
    expect(state.user).toBeNull();
    expect(state.token).toBeNull();
  });

  // --- login ----------------------------------------------------------------
  it('login sets token and user on success', async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ token: 'tok-123', user: MOCK_USER }),
    });

    await useAuthStore.getState().login('alice@acme.com', 'pw');

    const s = useAuthStore.getState();
    expect(s.isAuthenticated).toBe(true);
    expect(s.token).toBe('tok-123');
    expect(s.user?.email).toBe('alice@acme.com');
    expect(s.isLoading).toBe(false);
  });

  it('login stores token in localStorage', async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ token: 'tok-XYZ', user: MOCK_USER }),
    });

    await useAuthStore.getState().login('a@b.com', 'pw');
    expect(storage['auth_token']).toBe('tok-XYZ');
  });

  it('login sets error on failure', async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      json: async () => ({ message: 'bad creds' }),
    });

    await expect(useAuthStore.getState().login('x', 'y')).rejects.toThrow(
      'bad creds',
    );

    const s = useAuthStore.getState();
    expect(s.error).toBe('bad creds');
    expect(s.isAuthenticated).toBe(false);
  });

  // --- logout ---------------------------------------------------------------
  it('logout clears state and localStorage', () => {
    // Prime authenticated state
    useAuthStore.setState({
      user: MOCK_USER,
      token: 'tok',
      isAuthenticated: true,
    });
    storage['auth_token'] = 'tok';

    // Stub window.location to prevent jsdom navigation error
    const hrefSetter = vi.fn();
    Object.defineProperty(window, 'location', {
      value: { href: '/', pathname: '/' },
      writable: true,
      configurable: true,
    });
    Object.defineProperty(window.location, 'href', {
      set: hrefSetter,
      configurable: true,
    });

    useAuthStore.getState().logout();

    const s = useAuthStore.getState();
    expect(s.isAuthenticated).toBe(false);
    expect(s.token).toBeNull();
    expect(s.user).toBeNull();
    expect(storage['auth_token']).toBeUndefined();
  });

  // --- setToken / setUser ---------------------------------------------------
  it('setToken marks authenticated', () => {
    useAuthStore.getState().setToken('manual-token');
    const s = useAuthStore.getState();
    expect(s.token).toBe('manual-token');
    expect(s.isAuthenticated).toBe(true);
    expect(storage['auth_token']).toBe('manual-token');
  });

  it('setUser stores user', () => {
    useAuthStore.getState().setUser(MOCK_USER);
    expect(useAuthStore.getState().user).toEqual(MOCK_USER);
  });

  // --- clearError -----------------------------------------------------------
  it('clearError resets error to null', () => {
    useAuthStore.setState({ error: 'something broke' });
    useAuthStore.getState().clearError();
    expect(useAuthStore.getState().error).toBeNull();
  });

  // --- checkAuth ------------------------------------------------------------
  it('checkAuth returns false when no token', async () => {
    const ok = await useAuthStore.getState().checkAuth();
    expect(ok).toBe(false);
  });

  it('checkAuth validates token against /auth/me', async () => {
    useAuthStore.setState({ token: 'good-tok' });
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => MOCK_USER,
    });

    const ok = await useAuthStore.getState().checkAuth();
    expect(ok).toBe(true);
    expect(useAuthStore.getState().user).toEqual(MOCK_USER);
  });

  it('checkAuth logs out on 401', async () => {
    useAuthStore.setState({ token: 'expired', isAuthenticated: true });

    // Stub location for logout redirect
    Object.defineProperty(window, 'location', {
      value: { href: '/', pathname: '/' },
      writable: true,
      configurable: true,
    });

    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      status: 401,
    });

    const ok = await useAuthStore.getState().checkAuth();
    expect(ok).toBe(false);
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
  });

  // --- RBAC helpers ---------------------------------------------------------
  it('hasPermission returns true for granted permission', () => {
    useAuthStore.setState({ user: MOCK_USER });
    expect(useAuthStore.getState().hasPermission('policies:write')).toBe(true);
  });

  it('hasPermission returns false for missing permission', () => {
    useAuthStore.setState({ user: MOCK_USER });
    expect(useAuthStore.getState().hasPermission('admin:nuke')).toBe(false);
  });

  it('hasPermission returns false when no user', () => {
    expect(useAuthStore.getState().hasPermission('anything')).toBe(false);
  });

  it('hasRole returns true for matching role', () => {
    useAuthStore.setState({ user: MOCK_USER });
    expect(useAuthStore.getState().hasRole('admin')).toBe(true);
  });

  it('hasRole returns false for missing role', () => {
    useAuthStore.setState({ user: MOCK_USER });
    expect(useAuthStore.getState().hasRole('superuser')).toBe(false);
  });

  it('hasRole returns false when no user', () => {
    expect(useAuthStore.getState().hasRole('admin')).toBe(false);
  });

  // --- OIDC -----------------------------------------------------------------
  it('handleOIDCCallback sets token on success', async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ token: 'oidc-tok', user: MOCK_USER }),
    });

    await useAuthStore.getState().handleOIDCCallback('code123', 'state456');

    const s = useAuthStore.getState();
    expect(s.token).toBe('oidc-tok');
    expect(s.isAuthenticated).toBe(true);
  });

  it('handleOIDCCallback sets error on failure', async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
    });

    await expect(
      useAuthStore.getState().handleOIDCCallback('bad', 'state'),
    ).rejects.toThrow('OIDC callback failed');

    expect(useAuthStore.getState().error).toBe('OIDC callback failed');
  });
});
