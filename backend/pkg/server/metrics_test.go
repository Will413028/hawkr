package server

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/config"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
)

func TestServerLifecycle_RecordsOTelMetrics(t *testing.T) {
	previous := otel.GetMeterProvider()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetMeterProvider(previous)
	})

	lifecycle := fxtest.NewLifecycle(t)
	NewGRPCServer(GRPCServerParams{
		Lifecycle: lifecycle,
		Config:    ServerConfig{ServerConfig: config.ServerConfig{GRPCPort: freePort(t)}},
		Logger:    zap.NewNop(),
	})
	NewInternalHTTPServer(HTTPServerParams{
		Lifecycle:   lifecycle,
		Config:      ServerConfig{ServerConfig: config.ServerConfig{HTTPPort: freePort(t)}, Env: "production"},
		ServiceName: "test-service",
		Logger:      zap.NewNop(),
	})

	lifecycle.RequireStart()
	lifecycle.RequireStop()

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &metrics); err != nil {
		t.Fatalf("collect lifecycle metrics: %v", err)
	}
	assertLifecycleMetricProtocols(t, metrics, "server_start_total")
	assertLifecycleMetricProtocols(t, metrics, "server_shutdown_total")
}

func TestServerLifecycle_FailedGRPCBindDoesNotRecordSuccessfulStart(t *testing.T) {
	previous := otel.GetMeterProvider()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetMeterProvider(previous)
	})

	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("reserve occupied port: %v", err)
	}
	defer occupied.Close()
	port := strconv.Itoa(occupied.Addr().(*net.TCPAddr).Port)

	lifecycle := fxtest.NewLifecycle(t)
	NewGRPCServer(GRPCServerParams{
		Lifecycle: lifecycle,
		Config:    ServerConfig{ServerConfig: config.ServerConfig{GRPCPort: port}},
		Logger:    zap.NewNop(),
	})
	if err := lifecycle.Start(context.Background()); err == nil {
		t.Fatal("lifecycle Start returned nil for an occupied gRPC port")
	}

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &metrics); err != nil {
		t.Fatalf("collect failed-bind metrics: %v", err)
	}
	assertNoLifecycleMetricProtocol(t, metrics, "server_start_total", "grpc")
}

func assertLifecycleMetricProtocols(t *testing.T, metrics metricdata.ResourceMetrics, name string) {
	t.Helper()
	want := map[string]bool{"grpc": false, "http": false}
	for _, scope := range metrics.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != name {
				continue
			}
			sum, ok := metric.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric %q data type = %T, want metricdata.Sum[int64]", name, metric.Data)
			}
			for _, point := range sum.DataPoints {
				protocol, ok := point.Attributes.Value("protocol")
				if !ok {
					t.Fatalf("metric %q datapoint missing protocol attribute", name)
				}
				if protocol.Type().String() != "STRING" {
					t.Fatalf("metric %q protocol attribute type = %s, want STRING", name, protocol.Type())
				}
				if _, exists := want[protocol.AsString()]; exists {
					want[protocol.AsString()] = true
				}
			}
		}
	}
	for protocol, seen := range want {
		if !seen {
			t.Errorf("metric %q has no datapoint for protocol %q", name, protocol)
		}
	}
}

func assertNoLifecycleMetricProtocol(t *testing.T, metrics metricdata.ResourceMetrics, name, protocol string) {
	t.Helper()
	for _, scope := range metrics.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != name {
				continue
			}
			sum, ok := metric.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric %q data type = %T, want metricdata.Sum[int64]", name, metric.Data)
			}
			for _, point := range sum.DataPoints {
				value, ok := point.Attributes.Value("protocol")
				if ok && value.AsString() == protocol {
					t.Fatalf("metric %q recorded a successful %s start after bind failure", name, protocol)
				}
			}
		}
	}
}
