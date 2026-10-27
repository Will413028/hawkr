package server

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/config"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestHealthController_FinalReadinessIsSeparateFromBind(t *testing.T) {
	const serviceName = "hawkr.notification.NotificationService"
	controller := NewHealthController(serviceName)

	assertHealthStatus(t, controller.server, "", healthpb.HealthCheckResponse_NOT_SERVING)
	assertHealthStatus(t, controller.server, serviceName, healthpb.HealthCheckResponse_NOT_SERVING)

	controller.MarkBound()
	assertHealthStatus(t, controller.server, "", healthpb.HealthCheckResponse_SERVING)
	assertHealthStatus(t, controller.server, serviceName, healthpb.HealthCheckResponse_NOT_SERVING)

	controller.MarkReady()
	assertHealthStatus(t, controller.server, serviceName, healthpb.HealthCheckResponse_SERVING)

	controller.MarkNotReady()
	assertHealthStatus(t, controller.server, "", healthpb.HealthCheckResponse_NOT_SERVING)
	assertHealthStatus(t, controller.server, serviceName, healthpb.HealthCheckResponse_NOT_SERVING)

	_, err := controller.server.Check(context.Background(), &healthpb.HealthCheckRequest{
		Service: "hawkr.unknown.Service",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unknown health service code = %s, want %s", status.Code(err), codes.NotFound)
	}
}

func TestNewGRPCServer_FinalHealthTransitionsThroughLifecycle(t *testing.T) {
	const serviceName = "hawkr.notification.NotificationService"
	port := freePort(t)
	lifecycle := fxtest.NewLifecycle(t)
	healthController := NewHealthController(serviceName)

	NewGRPCServer(GRPCServerParams{
		Lifecycle: lifecycle,
		Config: ServerConfig{
			ServerConfig: config.ServerConfig{GRPCPort: port},
		},
		Health: healthController,
		Logger: zap.NewNop(),
	})

	lifecycle.RequireStart()
	assertHealthStatus(t, healthController.server, "", healthpb.HealthCheckResponse_SERVING)
	assertHealthStatus(t, healthController.server, serviceName, healthpb.HealthCheckResponse_NOT_SERVING)

	healthController.MarkReady()
	assertHealthStatus(t, healthController.server, serviceName, healthpb.HealthCheckResponse_SERVING)

	lifecycle.RequireStop()
	assertHealthStatus(t, healthController.server, "", healthpb.HealthCheckResponse_NOT_SERVING)
	assertHealthStatus(t, healthController.server, serviceName, healthpb.HealthCheckResponse_NOT_SERVING)
}

func TestNewGRPCServer_UnexpectedServeErrorRequestsFXShutdown(t *testing.T) {
	wantErr := errors.New("serve failed")
	serveStarted := make(chan struct{})
	shutdownCalled := make(chan struct{})
	shutdowner := &recordingShutdowner{called: shutdownCalled}
	lifecycle := fxtest.NewLifecycle(t)

	NewGRPCServer(GRPCServerParams{
		Lifecycle:  lifecycle,
		Config:     ServerConfig{ServerConfig: config.ServerConfig{GRPCPort: freePort(t)}},
		Logger:     zap.NewNop(),
		Shutdowner: shutdowner,
		Serve: func(listener net.Listener) error {
			close(serveStarted)
			_ = listener.Close()
			return wantErr
		},
	})

	lifecycle.RequireStart()
	select {
	case <-serveStarted:
	case <-time.After(time.Second):
		t.Fatal("gRPC Serve was not called")
	}
	select {
	case <-shutdownCalled:
	case <-time.After(time.Second):
		t.Fatal("unexpected gRPC Serve error did not request FX shutdown")
	}
	lifecycle.RequireStop()
}

func TestNewGRPCServer_NormalServeStopDoesNotRequestFXShutdown(t *testing.T) {
	serveStarted := make(chan struct{})
	shutdownCalled := make(chan struct{})
	shutdowner := &recordingShutdowner{called: shutdownCalled}
	lifecycle := fxtest.NewLifecycle(t)

	NewGRPCServer(GRPCServerParams{
		Lifecycle:  lifecycle,
		Config:     ServerConfig{ServerConfig: config.ServerConfig{GRPCPort: freePort(t)}},
		Logger:     zap.NewNop(),
		Shutdowner: shutdowner,
		Serve: func(listener net.Listener) error {
			close(serveStarted)
			_ = listener.Close()
			return grpc.ErrServerStopped
		},
	})

	lifecycle.RequireStart()
	select {
	case <-serveStarted:
	case <-time.After(time.Second):
		t.Fatal("gRPC Serve was not called")
	}
	select {
	case <-shutdownCalled:
		t.Fatal("grpc.ErrServerStopped unexpectedly requested FX shutdown")
	case <-time.After(50 * time.Millisecond):
	}
	lifecycle.RequireStop()
}

func TestNewGRPCServer_UsesExplicitMeterProvider(t *testing.T) {
	previous := otel.GetMeterProvider()
	globalProvider := sdkmetric.NewMeterProvider()
	otel.SetMeterProvider(globalProvider)
	t.Cleanup(func() {
		_ = globalProvider.Shutdown(context.Background())
		otel.SetMeterProvider(previous)
	})

	reader := sdkmetric.NewManualReader()
	explicitProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = explicitProvider.Shutdown(context.Background()) })
	lifecycle := fxtest.NewLifecycle(t)

	NewGRPCServer(GRPCServerParams{
		Lifecycle:     lifecycle,
		Config:        ServerConfig{ServerConfig: config.ServerConfig{GRPCPort: freePort(t)}},
		Logger:        zap.NewNop(),
		MeterProvider: explicitProvider,
	})
	lifecycle.RequireStart()
	lifecycle.RequireStop()

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &metrics); err != nil {
		t.Fatalf("collect explicit-provider metrics: %v", err)
	}
	assertLifecycleMetricProtocol(t, metrics, "server_start_total", "grpc")
}

type recordingShutdowner struct {
	called chan struct{}
}

func (s *recordingShutdowner) Shutdown(...fx.ShutdownOption) error {
	select {
	case <-s.called:
	default:
		close(s.called)
	}
	return nil
}

func assertHealthStatus(t *testing.T, server *health.Server, service string, want healthpb.HealthCheckResponse_ServingStatus) {
	t.Helper()
	resp, err := server.Check(context.Background(), &healthpb.HealthCheckRequest{Service: service})
	if err != nil {
		t.Fatalf("health Check(%q): %v", service, err)
	}
	if got := resp.GetStatus(); got != want {
		t.Fatalf("health status for %q = %s, want %s", service, got, want)
	}
}

func assertLifecycleMetricProtocol(t *testing.T, metrics metricdata.ResourceMetrics, name, protocol string) {
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
					return
				}
			}
		}
	}
	t.Fatalf("metric %q has no datapoint for protocol %q", name, protocol)
}
