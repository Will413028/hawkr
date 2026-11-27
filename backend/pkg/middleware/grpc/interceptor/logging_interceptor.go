// Package interceptor provides gRPC server/client interceptors for logging,
// correlation ID propagation, metrics, and error handling.
package interceptor

import (
	"context"
	"path"
	"time"

	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func enrichContext(ctx context.Context, logger *zap.Logger) (context.Context, *zap.Logger) {
	var fields []zap.Field

	if corrID := CorrelationIDFromContext(ctx); corrID != "" {
		fields = append(fields, zap.String("correlation_id", corrID))
	}

	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.HasTraceID() {
		fields = append(fields, zap.String("trace_id", spanCtx.TraceID().String()))
	}
	if spanCtx.HasSpanID() {
		fields = append(fields, zap.String("span_id", spanCtx.SpanID().String()))
	}

	enriched := logger
	if len(fields) > 0 {
		enriched = logger.With(fields...)
	}
	return pkglogger.WithLogger(ctx, enriched), enriched
}

// LoggingOption configures LoggingInterceptor. Options are additive; the zero
// configuration reproduces the historical behaviour (every non-OK status logs
// at error level).
type LoggingOption func(*loggingConfig)

type loggingConfig struct {
	expectedCodes map[string]map[codes.Code]struct{}
}

// WithExpectedCodes marks specific (method, status code) pairs as expected
// business outcomes rather than failures: they log at info level under a
// ".expected" event instead of error level under ".error". The returned error
// is untouched — this changes observability only, never the wire contract.
//
// Scoping is deliberately per-method and opt-in. Some RPCs answer NotFound as
// their normal negative answer (a web session lookup for an expired or revoked
// cookie — a permanent share of real traffic),
// while for others the very same code means something genuinely went wrong.
// A blanket "NotFound is never an error" rule would silence the second group
// too, so there is intentionally no way to express one here: an unlisted method
// keeps logging at error level, and each entry names the exact codes it excuses.
//
// The keys are FULL method names ("/pkg.Service/Method") because that is what
// grpc.UnaryServerInfo carries; a bare method name never matches.
func WithExpectedCodes(m map[string][]codes.Code) LoggingOption {
	return func(c *loggingConfig) {
		if c.expectedCodes == nil {
			c.expectedCodes = make(map[string]map[codes.Code]struct{}, len(m))
		}
		for method, list := range m {
			set, ok := c.expectedCodes[method]
			if !ok {
				set = make(map[codes.Code]struct{}, len(list))
				c.expectedCodes[method] = set
			}
			for _, code := range list {
				set[code] = struct{}{}
			}
		}
	}
}

func (c *loggingConfig) isExpected(fullMethod string, code codes.Code) bool {
	set, ok := c.expectedCodes[fullMethod]
	if !ok {
		return false
	}
	_, ok = set[code]
	return ok
}

// LoggingInterceptor returns a gRPC unary server interceptor that logs requests
// and injects a context-aware logger with trace_id.
func LoggingInterceptor(logger *zap.Logger, opts ...LoggingOption) grpc.UnaryServerInterceptor {
	cfg := &loggingConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		ctx, enriched := enrichContext(ctx, logger)

		fullService := path.Dir(info.FullMethod)[1:]
		method := path.Base(info.FullMethod)
		service := fullService
		if idx := len(fullService) - 1; idx >= 0 {
			for i := idx; i >= 0; i-- {
				if fullService[i] == '.' {
					service = fullService[i+1:]
					break
				}
			}
		}

		enriched.Info("gRPC request started",
			zap.String("event", service+"."+method+".start"),
			zap.String("method", info.FullMethod),
		)

		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)

		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			} else {
				code = codes.Unknown
			}
		}

		expected := err != nil && cfg.isExpected(info.FullMethod, code)

		var event string
		switch {
		case err == nil:
			event = service + "." + method + ".end"
		case expected:
			event = service + "." + method + ".expected"
		default:
			event = service + "." + method + ".error"
		}

		fields := []zap.Field{
			zap.String("event", event),
			zap.String("method", info.FullMethod),
			zap.Duration("duration", duration),
			zap.String("code", code.String()),
		}

		switch {
		case err == nil:
			enriched.Info("gRPC request completed", fields...)
		case expected:
			fields = append(fields, zap.Error(err))
			enriched.Info("gRPC request completed with expected status", fields...)
		default:
			fields = append(fields, zap.Error(err))
			enriched.Error("gRPC request failed", fields...)
		}

		return resp, err
	}
}

// StreamLoggingInterceptor returns a gRPC stream server interceptor that logs streams.
func StreamLoggingInterceptor(logger *zap.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		err := handler(srv, ss)

		duration := time.Since(start)

		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			} else {
				code = codes.Unknown
			}
		}

		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.Duration("duration", duration),
			zap.String("code", code.String()),
			zap.Bool("client_stream", info.IsClientStream),
			zap.Bool("server_stream", info.IsServerStream),
		}

		if err != nil {
			fields = append(fields, zap.Error(err))
			logger.Error("gRPC stream failed", fields...)
		} else {
			logger.Info("gRPC stream completed", fields...)
		}

		return err
	}
}
