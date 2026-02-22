package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// ErrMockProviderInProduction is returned when MockOIDCProvider is
// instantiated with APP_ENV=production. This is a hard security gate to
// prevent mock authentication from ever running in a production environment.
var ErrMockProviderInProduction = errors.New("MockOIDCProvider cannot be used when APP_ENV=production")

// OIDCConfig holds OIDC provider configuration
type OIDCConfig struct {
	Issuer       string
	Audience     string // If set, overrides ClientID for "aud" claim validation
	ClientID     string
	ClientSecret string
	RedirectURL  string
	MaxTokenAge  time.Duration // Maximum acceptable age since "iat"; 0 disables the check
}

// OIDCProvider handles OIDC authentication
type OIDCProvider struct {
	config   OIDCConfig
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth2   oauth2.Config
	logger   *zap.Logger
}

func NewOIDCProvider(cfg OIDCConfig, logger *zap.Logger) (*OIDCProvider, error) {
	ctx := context.Background()

	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("create OIDC provider: %w", err)
	}

	oauth2Config := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	// Use explicit audience for token verification, defaulting to ClientID.
	audience := cfg.Audience
	if audience == "" {
		audience = cfg.ClientID
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: audience})

	return &OIDCProvider{
		config:   cfg,
		provider: provider,
		verifier: verifier,
		oauth2:   oauth2Config,
		logger:   logger,
	}, nil
}

// GetAuthURL returns the OAuth2 authorization URL
func (p *OIDCProvider) GetAuthURL(state string) string {
	return p.oauth2.AuthCodeURL(state)
}

// ExchangeCode exchanges an authorization code for tokens
func (p *OIDCProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := p.oauth2.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	return token, nil
}

// VerifyIDToken verifies and extracts claims from an ID token.
// In addition to the cryptographic/expiry checks done by the go-oidc verifier,
// this method enforces:
//   - Issuer must match the configured OIDC issuer.
//   - Token age (time since "iat") must not exceed MaxTokenAge when set.
func (p *OIDCProvider) VerifyIDToken(ctx context.Context, rawIDToken string) (*Claims, error) {
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify ID token: %w", err)
	}

	var claims Claims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	// Validate issuer matches expected configuration
	if claims.Issuer != "" && claims.Issuer != p.config.Issuer {
		return nil, fmt.Errorf("token issuer mismatch: got %q, expected %q", claims.Issuer, p.config.Issuer)
	}

	// Enforce maximum token age if configured
	if p.config.MaxTokenAge > 0 && claims.IssuedAt > 0 {
		issuedAt := time.Unix(claims.IssuedAt, 0)
		tokenAge := time.Since(issuedAt)
		if tokenAge > p.config.MaxTokenAge {
			return nil, fmt.Errorf("token too old: issued %s ago, max allowed %s",
				tokenAge.Truncate(time.Second), p.config.MaxTokenAge)
		}
	}

	return &claims, nil
}

// Claims represents OIDC ID token claims
type Claims struct {
	Subject   string `json:"sub"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Picture   string `json:"picture"`
	Issuer    string `json:"iss,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
	// Custom claims that OIDC providers can be configured to include
	OrgID string   `json:"org_id,omitempty"`
	Roles []string `json:"roles,omitempty"`
}

// MockOIDCProvider is a local development mock
type MockOIDCProvider struct {
	logger *zap.Logger
}

// NewMockOIDCProvider creates a mock OIDC provider for local development.
// It returns ErrMockProviderInProduction when appEnv is "production",
// providing a hard gate that prevents mock auth in production even if
// validateConfig is somehow bypassed.
func NewMockOIDCProvider(appEnv string, logger *zap.Logger) (*MockOIDCProvider, error) {
	if strings.EqualFold(appEnv, "production") {
		return nil, ErrMockProviderInProduction
	}
	logger.Warn("MockOIDCProvider is active — this MUST NOT be used in production",
		zap.String("APP_ENV", appEnv))
	return &MockOIDCProvider{logger: logger}, nil
}

func (m *MockOIDCProvider) GetAuthURL(state string) string {
	return fmt.Sprintf("/mock-login?state=%s", state)
}

func (m *MockOIDCProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	// Mock token
	return &oauth2.Token{
		AccessToken: "mock-access-token",
		TokenType:   "Bearer",
	}, nil
}

func (m *MockOIDCProvider) VerifyIDToken(ctx context.Context, rawIDToken string) (*Claims, error) {
	// Mock claims
	return &Claims{
		Subject: "mock-user-123",
		Email:   "dev@aegisrun.local",
		Name:    "Dev User",
	}, nil
}

// HandleMockLogin handles the mock login flow for local dev
func (m *MockOIDCProvider) HandleMockLogin(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	redirectURL := fmt.Sprintf("/auth/callback?code=mock-code&state=%s", state)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
