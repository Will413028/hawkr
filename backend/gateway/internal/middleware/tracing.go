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

		// Extract context from incoming request headers
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		// Trust boundary（#2451）：composite propagator 會把 browser 自帶的
		// W3C `baggage` header extract 進 ctx，而 outbound otelgrpc client
		// stats handler 用同一 propagator Inject——不清除就會原樣轉發進所有
		// 內部 gRPC metadata。trace 續鏈只需要 SpanContext；外部 baggage 一律
		// 在邊界丟棄，內部要用 baggage 必須由 gateway 在這行之後自己寫入。
		ctx = baggage.ContextWithoutBaggage(ctx)
		// 兩層互補：ctx 層擋 otelgrpc 的 gRPC 面，header 層擋 CreateProxy 系
		// 列 HTTP reverse proxy（media/seller/payuni-webhook）的原樣轉發面。
		// 只清 ctx 擋不住 header 透傳，只刪 header 也擋不住已 extract 的值。
		c.Request.Header.Del("baggage")

		// Start a new span — span name 與 attrs 只承載低基數 route template
		// （#2451 privacy）：raw path 含 dynamic ID、raw query 可能含個資，
		// 兩者都留在 access log 層，不進 span。unmatched route（404 probe）
		// 用固定值，避免掃描器輸入變成高基數 span name。
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

		// Store trace ID in context for logging
		traceID := span.SpanContext().TraceID().String()
		c.Set("trace_id", traceID)
		c.Header("x-trace-id", traceID)

		// 生成或提取 x-request-id（API 設計規範 §6.2）
		requestID := c.GetHeader("x-request-id")
		if requestID == "" {
			requestID = grpcClient.GenerateRequestID()
		}
		c.Set("request_id", requestID)
		c.Header("x-request-id", requestID)

		// Store trace context in Go request context for gRPC interceptor
		ctx = grpcClient.WithTraceContext(ctx, traceID, requestID)
		c.Request = c.Request.WithContext(ctx)

		// Process request
		c.Next()

		// Record response attributes
		span.SetAttributes(
			semconv.HTTPResponseStatusCodeKey.Int(c.Writer.Status()),
			attribute.Int("http.response_size", c.Writer.Size()),
		)

		// Record error if any
		if len(c.Errors) > 0 {
			span.SetAttributes(attribute.String("error", c.Errors.String()))
		}
	}
}
