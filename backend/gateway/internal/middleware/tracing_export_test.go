package middleware

import (
	"context"
	"net"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type traceExportCollector struct {
	collectortrace.UnimplementedTraceServiceServer

	mu       sync.Mutex
	requests []*collectortrace.ExportTraceServiceRequest
}

func (c *traceExportCollector) Export(_ context.Context, req *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, req)
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

func (c *traceExportCollector) snapshot() []*collectortrace.ExportTraceServiceRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*collectortrace.ExportTraceServiceRequest(nil), c.requests...)
}

func TestInitTracer_ExportsGatewaySpanOverOTLPGRPC(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for OTLP receiver: %v", err)
	}

	collector := &traceExportCollector{}
	server := grpc.NewServer()
	collectortrace.RegisterTraceServiceServer(server, collector)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	t.Setenv("ENV", "test")
	t.Setenv("TRACING_SAMPLING", "1")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", listener.Addr().String())

	shutdown := InitTracer(zap.NewNop())
	_, span := otel.Tracer("gateway").Start(context.Background(), "gateway-export-test")
	span.End()

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown tracer provider: %v", err)
	}

	requests := collector.snapshot()
	if len(requests) == 0 {
		t.Fatal("gateway span was not exported to the OTLP gRPC receiver")
	}

	var (
		gotServiceName bool
		gotEnvironment bool
		gotSpan        bool
	)
	for _, request := range requests {
		for _, resourceSpans := range request.GetResourceSpans() {
			for _, attr := range resourceSpans.GetResource().GetAttributes() {
				switch attr.GetKey() {
				case "service.name":
					gotServiceName = attr.GetValue().GetStringValue() == "gateway"
				case "deployment.environment":
					gotEnvironment = attr.GetValue().GetStringValue() == "test"
				}
			}
			for _, scopeSpans := range resourceSpans.GetScopeSpans() {
				for _, exportedSpan := range scopeSpans.GetSpans() {
					if exportedSpan.GetName() == "gateway-export-test" {
						gotSpan = true
					}
				}
			}
		}
	}

	if !gotServiceName {
		t.Fatal("exported resource is missing service.name=gateway")
	}
	if !gotEnvironment {
		t.Fatal("exported resource is missing deployment.environment=test")
	}
	if !gotSpan {
		t.Fatal("exported payload is missing gateway-export-test span")
	}
}
