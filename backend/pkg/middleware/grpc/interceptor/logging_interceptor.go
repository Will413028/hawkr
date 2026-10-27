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

// enrichContext injects a logger with trace_id, span_id, correlation_id into the context.
func enrichContext(ctx context.Context, logger *zap.Logger) (context.Context, *zap.Logger) {
	var fields []zap.Field

	// Add correlation_id from correlation interceptor
	if corrID := CorrelationIDFromContext(ctx); corrID != "" {
		fields = append(fields, zap.String("correlation_id", corrID))
	}

	// Add OTel trace/span IDs
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
	// expectedCodes maps a gRPC FULL method name (e.g.
	// "/hawkr.account.AccountService/GetWebSession") to the status codes
	// that are routine business outcomes *of that one method*.
	expectedCodes map[string]map[codes.Code]struct{}
}

// WithExpectedCodes marks specific (method, status code) pairs as expected
// business outcomes rather than failures: they log at info level under a
// ".expected" event instead of error level under ".error". The returned error
// is untouched — this changes observability only, never the wire contract.
//
// Scoping is deliberately per-method and opt-in. Some RPCs answer NotFound as
// their normal negative answer (a web session lookup for an expired or revoked
// cookie — a permanent share of real traffic, see 17 號設計書 §「負向快取的必要性」),
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

// isExpected reports whether code is a registered routine outcome of fullMethod.
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
		// Enrich context with logger + trace_id
		ctx, enriched := enrichContext(ctx, logger)

		// Build service/method names for event
		// FullMethod format: /package.ServiceName/Method → extract ServiceName only
		fullService := path.Dir(info.FullMethod)[1:] // e.g. "hawkr.social.SocialService"
		method := path.Base(info.FullMethod)         // e.g. "GetPostDetail"
		// Extract short service name (last segment after last dot)
		service := fullService
		if idx := len(fullService) - 1; idx >= 0 {
			for i := idx; i >= 0; i-- {
				if fullService[i] == '.' {
					service = fullService[i+1:]
					break
				}
			}
		}

		// Log start event
		enriched.Info("gRPC request started",
			zap.String("event", service+"."+method+".start"),
			zap.String("method", info.FullMethod),
		)

		start := time.Now()

		// Call the handler with enriched context
		resp, err := handler(ctx, req)

		// Calculate duration
		duration := time.Since(start)

		// Extract gRPC status code
		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			} else {
				code = codes.Unknown
			}
		}

		// An error whose code this method declared routine (WithExpectedCodes) is
		// a business answer, not a failure: it gets its own event name and info
		// level so it neither inflates the ".error" event count nor dilutes
		// error-level alerting. The error itself is still returned to the caller.
		expected := err != nil && cfg.isExpected(info.FullMethod, code)

		// Build end/expected/error event name
		var event string
		switch {
		case err == nil:
			event = service + "." + method + ".end"
		case expected:
			event = service + "." + method + ".expected"
		default:
			event = service + "." + method + ".error"
		}

		// Log the request
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

		// Call the handler
		err := handler(srv, ss)

		// Calculate duration
		duration := time.Since(start)

		// Extract gRPC status code
		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			} else {
				code = codes.Unknown
			}
		}

		// Log the stream
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
