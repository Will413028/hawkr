package config

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
)

func TestInitOTel_RegistersMeterProviderAndShutdownHook(t *testing.T) {
	previousMeterProvider := otel.GetMeterProvider()
	previousTracerProvider := otel.GetTracerProvider()
	t.Cleanup(func() {
		otel.SetMeterProvider(previousMeterProvider)
		otel.SetTracerProvider(previousTracerProvider)
	})
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	lifecycle := fxtest.NewLifecycle(t)
	initOTel(lifecycle, zap.NewNop())

	provider, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider)
	if !ok || provider == nil {
		t.Fatalf("global meter provider = %T, want *metric.MeterProvider", otel.GetMeterProvider())
	}

	if err := lifecycle.Stop(context.Background()); err != nil {
		t.Fatalf("telemetry lifecycle stop: %v", err)
	}
}

func TestModule_ProvidesMeterProviderThroughFX(t *testing.T) {
	previousMeterProvider := otel.GetMeterProvider()
	previousTracerProvider := otel.GetTracerProvider()
	t.Cleanup(func() {
		otel.SetMeterProvider(previousMeterProvider)
		otel.SetTracerProvider(previousTracerProvider)
	})
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
	t.Setenv("DB_PASSWORD", "test-password")
	t.Setenv("MEDIA_BASE_URL", "https://media.example.test")

	var provider *sdkmetric.MeterProvider
	app := fxtest.New(
		t,
		Module,
		fx.Populate(&provider),
	)
	if provider == nil {
		t.Fatal("review-service FX module did not provide a meter provider")
	}
	app.RequireStart()
	app.RequireStop()
}
