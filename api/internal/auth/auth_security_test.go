package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// fakeVerifier implements TokenVerifier for testing.
type fakeVerifier struct {
	claims *Claims
	err    error
}

func (f *fakeVerifier) VerifyIDToken(_ context.Context, _ string) (*Claims, error) {
	return f.claims, f.err
}

func testLogger(t *testing.T) *zap.Logger {
	return zaptest.NewLogger(t)
}

// callHandler is a helper that runs the Authenticate middleware with an
// inner handler that records whether it was reached.
func callHandler(t *testing.T, mw *AuthMiddleware, authHeader string) *httptest.ResponseRecorder {
	t.Helper()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	mw.Authenticate(inner).ServeHTTP(rr, req)
	return rr
}

// makeMiddleware builds an AuthMiddleware backed by a fakeVerifier.
func makeMiddleware(t *testing.T, claims *Claims, verifyErr error) *AuthMiddleware {
	t.Helper()
	return NewAuthMiddleware(
		&fakeVerifier{claims: claims, err: verifyErr},
		NewRBAC(),
		testLogger(t),
	)
}

// ===========================================================================
// 1. Auth Bypass Tests
// ===========================================================================

func TestAuthBypass_MissingAuthorizationHeader(t *testing.T) {
	mw := makeMiddleware(t, nil, nil)
	rr := callHandler(t, mw, "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthBypass_MalformedAuthorizationHeader(t *testing.T) {
	mw := makeMiddleware(t, nil, nil)

	malformed := []string{
		"Token abc",          // wrong scheme
		"bearer abc",         // lowercase (RFC 6750 requires "Bearer")
		"Bearer",             // no token value
		"Basic dXNlcjpwYXNz", // wrong auth type
		"",                   // empty
	}

	for _, header := range malformed {
		t.Run(header, func(t *testing.T) {
			rr := callHandler(t, mw, header)
			assert.Equal(t, http.StatusUnauthorized, rr.Code,
				"header %q should be rejected", header)
		})
	}
}

func TestAuthBypass_InvalidToken(t *testing.T) {
	mw := makeMiddleware(t, nil, fmt.Errorf("invalid signature"))
	rr := callHandler(t, mw, "Bearer totally-invalid-token")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthBypass_ExpiredToken(t *testing.T) {
	mw := makeMiddleware(t, nil, fmt.Errorf("token is expired"))
	rr := callHandler(t, mw, "Bearer expired-token")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthBypass_IssuerMismatch(t *testing.T) {
	mw := makeMiddleware(t, nil, fmt.Errorf("token issuer mismatch: got \"evil.com\", expected \"auth.example.com\""))
	rr := callHandler(t, mw, "Bearer wrong-issuer-token")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthBypass_TokenTooOld(t *testing.T) {
	mw := makeMiddleware(t, nil, fmt.Errorf("token too old: issued 25h0m0s ago, max allowed 1h0m0s"))
	rr := callHandler(t, mw, "Bearer old-token")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthBypass_ValidTokenPassesThrough(t *testing.T) {
	claims := &Claims{
		Subject: "user-1",
		Email:   "dev@acme.com",
		Name:    "Dev",
	}
	mw := makeMiddleware(t, claims, nil)
	rr := callHandler(t, mw, "Bearer valid-token")
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ===========================================================================
// 2. Org Isolation Tests
// ===========================================================================

func TestOrgIsolation_SameDomainSameOrg(t *testing.T) {
	org1 := orgIDFromEmail("alice@acme.com")
	org2 := orgIDFromEmail("bob@acme.com")
	assert.Equal(t, org1, org2, "users from the same email domain must share an org ID")
}

func TestOrgIsolation_DifferentDomainDifferentOrg(t *testing.T) {
	org1 := orgIDFromEmail("alice@acme.com")
	org2 := orgIDFromEmail("alice@evil.com")
	assert.NotEqual(t, org1, org2, "users from different domains must get different org IDs")
}

func TestOrgIsolation_EmptyEmailGetsUnknown(t *testing.T) {
	assert.Equal(t, "org-unknown", orgIDFromEmail(""))
}

func TestOrgIsolation_NoDomainGetsUnknown(t *testing.T) {
	assert.Equal(t, "org-unknown", orgIDFromEmail("nodomain"))
}

func TestOrgIsolation_ExplicitOrgIDPreferred(t *testing.T) {
	claims := &Claims{
		Subject: "user-1",
		Email:   "dev@acme.com",
		Name:    "Dev",
		OrgID:   "explicit-org-123",
	}
	mw := makeMiddleware(t, claims, nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		require.NotNil(t, user)
		assert.Equal(t, "explicit-org-123", user.OrgID,
			"when the token contains an explicit org_id, it should be used as-is")
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer valid")
	mw.Authenticate(inner).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOrgIsolation_OrgIsolationMiddlewareRejectsUnauthenticated(t *testing.T) {
	logger := testLogger(t)
	orgMw := NewOrgIsolationMiddleware(logger)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil)
	// No user in context
	orgMw.Isolate(inner).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOrgIsolation_UserContextPropagated(t *testing.T) {
	claims := &Claims{
		Subject: "user-1",
		Email:   "dev@acme.com",
		Name:    "Dev",
	}
	mw := makeMiddleware(t, claims, nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		require.NotNil(t, user, "user must be present in context after authentication")
		assert.Equal(t, "user-1", user.UserID)
		assert.NotEmpty(t, user.OrgID, "OrgID must be derived when not explicitly set")
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer valid")
	mw.Authenticate(inner).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ===========================================================================
// 3. Privilege Escalation Tests
// ===========================================================================

func TestPrivEsc_ViewerCannotCreate(t *testing.T) {
	rbac := NewRBAC()
	assert.False(t, rbac.HasPermission(RoleViewer, PermRunCreate),
		"viewer must not have run:create")
	assert.False(t, rbac.HasPermission(RoleViewer, PermPolicyCreate),
		"viewer must not have policy:create")
	assert.False(t, rbac.HasPermission(RoleViewer, PermUserManage),
		"viewer must not have user:manage")
}

func TestPrivEsc_DeveloperCannotManageUsers(t *testing.T) {
	rbac := NewRBAC()
	assert.False(t, rbac.HasPermission(RoleDeveloper, PermUserManage),
		"developer must not have user:manage")
	assert.False(t, rbac.HasPermission(RoleDeveloper, PermKeyManage),
		"developer must not have key:manage")
	assert.False(t, rbac.HasPermission(RoleDeveloper, PermPolicyCreate),
		"developer must not have policy:create")
	assert.False(t, rbac.HasPermission(RoleDeveloper, PermPolicyApprove),
		"developer must not have policy:approve")
}

func TestPrivEsc_PolicyAdminCannotApprove(t *testing.T) {
	rbac := NewRBAC()
	assert.False(t, rbac.HasPermission(RolePolicyAdmin, PermPolicyApprove),
		"policy_admin must not have policy:approve (separation of duties)")
	assert.False(t, rbac.HasPermission(RolePolicyAdmin, PermPolicyDeploy),
		"policy_admin must not have policy:deploy")
	assert.False(t, rbac.HasPermission(RolePolicyAdmin, PermUserManage),
		"policy_admin must not have user:manage")
}

func TestPrivEsc_ApproverCannotCreate(t *testing.T) {
	rbac := NewRBAC()
	assert.False(t, rbac.HasPermission(RoleApprover, PermPolicyCreate),
		"approver must not have policy:create")
	assert.False(t, rbac.HasPermission(RoleApprover, PermRunCreate),
		"approver must not have run:create")
}

func TestPrivEsc_OrgAdminHasAllPermissions(t *testing.T) {
	rbac := NewRBAC()
	allPerms := []Permission{
		PermRunView, PermRunCreate, PermRunCancel,
		PermPolicyView, PermPolicyCreate, PermPolicyEdit,
		PermPolicyApprove, PermPolicyDeploy,
		PermEvidenceView, PermEvidenceExport,
		PermUserManage, PermKeyManage, PermAuditView,
	}
	for _, perm := range allPerms {
		assert.True(t, rbac.HasPermission(RoleOrgAdmin, perm),
			"org_admin must have permission %s", perm)
	}
}

func TestPrivEsc_InvalidRoleFallsBackToDefault(t *testing.T) {
	claims := &Claims{
		Subject: "user-1",
		Email:   "dev@acme.com",
		Name:    "Dev",
		Roles:   []string{"superadmin"}, // non-existent role
	}
	mw := makeMiddleware(t, claims, nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		require.NotNil(t, user)
		assert.Equal(t, RoleDeveloper, user.Role,
			"unknown role in token must fall back to default (developer)")
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer valid")
	mw.Authenticate(inner).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestPrivEsc_RoleFromTokenIsHonored(t *testing.T) {
	claims := &Claims{
		Subject: "user-1",
		Email:   "admin@acme.com",
		Name:    "Admin",
		Roles:   []string{"org_admin"},
	}
	mw := makeMiddleware(t, claims, nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		require.NotNil(t, user)
		assert.Equal(t, RoleOrgAdmin, user.Role)
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer valid")
	mw.Authenticate(inner).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestPrivEsc_RequirePermissionBlocksUnauthorized(t *testing.T) {
	claims := &Claims{
		Subject: "viewer-1",
		Email:   "read@acme.com",
		Name:    "Reader",
		Roles:   []string{"viewer"},
	}
	mw := makeMiddleware(t, claims, nil)

	// Wrap with Authenticate then RequirePermission(run:create)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Authenticate(
		mw.RequirePermission(PermRunCreate)(inner),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer valid")
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestPrivEsc_RequireRoleBlocksMismatch(t *testing.T) {
	claims := &Claims{
		Subject: "dev-1",
		Email:   "dev@acme.com",
		Name:    "Developer",
		Roles:   []string{"developer"},
	}
	mw := makeMiddleware(t, claims, nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Authenticate(
		mw.RequireRole(RoleOrgAdmin)(inner),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer valid")
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// ===========================================================================
// 4. Mock OIDC Production Guard Tests
// ===========================================================================

func TestMockOIDC_BlockedInProduction(t *testing.T) {
	logger := testLogger(t)
	_, err := NewMockOIDCProvider("production", logger)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMockProviderInProduction)
}

func TestMockOIDC_BlockedInProductionCaseInsensitive(t *testing.T) {
	logger := testLogger(t)
	_, err := NewMockOIDCProvider("Production", logger)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMockProviderInProduction)

	_, err = NewMockOIDCProvider("PRODUCTION", logger)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMockProviderInProduction)
}

func TestMockOIDC_AllowedInDevelopment(t *testing.T) {
	logger := testLogger(t)
	provider, err := NewMockOIDCProvider("development", logger)
	require.NoError(t, err)
	require.NotNil(t, provider)
}

func TestMockOIDC_AllowedInStaging(t *testing.T) {
	logger := testLogger(t)
	provider, err := NewMockOIDCProvider("staging", logger)
	require.NoError(t, err)
	require.NotNil(t, provider)
}

func TestMockOIDC_AllowedWhenEmpty(t *testing.T) {
	logger := testLogger(t)
	provider, err := NewMockOIDCProvider("", logger)
	require.NoError(t, err)
	require.NotNil(t, provider)
}

func TestMockOIDC_ReturnsStaticClaims(t *testing.T) {
	logger := testLogger(t)
	provider, err := NewMockOIDCProvider("development", logger)
	require.NoError(t, err)

	claims, err := provider.VerifyIDToken(context.Background(), "any-token")
	require.NoError(t, err)
	assert.Equal(t, "mock-user-123", claims.Subject)
	assert.Equal(t, "dev@aegisrun.local", claims.Email)
}

// ===========================================================================
// 5. RBAC Role Validation Tests
// ===========================================================================

func TestRBAC_UnknownRoleHasNoPermissions(t *testing.T) {
	rbac := NewRBAC()
	assert.False(t, rbac.HasPermission(Role("hacker"), PermRunView))
	assert.False(t, rbac.HasPermission(Role(""), PermRunView))
}

func TestRBAC_ValidateRoleRejectsUnknown(t *testing.T) {
	rbac := NewRBAC()
	_, err := rbac.ValidateRole("superuser")
	assert.Error(t, err)
}

func TestRBAC_AllDefinedRolesHavePermissions(t *testing.T) {
	rbac := NewRBAC()
	roles := []Role{RoleViewer, RoleDeveloper, RolePolicyAdmin, RoleApprover, RoleOrgAdmin}
	for _, role := range roles {
		perms := rbac.GetPermissions(role)
		assert.NotEmpty(t, perms, "role %s must have at least one permission", role)
	}
}

func TestRBAC_RequirePermissionError(t *testing.T) {
	rbac := NewRBAC()
	err := rbac.RequirePermission(RoleViewer, PermUserManage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lacks permission")
}
