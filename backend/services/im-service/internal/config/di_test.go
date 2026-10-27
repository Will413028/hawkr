package config

import (
	"context"
	"net"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	metricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func TestInitOTel_RegistersMeterProviderAndFlushesOnShutdown(t *testing.T) {
	previousMeterProvider := otel.GetMeterProvider()
	previousTracerProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetMeterProvider(previousMeterProvider)
		otel.SetTracerProvider(previousTracerProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	collector := newMetricsCollector(t)
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "http://"+collector.address)

	lifecycle := fxtest.NewLifecycle(t)
	provider := initOTel(lifecycle, zap.NewNop())

	globalProvider, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider)
	if !ok || globalProvider != provider {
		t.Fatalf("global meter provider = %T, want *metric.MeterProvider", otel.GetMeterProvider())
	}

	counter, err := provider.Meter("github.com/Will413028/hawkr/im-service/internal/config/test").Int64Counter("im_service_shutdown_test_total")
	if err != nil {
		t.Fatalf("create shutdown test counter: %v", err)
	}
	counter.Add(context.Background(), 1)

	lifecycle.RequireStart()
	lifecycle.RequireStop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	select {
	case request := <-collector.requests:
		if !containsMetric(request, "im_service_shutdown_test_total") {
			t.Fatalf("shutdown export does not contain im_service_shutdown_test_total: %v", request)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for telemetry export during lifecycle shutdown")
	}
}

func TestModule_ProvidesMeterProviderThroughFX(t *testing.T) {
	previousMeterProvider := otel.GetMeterProvider()
	previousTracerProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetMeterProvider(previousMeterProvider)
		otel.SetTracerProvider(previousTracerProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
	t.Setenv("MEDIA_BASE_URL", "https://media.example.test")

	var provider *sdkmetric.MeterProvider
	app := fxtest.New(
		t,
		Module,
		fx.Populate(&provider),
	)
	if provider == nil {
		t.Fatal("IM FX module did not provide a meter provider")
	}
	app.RequireStart()
	app.RequireStop()
}

type metricsCollector struct {
	metricpb.UnimplementedMetricsServiceServer
	address  string
	requests chan *metricpb.ExportMetricsServiceRequest
}

func newMetricsCollector(t *testing.T) *metricsCollector {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for fake Collector: %v", err)
	}

	collector := &metricsCollector{
		address:  listener.Addr().String(),
		requests: make(chan *metricpb.ExportMetricsServiceRequest, 1),
	}
	server := grpc.NewServer()
	metricpb.RegisterMetricsServiceServer(server, collector)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return collector
}

func (c *metricsCollector) Export(_ context.Context, request *metricpb.ExportMetricsServiceRequest) (*metricpb.ExportMetricsServiceResponse, error) {
	c.requests <- request
	return &metricpb.ExportMetricsServiceResponse{}, nil
}

func containsMetric(request *metricpb.ExportMetricsServiceRequest, want string) bool {
	for _, resourceMetrics := range request.GetResourceMetrics() {
		for _, scopeMetrics := range resourceMetrics.GetScopeMetrics() {
			for _, metric := range scopeMetrics.GetMetrics() {
				if metric.GetName() == want {
					return true
				}
			}
		}
	}
	return false
}
