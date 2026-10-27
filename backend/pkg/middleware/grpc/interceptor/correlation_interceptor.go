package interceptor

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type correlationIDKey struct{}

const metadataKeyRequestID = "x-request-id"

// CorrelationIDFromContext extracts the correlation ID from context.
// Returns empty string if not found.
func CorrelationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey{}).(string); ok {
		return id
	}
	return ""
}

// RequestIDFromContext is an alias for CorrelationIDFromContext for backwards compatibility.
func RequestIDFromContext(ctx context.Context) string {
	return CorrelationIDFromContext(ctx)
}

// WithCorrelationID stores a correlation ID in the context.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey{}, id)
}

// WithRequestID is an alias for WithCorrelationID for backwards compatibility.
func WithRequestID(ctx context.Context, id string) context.Context {
	return WithCorrelationID(ctx, id)
}

// CorrelationInterceptor returns a gRPC unary server interceptor that
// extracts or generates a correlation_id and stores it in the context.
//
// Resolution order:
//  1. x-request-id from incoming gRPC metadata (set by gateway)
//  2. OTel trace_id (if available)
//  3. Generated UUID
func CorrelationInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		ctx = WithCorrelationID(ctx, resolveCorrelationID(ctx))
		return handler(ctx, req)
	}
}

// StreamCorrelationInterceptor returns a gRPC stream server interceptor
// that extracts or generates a correlation_id and stores it in the context.
func StreamCorrelationInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()
		ctx = WithCorrelationID(ctx, resolveCorrelationID(ctx))
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
	}
}

// resolveCorrelationID extracts or generates a correlation ID.
func resolveCorrelationID(ctx context.Context) string {
	// 1. Check incoming gRPC metadata
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ids := md.Get(metadataKeyRequestID); len(ids) > 0 && ids[0] != "" {
			return ids[0]
		}
	}

	// 2. Fall back to OTel trace ID
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.HasTraceID() {
		return spanCtx.TraceID().String()
	}

	// 3. Generate a new UUID
	return uuid.New().String()
}

// wrappedStream wraps a grpc.ServerStream to override the context.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}
