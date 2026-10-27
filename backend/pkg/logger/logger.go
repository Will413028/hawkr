package logger

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ctxKey struct{}

// NewLogger creates a new Zap logger based on the environment.
// All environments output JSON format for unified Loki parsing.
// Non-production environments use DEBUG level; production uses INFO level.
func NewLogger(env string) *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	if env != "production" && env != "prod" {
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}

	logger, err := cfg.Build(
		zap.AddCallerSkip(0),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.WrapCore(newRedactingCore),
	)
	if err != nil {
		return zap.NewExample()
	}

	return logger
}

// NewLoggerFromEnv creates a logger based on the ENV environment variable.
func NewLoggerFromEnv() *zap.Logger {
	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}
	return NewLogger(env)
}

// FromContext extracts the logger from context.
// Returns the global logger if none is found.
func FromContext(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok {
		return l
	}
	return zap.L()
}

// WithLogger adds a logger to the context.
func WithLogger(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// WithFields returns a context with additional fields added to the logger.
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	return WithLogger(ctx, FromContext(ctx).With(fields...))
}

// Debug logs a debug message using the logger from context.
func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).Debug(msg, fields...)
}

// Info logs an info message using the logger from context.
func Info(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).Info(msg, fields...)
}

// Warn logs a warning message using the logger from context.
func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).Warn(msg, fields...)
}

// Error logs an error message using the logger from context.
func Error(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).Error(msg, fields...)
}
