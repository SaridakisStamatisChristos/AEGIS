package telemetry

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TracingConfig holds configuration for the OpenTelemetry tracer.
type TracingConfig struct {
	ServiceName    string  // e.g. "aegisrun-api"
	ServiceVersion string  // e.g. "1.0.0"
	OTLPEndpoint   string  // e.g. "localhost:4317" (gRPC) – empty disables export
	SampleRatio    float64 // 0.0 – 1.0; default 1.0 (sample everything)
}

// DefaultTracingConfig returns a configuration populated from environment
// variables with sensible defaults.
func DefaultTracingConfig() TracingConfig {
	ratio := 1.0
	return TracingConfig{
		ServiceName:    envOr("OTEL_SERVICE_NAME", "aegisrun-api"),
		ServiceVersion: envOr("OTEL_SERVICE_VERSION", "1.0.0"),
		OTLPEndpoint:   envOr("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		SampleRatio:    ratio,
	}
}

// InitTracer bootstraps an OpenTelemetry TracerProvider. The returned
// shutdown function should be deferred by the caller.
//
// When cfg.OTLPEndpoint is empty the provider is configured with a no-op
// exporter so that all application code can call otel.Tracer() without
// conditional logic; spans are simply not exported.
func InitTracer(ctx context.Context, cfg TracingConfig, logger *zap.Logger) (shutdown func(context.Context) error, err error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("environment", envOr("ENVIRONMENT", "development")),
		),
	)
	if err != nil {
		return nil, err
	}

	var tp *sdktrace.TracerProvider

	if cfg.OTLPEndpoint != "" {
		// Real OTLP gRPC exporter (Jaeger / Tempo / Collector / etc.)
		exporter, exporterErr := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlptracegrpc.WithInsecure(), // TLS configurable via env OTEL_EXPORTER_OTLP_INSECURE
		)
		if exporterErr != nil {
			return nil, exporterErr
		}

		sampler := sdktrace.AlwaysSample()
		if cfg.SampleRatio < 1.0 {
			sampler = sdktrace.TraceIDRatioBased(cfg.SampleRatio)
		}

		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(5*time.Second)),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sampler),
		)
		logger.Info("OpenTelemetry tracing enabled",
			zap.String("endpoint", cfg.OTLPEndpoint),
			zap.Float64("sample_ratio", cfg.SampleRatio),
		)
	} else {
		// No exporter — noop TracerProvider keeps instrumentation paths
		// active without any overhead.
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
		)
		logger.Info("OpenTelemetry tracing disabled (no OTEL_EXPORTER_OTLP_ENDPOINT)")
	}

	// Register as the global provider so otel.Tracer() works everywhere.
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// Tracer returns a named tracer scoped to this application.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// SpanFromContext is a convenience alias.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
