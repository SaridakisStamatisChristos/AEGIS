/**
 * Authentication state store using Zustand
 */

import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';

interface User {
  user_id: string;
  org_id: string;
  email: string;
  name: string;
  roles: string[];
  permissions: string[];
}

interface AuthState {
  // State
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;

  // Actions
  login: (email: string, password: string) => Promise<void>;
  loginWithOIDC: () => void;
  handleOIDCCallback: (code: string, state: string) => Promise<void>;
  logout: () => void;
  setToken: (token: string) => void;
  setUser: (user: User) => void;
  clearError: () => void;
  checkAuth: () => Promise<boolean>;
  hasPermission: (permission: string) => boolean;
  hasRole: (role: string) => boolean;
}

const API_BASE_URL = '/api/v1';

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      // Initial state
      user: null,
      token: null,
      isAuthenticated: false,
      isLoading: false,
      error: null,

      // Login with email/password
      login: async (email: string, password: string) => {
        set({ isLoading: true, error: null });

        try {
          const response = await fetch(`${API_BASE_URL}/auth/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password }),
          });

          if (!response.ok) {
            const error = await response.json();
            throw new Error(error.message || 'Login failed');
          }

          const data = await response.json();
          set({
            token: data.token,
            user: data.user,
            isAuthenticated: true,
            isLoading: false,
          });

          // Update axios client headers
          localStorage.setItem('auth_token', data.token);
        } catch (error) {
          set({
            error: error instanceof Error ? error.message : 'Login failed',
            isLoading: false,
          });
          throw error;
        }
      },

      // Initiate OIDC login flow
      loginWithOIDC: () => {
        // Redirect to OIDC login endpoint
        window.location.href = '/auth/login';
      },

      // Handle OIDC callback
      handleOIDCCallback: async (code: string, state: string) => {
        set({ isLoading: true, error: null });

        try {
          const response = await fetch(
            `/auth/callback?code=${encodeURIComponent(code)}&state=${encodeURIComponent(state)}`
          );

          if (!response.ok) {
            throw new Error('OIDC callback failed');
          }

          const data = await response.json();
          set({
            token: data.token,
            user: data.user,
            isAuthenticated: true,
            isLoading: false,
          });

          localStorage.setItem('auth_token', data.token);
        } catch (error) {
          set({
            error:
              error instanceof Error ? error.message : 'Authentication failed',
            isLoading: false,
          });
          throw error;
        }
      },

      // Logout
      logout: () => {
        localStorage.removeItem('auth_token');
        set({
          user: null,
          token: null,
          isAuthenticated: false,
          error: null,
        });
        // Redirect to home
        window.location.href = '/';
      },

      // Set token directly (for testing/debugging)
      setToken: (token: string) => {
        localStorage.setItem('auth_token', token);
        set({ token, isAuthenticated: true });
      },

      // Set user directly
      setUser: (user: User) => {
        set({ user });
      },

      // Clear error
      clearError: () => {
        set({ error: null });
      },

      // Check if current auth is valid
      checkAuth: async () => {
        const { token } = get();
        if (!token) {
          return false;
        }

        try {
          const response = await fetch(`${API_BASE_URL}/auth/me`, {
            headers: {
              Authorization: `Bearer ${token}`,
            },
          });

          if (!response.ok) {
            get().logout();
            return false;
          }

          const user = await response.json();
          set({ user, isAuthenticated: true });
          return true;
        } catch {
          get().logout();
          return false;
        }
      },

      // Permission check
      hasPermission: (permission: string) => {
        const { user } = get();
        if (!user) return false;
        return user.permissions.includes(permission);
      },

      // Role check
      hasRole: (role: string) => {
        const { user } = get();
        if (!user) return false;
        return user.roles.includes(role);
      },
    }),
    {
      name: 'aegisrun-auth',
      storage: createJSONStorage(() => localStorage),
      // Persist user profile and token for session continuity.
      // On page refresh both token and isAuthenticated are restored together.
      partialize: (state) => ({
        user: state.user,
        token: state.token,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
);

// Selector hooks for common patterns
export const useUser = () => useAuthStore((state) => state.user);
export const useIsAuthenticated = () =>
  useAuthStore((state) => state.isAuthenticated);
export const useAuthLoading = () => useAuthStore((state) => state.isLoading);
export const useAuthError = () => useAuthStore((state) => state.error);

// Permission-based hooks
export const useCanManagePolicies = () =>
  useAuthStore((state) => state.hasPermission('policies:write'));
export const useCanApprove = () =>
  useAuthStore((state) => state.hasPermission('approvals:write'));
export const useCanViewEvidence = () =>
  useAuthStore((state) => state.hasPermission('evidence:read'));
export const useIsAdmin = () => useAuthStore((state) => state.hasRole('admin'));

// Auth guard component helper
export function requireAuth(): boolean {
  const { isAuthenticated, checkAuth } = useAuthStore.getState();

  if (!isAuthenticated) {
    checkAuth();
    return false;
  }

  return true;
}
