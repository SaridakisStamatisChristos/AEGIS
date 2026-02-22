package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aegisrun/aegisrun/internal/auth"
	"github.com/aegisrun/aegisrun/internal/gateway"
	"github.com/aegisrun/aegisrun/internal/ledger"
	"github.com/aegisrun/aegisrun/internal/server"
	"github.com/aegisrun/aegisrun/internal/store"
	"github.com/aegisrun/aegisrun/internal/telemetry"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting AegisRun API server",
		zap.String("version", "1.0.0"))

	// Initialize distributed tracing (OpenTelemetry)
	tracingCfg := telemetry.DefaultTracingConfig()
	shutdownTracer, err := telemetry.InitTracer(context.Background(), tracingCfg, logger)
	if err != nil {
		logger.Fatal("failed to initialize tracing", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracer(ctx); err != nil {
			logger.Error("tracing shutdown error", zap.Error(err))
		}
	}()

	// Load configuration from environment
	cfg := loadConfig()

	// Validate configuration — fatal in production if required settings are missing
	validateConfig(cfg, logger)

	// Initialize database
	dbStore, err := store.New(store.Config{
		Host:            cfg.DBHost,
		Port:            cfg.DBPort,
		User:            cfg.DBUser,
		Password:        cfg.DBPassword,
		Database:        cfg.DBName,
		SSLMode:         cfg.DBSSLMode,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}, logger)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer dbStore.Close()

	// Initialize stores
	runStore := store.NewRunStore(dbStore)
	stepStore := store.NewStepStore(dbStore)
	toolCallStore := store.NewToolCallStore(dbStore)
	eventStore := store.NewEventStore(dbStore)
	policyStore := store.NewPolicyStore(dbStore)
	approvalStore := store.NewApprovalStore(dbStore)
	keyStore := store.NewKeyStore(dbStore)

	// Initialize gateway
	gw := gateway.NewGateway(
		dbStore,
		runStore,
		stepStore,
		toolCallStore,
		eventStore,
		policyStore,
		logger,
	)

	// Initialize ledger
	ledgerService := ledger.NewLedger(
		eventStore,
		runStore,
		policyStore,
		keyStore,
		logger,
	)

	// Initialize bundler
	bundler := ledger.NewBundler(
		eventStore,
		runStore,
		policyStore,
		keyStore,
		logger,
	)

	// Initialize auth
	var oidcProvider server.OIDCProvider

	if cfg.OIDCIssuer == "mock" {
		logger.Info("using mock OIDC provider for development")
		mockProvider, mockErr := auth.NewMockOIDCProvider(cfg.AppEnv, logger)
		if mockErr != nil {
			logger.Fatal("cannot start with mock OIDC provider", zap.Error(mockErr))
		}
		oidcProvider = mockProvider
	} else {
		oidcProvider, err = auth.NewOIDCProvider(auth.OIDCConfig{
			Issuer:       cfg.OIDCIssuer,
			Audience:     cfg.OIDCAudience,
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  cfg.OIDCRedirectURL,
			MaxTokenAge:  time.Duration(cfg.MaxTokenAgeSec) * time.Second,
		}, logger)
		if err != nil {
			logger.Fatal("failed to create OIDC provider", zap.Error(err))
		}
	}

	rbac := auth.NewRBAC()

	// Initialize telemetry
	metricsRegistry := telemetry.NewMetricsRegistry()
	metricsRegistry.Register()

	// Create server
	srv := server.New(server.Config{
		Port:            cfg.Port,
		CORSAllowOrigin: cfg.CORSAllowOrigin,
		RateLimitRPS:    cfg.RateLimitRPS,
		RateLimitBurst:  cfg.RateLimitBurst,
	}, server.Dependencies{
		Store:         dbStore,
		RunStore:      runStore,
		StepStore:     stepStore,
		ToolCallStore: toolCallStore,
		EventStore:    eventStore,
		PolicyStore:   policyStore,
		ApprovalStore: approvalStore,
		KeyStore:      keyStore,
		Gateway:       gw,
		Ledger:        ledgerService,
		Bundler:       bundler,
		OIDC:          oidcProvider,
		RBAC:          rbac,
		Metrics:       metricsRegistry,
		Logger:        logger,
	})

	// Start server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		logger.Info("server listening", zap.Int("port", cfg.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", zap.Error(err))
	}

	logger.Info("server stopped")
}

