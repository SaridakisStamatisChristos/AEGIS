import { createContext, useContext, ReactNode } from 'react';
import { useAuthStore } from '../stores/authStore';

interface AuthContextType {
  user: {
    id: string;
    email: string;
    name: string;
    roles: string[];
  } | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (token: string) => void;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

/**
 * AuthProvider that delegates to the Zustand authStore.
 * This avoids duplicating auth state; both useAuth() and useAuthStore() stay in sync.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const storeUser = useAuthStore((s) => s.user);
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const isLoading = useAuthStore((s) => s.isLoading);
  const storeSetToken = useAuthStore((s) => s.setToken);
  const storeLogout = useAuthStore((s) => s.logout);

  const user = storeUser
    ? {
        id: storeUser.user_id,
        email: storeUser.email,
        name: storeUser.name,
        roles: storeUser.roles,
      }
    : null;

  const login = (token: string) => {
    storeSetToken(token);
  };

  const logout = () => {
    storeLogout();
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated,
        isLoading,
        login,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
