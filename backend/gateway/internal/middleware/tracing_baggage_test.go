package middleware

import (
	"context"
	"net"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	grpcClient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

// setupTracingTest 用 in-memory exporter 接管 package-level tracer 與 global
// propagator（InitTracer 在測試中不跑），t.Cleanup 還原——這兩個都是 process
// 全域，本檔測試不可 t.Parallel。
func setupTracingTest(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prevTracer := tracer
	tracer = tp.Tracer("tracing-test")
	prevProp := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() {
		tracer = prevTracer
		otel.SetTextMapPropagator(prevProp)
		_ = tp.Shutdown(context.Background())
	})
	return exporter
}

const testTraceParent = "00-0102030405060708090a0b0c0d0e0f10-0102030405060708-01"

// #2451 trust boundary：browser 自帶的 W3C `baggage` header 不得進入 request
// context（否則 outbound otelgrpc Inject 會原樣轉發進內部 gRPC metadata）；
// 同時 remote parent 的 SpanContext 必須保留，trace 續鏈不能被誤傷。
func TestTracingMiddleware_DropsBrowserBaggage_KeepsRemoteParent(t *testing.T) {
	setupTracingTest(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(TracingMiddleware())
	var gotBaggage baggage.Baggage
	var gotSpan trace.SpanContext
	r.GET("/api/v1/foo", func(c *gin.Context) {
		gotBaggage = baggage.FromContext(c.Request.Context())
		gotSpan = trace.SpanContextFromContext(c.Request.Context())
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/api/v1/foo", nil)
	req.Header.Set("traceparent", testTraceParent)
	req.Header.Set("baggage", "attack=injected,pii=email%40example.com")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if gotBaggage.Len() != 0 {
		t.Fatalf("browser baggage 洩入 request context: %v", gotBaggage.Members())
	}
	if got := gotSpan.TraceID().String(); got != "0102030405060708090a0b0c0d0e0f10" {
		t.Fatalf("remote parent 續鏈被破壞: trace ID = %s", got)
	}
}

// ctx 層 scrub 只擋 otelgrpc Inject 這條 gRPC 面；CreateProxy 系列 HTTP
// reverse proxy（media/seller/payuni-webhook）是把 c.Request 的 header 原樣
// 轉發到內部 upstream 的，所以 header 本身也要在同一個 trust boundary 清掉
// ——兩層互補，缺一面就有一條透傳路徑。
func TestTracingMiddleware_StripsBaggageHeaderForDownstreamProxies(t *testing.T) {
	setupTracingTest(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(TracingMiddleware())
	var gotHeader []string
	r.GET("/api/v1/foo", func(c *gin.Context) {
		gotHeader = c.Request.Header.Values("baggage")
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/api/v1/foo", nil)
	req.Header.Set("traceparent", testTraceParent)
	req.Header.Set("baggage", "attack=injected")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if len(gotHeader) != 0 {
		t.Fatalf("baggage header 未在 trust boundary 移除，會被 HTTP reverse proxy 原樣透傳: %v", gotHeader)
	}
}

// 清除 context 裡的 baggage 才算修好——刪 incoming header 擋不住已 extract 的
// 值。這裡驗實際 wire：完整 DefaultDialOpts（含 OTel client stats handler）
// 打進 bufconn server，斷言 incoming metadata 零 baggage、traceparent 續同一
// 條 trace。
func TestTracingMiddleware_OutboundGRPCWire_NoBaggage(t *testing.T) {
	setupTracingTest(t)

	lis := bufconn.Listen(1 << 20)
	var mu sync.Mutex
	var captured metadata.MD
	srv := grpc.NewServer(grpc.UnaryInterceptor(
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
			if md, ok := metadata.FromIncomingContext(ctx); ok {
				mu.Lock()
				captured = md.Copy()
				mu.Unlock()
			}
			return h(ctx, req)
		}))
	healthpb.RegisterHealthServer(srv, health.NewServer())
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	dialOpts := append([]grpc.DialOption{
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}, grpcClient.DefaultDialOpts()...)
	conn, err := grpc.NewClient("passthrough:///bufnet", dialOpts...)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(TracingMiddleware())
	var handlerCtx context.Context
	r.GET("/api/v1/foo", func(c *gin.Context) {
		handlerCtx = c.Request.Context()
		c.Status(200)
	})
	req := httptest.NewRequest("GET", "/api/v1/foo", nil)
	req.Header.Set("traceparent", testTraceParent)
	req.Header.Set("baggage", "attack=injected")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if _, err := healthpb.NewHealthClient(conn).Check(handlerCtx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("health check over bufconn: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if got := captured.Get("baggage"); len(got) != 0 {
		t.Fatalf("browser baggage 洩到 outbound gRPC wire: %v", got)
	}
	tp := captured.Get("traceparent")
	if len(tp) == 0 {
		t.Fatal("outbound wire 缺 traceparent——TraceContext 注入被誤傷")
	}
	if !strings.Contains(tp[0], "0102030405060708090a0b0c0d0e0f10") {
		t.Fatalf("outbound traceparent 不在同一條 trace 上: %s", tp[0])
	}
}
