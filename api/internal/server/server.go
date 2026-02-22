package server

import (
	"context"
	"net/http"
	"time"

	"github.com/aegisrun/aegisrun/internal/auth"
	"github.com/aegisrun/aegisrun/internal/gateway"
	"github.com/aegisrun/aegisrun/internal/ledger"
	"github.com/aegisrun/aegisrun/internal/server/handlers"
	"github.com/aegisrun/aegisrun/internal/store"
	"github.com/aegisrun/aegisrun/internal/telemetry"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type Config struct {
	Port            int
	CORSAllowOrigin string
	RateLimitRPS    float64 // requests per second per IP (0 = disabled)
	RateLimitBurst  int     // burst bucket size per IP
}

// OIDCProvider is the interface that both real and mock OIDC providers implement.
type OIDCProvider interface {
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error)
	VerifyIDToken(ctx context.Context, rawIDToken string) (*auth.Claims, error)
}

type Dependencies struct {
	Store         *store.Store
	RunStore      *store.RunStore
	StepStore     *store.StepStore
	ToolCallStore *store.ToolCallStore
	EventStore    *store.EventStore
	PolicyStore   *store.PolicyStore
	ApprovalStore *store.ApprovalStore
	KeyStore      *store.KeyStore
	Gateway       *gateway.Gateway
	Ledger        *ledger.Ledger
	Bundler       *ledger.Bundler
	OIDC          OIDCProvider
	RBAC          *auth.RBAC
	Metrics       *telemetry.MetricsRegistry
	Logger        *zap.Logger
}

type Server struct {
	config Config
	deps   Dependencies
	router *chi.Mux
	logger *zap.Logger
}

func New(config Config, deps Dependencies) *Server {
	s := &Server{
		config: config,
		deps:   deps,
		router: chi.NewRouter(),
		logger: deps.Logger,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) setupMiddleware() {
	// Request ID
	s.router.Use(middleware.RequestID)

	// Real IP
	s.router.Use(middleware.RealIP)

	// Distributed tracing (OpenTelemetry)
	s.router.Use(TracingMiddleware())

	// Security headers (X-Frame-Options, CSP, etc.)
	s.router.Use(HeadersMiddleware)

	// Structured logging
	s.router.Use(RequestLogger(s.logger))

	// Recover from panics
	s.router.Use(RecoveryMiddleware(s.logger))

	// CORS
	s.router.Use(CORSMiddleware(s.config.CORSAllowOrigin))

	// Metrics
	s.router.Use(MetricsMiddleware(s.deps.Metrics))

	// Per-IP rate limiting
	s.router.Use(RateLimitMiddleware(s.config.RateLimitRPS, s.config.RateLimitBurst))

	// Request timeout
	s.router.Use(middleware.Timeout(60 * time.Second))
}

func (s *Server) setupRoutes() {
	// Health check (public)
	healthHandler := handlers.NewHealthHandler(s.deps.Store)
	s.router.Get("/health", healthHandler.Health)
	s.router.Get("/ready", healthHandler.Ready)

	// Metrics (public for Prometheus scraping)
	s.router.Handle("/metrics", promhttp.Handler())

	// Create auth middleware instances
	authMw := auth.NewAuthMiddleware(s.deps.OIDC, s.deps.RBAC, s.logger)
	orgMw := auth.NewOrgIsolationMiddleware(s.logger)

	// API routes
	s.router.Route("/api/v1", func(r chi.Router) {
		// Auth middleware for all API routes
		r.Use(authMw.Authenticate)

		// Runs
		runsHandler := handlers.NewRunsHandler(
			s.deps.RunStore,
			s.deps.StepStore,
			s.deps.ToolCallStore,
			s.deps.EventStore,
			s.logger,
		)
		r.Route("/runs", func(r chi.Router) {
			r.Use(orgMw.Isolate)
			r.With(authMw.RequirePermission(auth.PermRunView)).Get("/", runsHandler.List)
			r.With(authMw.RequirePermission(auth.PermRunCreate)).Post("/", runsHandler.Create)
			r.Route("/{runID}", func(r chi.Router) {
				r.With(authMw.RequirePermission(auth.PermRunView)).Get("/", runsHandler.Get)
				r.With(authMw.RequirePermission(auth.PermRunView)).Get("/steps", runsHandler.ListSteps)
				r.With(authMw.RequirePermission(auth.PermRunView)).Get("/events", runsHandler.ListEvents)
				r.With(authMw.RequirePermission(auth.PermRunCreate)).Post("/events", runsHandler.SubmitEvent)
			})
		})

		// Policies
		policiesHandler := handlers.NewPoliciesHandler(
			s.deps.PolicyStore,
			s.logger,
		)
		r.Route("/policies", func(r chi.Router) {
			r.Use(orgMw.Isolate)
			r.With(authMw.RequirePermission(auth.PermPolicyView)).Get("/", policiesHandler.List)
			r.With(authMw.RequirePermission(auth.PermPolicyCreate)).Post("/", policiesHandler.Create)
			r.Route("/{policyID}", func(r chi.Router) {
				r.With(authMw.RequirePermission(auth.PermPolicyView)).Get("/", policiesHandler.Get)
				r.With(authMw.RequirePermission(auth.PermPolicyEdit)).Put("/", policiesHandler.Update)
				r.With(authMw.RequirePermission(auth.PermPolicyEdit)).Delete("/", policiesHandler.Delete)
				r.With(authMw.RequirePermission(auth.PermPolicyDeploy)).Post("/activate", policiesHandler.Activate)
				r.With(authMw.RequirePermission(auth.PermPolicyDeploy)).Post("/deactivate", policiesHandler.Deactivate)
			})
		})

		// Approvals
		approvalsHandler := handlers.NewApprovalsHandler(
			s.deps.ApprovalStore,
			s.deps.PolicyStore,
			s.logger,
		)
		r.Route("/approvals", func(r chi.Router) {
			r.Use(orgMw.Isolate)
			r.With(authMw.RequirePermission(auth.PermPolicyView)).Get("/", approvalsHandler.List)
			r.With(authMw.RequirePermission(auth.PermPolicyView)).Get("/{approvalID}", approvalsHandler.Get)
			r.With(authMw.RequirePermission(auth.PermPolicyApprove)).Post("/policies/{policyID}/approve", approvalsHandler.Approve)
			r.With(authMw.RequirePermission(auth.PermPolicyApprove)).Post("/policies/{policyID}/reject", approvalsHandler.Reject)
		})

		// Evidence
		evidenceHandler := handlers.NewEvidenceHandler(
			s.deps.Bundler,
			s.deps.EventStore,
			s.logger,
		)
		r.Route("/evidence", func(r chi.Router) {
			r.Use(orgMw.Isolate)
			r.With(authMw.RequirePermission(auth.PermEvidenceExport)).
				Get("/runs/{runID}/bundle", evidenceHandler.ExportBundle)
			r.With(authMw.RequirePermission(auth.PermEvidenceView)).
				Post("/verify", evidenceHandler.VerifyBundle)
		})

		// Gateway (agent requests)
		gatewayHandler := handlers.NewGatewayHandler(
			s.deps.Gateway,
			s.logger,
		)
		r.Route("/gateway", func(r chi.Router) {
			r.Post("/execute", gatewayHandler.Execute)
		})

		// Stats (server-side aggregation)
		statsHandler := handlers.NewStatsHandler(s.deps.RunStore, s.logger)
		r.With(authMw.RequirePermission(auth.PermRunView)).Get("/stats", statsHandler.Get)
	})
}
