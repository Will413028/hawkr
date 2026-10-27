package infrastructure

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
)

func TestNewTelemetry_ProvidesMeterAndLifecycleShutdown(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	lifecycle := fxtest.NewLifecycle(t)
	provider := newTelemetry(lifecycle, zap.NewNop())
	if provider == nil {
		t.Fatal("newTelemetry returned nil meter provider")
	}
	if _, ok := any(provider).(*sdkmetric.MeterProvider); !ok {
		t.Fatalf("newTelemetry returned %T, want *sdkmetric.MeterProvider", provider)
	}

	if err := lifecycle.Start(context.Background()); err != nil {
		t.Fatalf("telemetry lifecycle start: %v", err)
	}
	if err := lifecycle.Stop(context.Background()); err != nil {
		t.Fatalf("telemetry lifecycle stop: %v", err)
	}
}
