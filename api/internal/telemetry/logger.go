package telemetry

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogConfig struct {
	Level       string
	Development bool
	Encoding    string // "json" or "console"
}

func NewLogger(cfg LogConfig) (*zap.Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Build encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Build encoder
	var encoder zapcore.Encoder
	if cfg.Encoding == "console" || cfg.Development {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Build core
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	// Build logger
	opts := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	}

	if cfg.Development {
		opts = append(opts, zap.Development())
	}

	return zap.New(core, opts...), nil
}

func NewDevelopmentLogger() (*zap.Logger, error) {
	return NewLogger(LogConfig{
		Level:       "debug",
		Development: true,
		Encoding:    "console",
	})
}

func NewProductionLogger() (*zap.Logger, error) {
	return NewLogger(LogConfig{
		Level:       getEnvOrDefault("LOG_LEVEL", "info"),
		Development: false,
		Encoding:    "json",
	})
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// LoggerWith creates a child logger with additional fields
func LoggerWith(logger *zap.Logger, fields ...zap.Field) *zap.Logger {
	return logger.With(fields...)
}

// RequestLogger creates a logger for HTTP request tracing
func RequestLogger(logger *zap.Logger, requestID, method, path string) *zap.Logger {
	return logger.With(
		zap.String("request_id", requestID),
		zap.String("method", method),
		zap.String("path", path),
	)
}

// RunLogger creates a logger for run operations
func RunLogger(logger *zap.Logger, orgID, runID string) *zap.Logger {
	return logger.With(
		zap.String("org_id", orgID),
		zap.String("run_id", runID),
	)
}

// ToolCallLogger creates a logger for tool call operations
func ToolCallLogger(logger *zap.Logger, runID, toolCallID, toolName string) *zap.Logger {
	return logger.With(
		zap.String("run_id", runID),
		zap.String("tool_call_id", toolCallID),
		zap.String("tool_name", toolName),
	)
}

// PolicyLogger creates a logger for policy operations
func PolicyLogger(logger *zap.Logger, policyID, policyName string) *zap.Logger {
	return logger.With(
		zap.String("policy_id", policyID),
		zap.String("policy_name", policyName),
	)
}
