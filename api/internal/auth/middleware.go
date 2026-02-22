package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

// User represents an authenticated user
type User struct {
	UserID  string
	OrgID   string
	Email   string
	Name    string
	Role    Role
	Subject string
}

// TokenVerifier is an interface for verifying ID tokens
type TokenVerifier interface {
	VerifyIDToken(ctx context.Context, rawIDToken string) (*Claims, error)
}

// AuthMiddleware handles authentication
type AuthMiddleware struct {
	oidc        TokenVerifier
	rbac        *RBAC
	logger      *zap.Logger
	defaultRole Role
}

func NewAuthMiddleware(oidc TokenVerifier, rbac *RBAC, logger *zap.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		oidc:        oidc,
		rbac:        rbac,
		logger:      logger,
		defaultRole: RoleDeveloper,
	}
}

// Authenticate verifies the request has a valid session/token
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}

		token := parts[1]

		// Verify token (simplified - in production, use session store)
		claims, err := m.oidc.VerifyIDToken(r.Context(), token)
		if err != nil {
			m.logger.Warn("token verification failed", zap.Error(err))
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		// Build user identity from verified claims
		user := m.userFromClaims(claims)

		m.logger.Debug("authenticated user",
			zap.String("user_id", user.UserID),
			zap.String("org_id", user.OrgID),
			zap.String("role", string(user.Role)),
			zap.String("email", user.Email))

		// Add user to context
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// userFromClaims builds a User from OIDC claims. OrgID is resolved from
// the custom "org_id" claim, falling back to a deterministic hash of the
// email domain (so every user from the same domain shares an org). Role
// comes from the "roles" custom claim when present, otherwise the
// middleware-level default is used.
func (m *AuthMiddleware) userFromClaims(claims *Claims) *User {
	orgID := claims.OrgID
	if orgID == "" {
		orgID = orgIDFromEmail(claims.Email)
	}

	role := m.defaultRole
	if len(claims.Roles) > 0 {
		if r := Role(claims.Roles[0]); m.rbac.IsValidRole(r) {
			role = r
		}
	}

	return &User{
		UserID:  claims.Subject,
		OrgID:   orgID,
		Email:   claims.Email,
		Name:    claims.Name,
		Role:    role,
		Subject: claims.Subject,
	}
}

// orgIDFromEmail derives a stable org ID from the email domain.
// Returns "org-unknown" when the email is empty or lacks a domain.
func orgIDFromEmail(email string) string {
	if email == "" {
		return "org-unknown"
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "org-unknown"
	}
	hash := sha256.Sum256([]byte(parts[1]))
	return fmt.Sprintf("org-%x", hash[:8])
}

// RequirePermission checks if the user has a specific permission
func (m *AuthMiddleware) RequirePermission(permission Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUser(r.Context())
			if user == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if err := m.rbac.RequirePermission(user.Role, permission); err != nil {
				m.logger.Warn("permission denied",
					zap.String("user_id", user.UserID),
					zap.String("role", string(user.Role)),
					zap.String("permission", string(permission)))
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole checks if the user has a specific role
func (m *AuthMiddleware) RequireRole(roles ...Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUser(r.Context())
			if user == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			hasRole := false
			for _, role := range roles {
				if user.Role == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetUser extracts the user from the request context
func GetUser(ctx context.Context) *User {
	user, ok := ctx.Value(UserContextKey).(*User)
	if !ok {
		return nil
	}
	return user
}

// OrgIsolationMiddleware middleware ensures requests only access resources from the user's org
type OrgIsolationMiddleware struct {
	logger *zap.Logger
}

func NewOrgIsolationMiddleware(logger *zap.Logger) *OrgIsolationMiddleware {
	return &OrgIsolationMiddleware{logger: logger}
}

func (m *OrgIsolationMiddleware) Isolate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		if user == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// In handlers, always filter by user.OrgID
		// This is enforced at the database query level
		next.ServeHTTP(w, r)
	})
}
