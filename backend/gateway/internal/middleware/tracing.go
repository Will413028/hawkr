package middleware

import (
	"context"

	grpcClient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pkgotel "github.com/Will413028/hawkr/go-pkg/otel"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer trace.Tracer

// InitTracer initializes the gateway tracer through the shared OTLP gRPC setup.
func InitTracer(logger *zap.Logger) func(context.Context) error {
	shutdown := pkgotel.InitTracer("gateway", logger)
	tracer = otel.Tracer("gateway")
	return shutdown
}

// TracingMiddleware creates a tracing middleware for Gin
func TracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if tracer == nil {
			c.Next()
			return
		}

		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		ctx = baggage.ContextWithoutBaggage(ctx)
		c.Request.Header.Del("baggage")

		spanName := c.FullPath()
		if spanName == "" {
			spanName = "unmatched"
		}

		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(c.Request.Method),
				semconv.HTTPRouteKey.String(spanName),
				attribute.String("http.client_ip", c.ClientIP()),
			),
		)
		defer span.End()

		traceID := span.SpanContext().TraceID().String()
		c.Set("trace_id", traceID)
		c.Header("x-trace-id", traceID)

		requestID := c.GetHeader("x-request-id")
		if requestID == "" {
			requestID = grpcClient.GenerateRequestID()
		}
		c.Set("request_id", requestID)
		c.Header("x-request-id", requestID)

		ctx = grpcClient.WithTraceContext(ctx, traceID, requestID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		span.SetAttributes(
			semconv.HTTPResponseStatusCodeKey.Int(c.Writer.Status()),
			attribute.Int("http.response_size", c.Writer.Size()),
		)

		if len(c.Errors) > 0 {
			span.SetAttributes(attribute.String("error", c.Errors.String()))
		}
	}
}
