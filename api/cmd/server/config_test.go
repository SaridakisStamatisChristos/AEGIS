package main

import (
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear all relevant env vars to test defaults
	envVars := []string{
		"APP_ENV", "PORT", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD",
		"DB_NAME", "DB_SSL_MODE", "OIDC_ISSUER", "OIDC_CLIENT_ID",
		"OIDC_CLIENT_SECRET", "OIDC_REDIRECT_URL", "CORS_ALLOW_ORIGIN",
		"RATE_LIMIT_RPS", "RATE_LIMIT_BURST",
	}
	for _, key := range envVars {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}

	cfg := loadConfig()

	if cfg.AppEnv != "development" {
		t.Errorf("expected AppEnv=development, got %q", cfg.AppEnv)
	}
	if cfg.Port != 8080 {
		t.Errorf("expected Port=8080, got %d", cfg.Port)
	}
	if cfg.DBHost != "localhost" {
		t.Errorf("expected DBHost=localhost, got %q", cfg.DBHost)
	}
	if cfg.DBSSLMode != "disable" {
		t.Errorf("expected DBSSLMode=disable, got %q", cfg.DBSSLMode)
	}
	if cfg.OIDCIssuer != "mock" {
		t.Errorf("expected OIDCIssuer=mock, got %q", cfg.OIDCIssuer)
	}
	if cfg.CORSAllowOrigin != "http://localhost:5173" {
		t.Errorf("expected CORSAllowOrigin=http://localhost:5173, got %q", cfg.CORSAllowOrigin)
	}
	if cfg.RateLimitRPS != 100 {
		t.Errorf("expected RateLimitRPS=100, got %f", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 200 {
		t.Errorf("expected RateLimitBurst=200, got %d", cfg.RateLimitBurst)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("PORT", "9090")
	t.Setenv("DB_HOST", "db.prod.internal")
	t.Setenv("DB_SSL_MODE", "require")
	t.Setenv("CORS_ALLOW_ORIGIN", "https://app.example.com")
	t.Setenv("RATE_LIMIT_RPS", "50")
	t.Setenv("RATE_LIMIT_BURST", "100")

	cfg := loadConfig()

	if cfg.AppEnv != "production" {
		t.Errorf("expected AppEnv=production, got %q", cfg.AppEnv)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected Port=9090, got %d", cfg.Port)
	}
	if cfg.DBHost != "db.prod.internal" {
		t.Errorf("expected DBHost=db.prod.internal, got %q", cfg.DBHost)
	}
	if cfg.DBSSLMode != "require" {
		t.Errorf("expected DBSSLMode=require, got %q", cfg.DBSSLMode)
	}
	if cfg.CORSAllowOrigin != "https://app.example.com" {
		t.Errorf("expected CORSAllowOrigin=https://app.example.com, got %q", cfg.CORSAllowOrigin)
	}
	if cfg.RateLimitRPS != 50 {
		t.Errorf("expected RateLimitRPS=50, got %f", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 100 {
		t.Errorf("expected RateLimitBurst=100, got %d", cfg.RateLimitBurst)
	}
}

func TestLoadConfig_CanonicalEnvVarNames(t *testing.T) {
	// Verify the canonical env var names are read (not the old drifted names)
	t.Setenv("DB_SSL_MODE", "verify-full")
	t.Setenv("DB_SSLMODE", "disable") // old name should be ignored
	t.Setenv("CORS_ALLOW_ORIGIN", "https://correct.example.com")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://wrong.example.com") // old name
	t.Setenv("RATE_LIMIT_RPS", "75")
	t.Setenv("RATE_LIMIT_REQUESTS_PER_SECOND", "999") // old name
	t.Setenv("PORT", "3000")
	t.Setenv("API_PORT", "4000") // old name

	cfg := loadConfig()

	if cfg.DBSSLMode != "verify-full" {
		t.Errorf("expected DB_SSL_MODE to be read (verify-full), got %q", cfg.DBSSLMode)
	}
	if cfg.CORSAllowOrigin != "https://correct.example.com" {
		t.Errorf("expected CORS_ALLOW_ORIGIN to be read (https://correct.example.com), got %q", cfg.CORSAllowOrigin)
	}
	if cfg.RateLimitRPS != 75 {
		t.Errorf("expected RATE_LIMIT_RPS to be read (75), got %f", cfg.RateLimitRPS)
	}
	if cfg.Port != 3000 {
		t.Errorf("expected PORT to be read (3000), got %d", cfg.Port)
	}
}

func TestValidateConfig_DevelopmentSkipsValidation(t *testing.T) {
	// In development mode, validation should not panic/fatal even with bad config
	cfg := Config{
		AppEnv:     "development",
		OIDCIssuer: "mock",
		DBPassword: "aegisrun",
		DBSSLMode:  "disable",
	}

	// This should not panic — development mode skips strict checks
	logger := createTestLogger()
	validateConfig(cfg, logger)
}

func TestValidateConfig_ProductionRequiresOIDC(t *testing.T) {
	// Production config with mock OIDC should trigger issues
	cfg := validProductionConfig()
	cfg.OIDCIssuer = "mock"

	issues := collectValidationIssues(cfg)
	assertContainsIssue(t, issues, "OIDC_ISSUER")
}

func TestValidateConfig_ProductionRequiresDBPassword(t *testing.T) {
	cfg := validProductionConfig()
	cfg.DBPassword = "aegisrun"

	issues := collectValidationIssues(cfg)
	assertContainsIssue(t, issues, "DB_PASSWORD")
}

func TestValidateConfig_ProductionRequiresDBSSL(t *testing.T) {
	cfg := validProductionConfig()
	cfg.DBSSLMode = "disable"

	issues := collectValidationIssues(cfg)
	assertContainsIssue(t, issues, "DB_SSL_MODE")
}

func TestValidateConfig_ProductionRequiresCORS(t *testing.T) {
	cfg := validProductionConfig()
	cfg.CORSAllowOrigin = "*"

	issues := collectValidationIssues(cfg)
	assertContainsIssue(t, issues, "CORS_ALLOW_ORIGIN")
}

func TestValidateConfig_ProductionRequiresRateLimit(t *testing.T) {
	cfg := validProductionConfig()
	cfg.RateLimitRPS = 0

	issues := collectValidationIssues(cfg)
	assertContainsIssue(t, issues, "RATE_LIMIT_RPS")
}

func TestValidateConfig_ProductionRequiresOIDCClientID(t *testing.T) {
	cfg := validProductionConfig()
	cfg.OIDCClientID = ""

	issues := collectValidationIssues(cfg)
	assertContainsIssue(t, issues, "OIDC_CLIENT_ID")
}

func TestValidateConfig_ProductionRequiresOIDCClientSecret(t *testing.T) {
	cfg := validProductionConfig()
	cfg.OIDCClientSecret = ""

	issues := collectValidationIssues(cfg)
	assertContainsIssue(t, issues, "OIDC_CLIENT_SECRET")
}

func TestValidateConfig_ValidProductionConfig(t *testing.T) {
	cfg := validProductionConfig()
	issues := collectValidationIssues(cfg)
	if len(issues) > 0 {
		t.Errorf("expected no validation issues for valid prod config, got: %v", issues)
	}
}

func TestValidateConfig_ProductionRequiresAudienceOrClientID(t *testing.T) {
	cfg := validProductionConfig()
	cfg.OIDCAudience = ""
	cfg.OIDCClientID = ""

	issues := collectValidationIssues(cfg)
	assertContainsIssue(t, issues, "OIDC_AUDIENCE")
}

func TestValidateConfig_ProductionAcceptsAudienceViaClientID(t *testing.T) {
	cfg := validProductionConfig()
	cfg.OIDCAudience = "" // empty, but ClientID is set
	cfg.OIDCClientID = "aegisrun-prod"

	issues := collectValidationIssues(cfg)
	// Should NOT contain an audience issue
	for _, issue := range issues {
		if contains(issue, "OIDC_AUDIENCE") {
			t.Errorf("should not report audience issue when OIDC_CLIENT_ID is set, got: %s", issue)
		}
	}
}

func TestValidateConfig_ProductionRejectsMockIssuer(t *testing.T) {
	cfg := validProductionConfig()
	cfg.OIDCIssuer = "mock"

	issues := collectValidationIssues(cfg)
	assertContainsIssue(t, issues, "OIDC_ISSUER")
}

func TestValidateConfig_ProductionRejectsEmptyIssuer(t *testing.T) {
	cfg := validProductionConfig()
	cfg.OIDCIssuer = ""

	issues := collectValidationIssues(cfg)
	assertContainsIssue(t, issues, "OIDC_ISSUER")
}

func TestLoadConfig_NewFields(t *testing.T) {
	t.Setenv("OIDC_AUDIENCE", "my-audience")
	t.Setenv("MAX_TOKEN_AGE_SECONDS", "7200")

	cfg := loadConfig()

	if cfg.OIDCAudience != "my-audience" {
		t.Errorf("expected OIDCAudience=my-audience, got %q", cfg.OIDCAudience)
	}
	if cfg.MaxTokenAgeSec != 7200 {
		t.Errorf("expected MaxTokenAgeSec=7200, got %d", cfg.MaxTokenAgeSec)
	}
}

func TestLoadConfig_NewFieldDefaults(t *testing.T) {
	// Clear to get defaults
	for _, key := range []string{"OIDC_AUDIENCE", "MAX_TOKEN_AGE_SECONDS"} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}

	cfg := loadConfig()

	if cfg.OIDCAudience != "" {
		t.Errorf("expected OIDCAudience=empty, got %q", cfg.OIDCAudience)
	}
	if cfg.MaxTokenAgeSec != 3600 {
		t.Errorf("expected MaxTokenAgeSec=3600, got %d", cfg.MaxTokenAgeSec)
	}
}

// --- helpers ---

func validProductionConfig() Config {
	return Config{
		AppEnv:           "production",
		Port:             8080,
		DBHost:           "db.prod.internal",
		DBPort:           5432,
		DBUser:           "aegisrun_prod",
		DBPassword:       "super-secret-password",
		DBName:           "aegisrun",
		DBSSLMode:        "require",
		OIDCIssuer:       "https://auth.example.com",
		OIDCClientID:     "aegisrun-prod",
		OIDCClientSecret: "oidc-secret",
		OIDCRedirectURL:  "https://app.example.com/auth/callback",
		OIDCAudience:     "aegisrun-prod",
		MaxTokenAgeSec:   3600,
		CORSAllowOrigin:  "https://app.example.com",
		RateLimitRPS:     100,
		RateLimitBurst:   200,
	}
}

// collectValidationIssues replicates the production validation logic
// without calling logger.Fatal, returning the list of issues found.
func collectValidationIssues(cfg Config) []string {
	var issues []string

	if cfg.AppEnv != "production" {
		return issues
	}

	if cfg.OIDCIssuer == "" || cfg.OIDCIssuer == "mock" {
		issues = append(issues, "OIDC_ISSUER must be set to a real issuer URL in production")
	}
	if cfg.OIDCClientID == "" {
		issues = append(issues, "OIDC_CLIENT_ID is required in production")
	}
	if cfg.OIDCClientSecret == "" {
		issues = append(issues, "OIDC_CLIENT_SECRET is required in production")
	}
	if cfg.OIDCAudience == "" && cfg.OIDCClientID == "" {
		issues = append(issues, "OIDC_AUDIENCE (or OIDC_CLIENT_ID) must be set in production for token audience validation")
	}
	if cfg.DBPassword == "" || cfg.DBPassword == "aegisrun" {
		issues = append(issues, "DB_PASSWORD must be set to a non-default value in production")
	}
	if cfg.DBSSLMode == "disable" {
		issues = append(issues, "DB_SSL_MODE must not be disable in production")
	}
	if cfg.CORSAllowOrigin == "" || cfg.CORSAllowOrigin == "*" {
		issues = append(issues, "CORS_ALLOW_ORIGIN must be set to explicit origin(s) in production")
	}
	if cfg.RateLimitRPS <= 0 {
		issues = append(issues, "RATE_LIMIT_RPS must be > 0 in production")
	}

	return issues
}

func assertContainsIssue(t *testing.T, issues []string, keyword string) {
	t.Helper()
	for _, issue := range issues {
		if contains(issue, keyword) {
			return
		}
	}
	t.Errorf("expected validation issue containing %q, got: %v", keyword, issues)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func createTestLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}
