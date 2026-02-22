package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsRegistry struct {
	// HTTP metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge

	// Gateway metrics
	ToolCallsTotal       *prometheus.CounterVec
	ToolCallDuration     *prometheus.HistogramVec
	PolicyEvalDuration   *prometheus.HistogramVec
	PolicyDecisions      *prometheus.CounterVec

	// Approval metrics
	ApprovalsCreated     *prometheus.CounterVec
	ApprovalsResolved    *prometheus.CounterVec
	ApprovalsPending     prometheus.Gauge

	// Ledger metrics
	EventsCreated        *prometheus.CounterVec
	BundlesExported      prometheus.Counter
	ChainVerifications   *prometheus.CounterVec

	// Database metrics
	DBQueryDuration      *prometheus.HistogramVec
	DBConnectionsActive  prometheus.Gauge
	DBConnectionsIdle    prometheus.Gauge
}

func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		// HTTP metrics
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "aegisrun",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "aegisrun",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		HTTPRequestsInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "aegisrun",
				Subsystem: "http",
				Name:      "requests_in_flight",
				Help:      "Number of HTTP requests currently being processed",
			},
		),

		// Gateway metrics
		ToolCallsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "aegisrun",
				Subsystem: "gateway",
				Name:      "tool_calls_total",
				Help:      "Total number of tool calls processed",
			},
			[]string{"tool", "decision"},
		),
		ToolCallDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "aegisrun",
				Subsystem: "gateway",
				Name:      "tool_call_duration_seconds",
				Help:      "Tool call processing duration in seconds",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"tool"},
		),
		PolicyEvalDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "aegisrun",
				Subsystem: "gateway",
				Name:      "policy_eval_duration_seconds",
				Help:      "Policy evaluation duration in seconds",
				Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1},
			},
			[]string{"policy"},
		),
		PolicyDecisions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "aegisrun",
				Subsystem: "gateway",
				Name:      "policy_decisions_total",
				Help:      "Total number of policy decisions",
			},
			[]string{"policy", "rule", "decision"},
		),

		// Approval metrics
		ApprovalsCreated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "aegisrun",
				Subsystem: "approvals",
				Name:      "created_total",
				Help:      "Total number of approvals created",
			},
			[]string{"policy"},
		),
		ApprovalsResolved: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "aegisrun",
				Subsystem: "approvals",
				Name:      "resolved_total",
				Help:      "Total number of approvals resolved",
			},
			[]string{"status"},
		),
		ApprovalsPending: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "aegisrun",
				Subsystem: "approvals",
				Name:      "pending",
				Help:      "Number of pending approvals",
			},
		),

		// Ledger metrics
		EventsCreated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "aegisrun",
				Subsystem: "ledger",
				Name:      "events_created_total",
				Help:      "Total number of events created",
			},
			[]string{"event_type"},
		),
		BundlesExported: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "aegisrun",
				Subsystem: "ledger",
				Name:      "bundles_exported_total",
				Help:      "Total number of evidence bundles exported",
			},
		),
		ChainVerifications: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "aegisrun",
				Subsystem: "ledger",
				Name:      "chain_verifications_total",
				Help:      "Total number of chain verifications",
			},
			[]string{"result"},
		),

		// Database metrics
		DBQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "aegisrun",
				Subsystem: "db",
				Name:      "query_duration_seconds",
				Help:      "Database query duration in seconds",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"query"},
		),
		DBConnectionsActive: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "aegisrun",
				Subsystem: "db",
				Name:      "connections_active",
				Help:      "Number of active database connections",
			},
		),
		DBConnectionsIdle: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "aegisrun",
				Subsystem: "db",
				Name:      "connections_idle",
				Help:      "Number of idle database connections",
			},
		),
	}
}

func (m *MetricsRegistry) Register() {
	prometheus.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.HTTPRequestsInFlight,
		m.ToolCallsTotal,
		m.ToolCallDuration,
		m.PolicyEvalDuration,
		m.PolicyDecisions,
		m.ApprovalsCreated,
		m.ApprovalsResolved,
		m.ApprovalsPending,
		m.EventsCreated,
		m.BundlesExported,
		m.ChainVerifications,
		m.DBQueryDuration,
		m.DBConnectionsActive,
		m.DBConnectionsIdle,
	)
}

func (m *MetricsRegistry) RecordToolCall(tool, decision string) {
	m.ToolCallsTotal.WithLabelValues(tool, decision).Inc()
}

func (m *MetricsRegistry) RecordPolicyDecision(policy, rule, decision string) {
	m.PolicyDecisions.WithLabelValues(policy, rule, decision).Inc()
}

func (m *MetricsRegistry) RecordEvent(eventType string) {
	m.EventsCreated.WithLabelValues(eventType).Inc()
}
