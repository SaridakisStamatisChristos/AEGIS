import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Runs from './pages/Runs';
import RunDetail from './pages/RunDetail';
import Policies from './pages/Policies';
import PolicyDetail from './pages/PolicyDetail';
import PolicyCreate from './pages/PolicyCreate';
import Approvals from './pages/Approvals';
import Evidence from './pages/Evidence';
import Settings from './pages/Settings';
import { AuthProvider } from './context/AuthContext';
import { useIsAuthenticated, useAuthStore } from './stores/authStore';

/** Route guard — redirects to login if not authenticated. */
function RequireAuth({ children }: { children: React.ReactElement }) {
  const isAuthenticated = useIsAuthenticated();
  const token = useAuthStore((s) => s.token);

  // Not authed and no token → redirect to OIDC login
  if (!isAuthenticated || !token) {
    return <Navigate to="/login" replace />;
  }

  return children;
}

/** Simple login placeholder that initiates OIDC flow. */
function LoginPage() {
  const loginWithOIDC = useAuthStore((s) => s.loginWithOIDC);
  return (
    <div className="flex items-center justify-center h-screen bg-gray-50">
      <div className="text-center">
        <h1 className="text-2xl font-semibold mb-4">AegisRun</h1>
        <button
          onClick={loginWithOIDC}
          className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
        >
          Sign in
        </button>
      </div>
    </div>
  );
}

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/" element={<RequireAuth><Layout /></RequireAuth>}>
            <Route index element={<Navigate to="/runs" replace />} />
            <Route path="dashboard" element={<Dashboard />} />
            <Route path="runs" element={<Runs />} />
            <Route path="runs/:runId" element={<RunDetail />} />
            <Route path="policies" element={<Policies />} />
            <Route path="policies/new" element={<PolicyCreate />} />
            <Route path="policies/:policyId" element={<PolicyDetail />} />
            <Route path="approvals" element={<Approvals />} />
            <Route path="evidence" element={<Evidence />} />
            <Route path="settings" element={<Settings />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}

export default App;