type Config struct {
	AppEnv           string
	Port             int
	DBHost           string
	DBPort           int
	DBUser           string
	DBPassword       string
	DBName           string
	DBSSLMode        string
	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	OIDCAudience     string
	MaxTokenAgeSec   int
	CORSAllowOrigin  string
	RateLimitRPS     float64
	RateLimitBurst   int
}

func loadConfig() Config {
	return Config{
		AppEnv:           getEnv("APP_ENV", "development"),
		Port:             getEnvInt("PORT", 8080),
		DBHost:           getEnv("DB_HOST", "localhost"),
		DBPort:           getEnvInt("DB_PORT", 5432),
		DBUser:           getEnv("DB_USER", "aegisrun"),
		DBPassword:       getEnv("DB_PASSWORD", "aegisrun"),
		DBName:           getEnv("DB_NAME", "aegisrun"),
		DBSSLMode:        getEnv("DB_SSL_MODE", "disable"),
		OIDCIssuer:       getEnv("OIDC_ISSUER", "mock"),
		OIDCClientID:     getEnv("OIDC_CLIENT_ID", ""),
		OIDCClientSecret: getEnv("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:  getEnv("OIDC_REDIRECT_URL", "http://localhost:8080/auth/callback"),
		OIDCAudience:     getEnv("OIDC_AUDIENCE", ""),
		MaxTokenAgeSec:   getEnvInt("MAX_TOKEN_AGE_SECONDS", 3600),
		CORSAllowOrigin:  getEnv("CORS_ALLOW_ORIGIN", "http://localhost:5173"),
		RateLimitRPS:     getEnvFloat("RATE_LIMIT_RPS", 100),
		RateLimitBurst:   getEnvInt("RATE_LIMIT_BURST", 200),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return defaultValue
		}
		return intValue
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return defaultValue
		}
		return f
	}
	return defaultValue
}

// validateConfig checks for required production settings and logs fatal if any
// critical configuration is missing or insecure when APP_ENV=production.
func validateConfig(cfg Config, logger *zap.Logger) {
	isProduction := strings.EqualFold(cfg.AppEnv, "production")

	if !isProduction {
		logger.Info("running in non-production mode, skipping strict config validation",
			zap.String("APP_ENV", cfg.AppEnv))
		return
	}

	logger.Info("validating production configuration", zap.String("APP_ENV", cfg.AppEnv))

	var fatal []string

	// OIDC must not be mock in production
	if cfg.OIDCIssuer == "" || cfg.OIDCIssuer == "mock" {
		fatal = append(fatal, "OIDC_ISSUER must be set to a real issuer URL in production (got: \""+cfg.OIDCIssuer+"\")")
	}
	if cfg.OIDCClientID == "" {
		fatal = append(fatal, "OIDC_CLIENT_ID is required in production")
	}
	if cfg.OIDCClientSecret == "" {
		fatal = append(fatal, "OIDC_CLIENT_SECRET is required in production")
	}

	// Audience should be explicitly set in production
	// (falls back to OIDC_CLIENT_ID at runtime, but explicit config is preferred)
	if cfg.OIDCAudience == "" && cfg.OIDCClientID == "" {
		fatal = append(fatal, "OIDC_AUDIENCE (or OIDC_CLIENT_ID) must be set in production for token audience validation")
	}

	// Database credentials must not be defaults
	if cfg.DBPassword == "" || cfg.DBPassword == "aegisrun" {
		fatal = append(fatal, "DB_PASSWORD must be set to a non-default value in production")
	}
	if cfg.DBSSLMode == "disable" {
		fatal = append(fatal, "DB_SSL_MODE must not be \"disable\" in production (use \"require\" or \"verify-full\")")
	}

	// CORS must be explicitly configured
	if cfg.CORSAllowOrigin == "" || cfg.CORSAllowOrigin == "*" {
		fatal = append(fatal, "CORS_ALLOW_ORIGIN must be set to explicit origin(s) in production (not empty or \"*\")")
	}

	// Rate limiting should be enabled
	if cfg.RateLimitRPS <= 0 {
		fatal = append(fatal, "RATE_LIMIT_RPS must be > 0 in production")
	}

	if len(fatal) > 0 {
		for _, msg := range fatal {
			logger.Error("production config validation failure", zap.String("issue", msg))
		}
		logger.Fatal("aborting startup: production configuration validation failed",
			zap.Int("errors", len(fatal)),
			zap.Strings("issues", fatal))
	}

	logger.Info("production configuration validated successfully")
}
