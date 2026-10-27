package infrastructure

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/crm-service/internal/config"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func TestNewGRPCServerConfig_ExcludesLegacyHTTPPort(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("GRPC_PORT", "10030")
	t.Setenv("HTTP_PORT", "10031")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	got := newGRPCServerConfig(cfg)
	if got.GRPCPort != "10030" {
		t.Errorf("GRPCPort = %q, want %q", got.GRPCPort, "10030")
	}
	if got.HTTPPort != "" {
		t.Errorf("HTTPPort = %q, want empty in final gRPC-only runtime", got.HTTPPort)
	}
	if got.Env != "development" {
		t.Errorf("Env = %q, want %q", got.Env, "development")
	}
	if !got.EnableStreamInterceptors {
		t.Error("EnableStreamInterceptors = false, want true for CRM")
	}
}

func TestModule_ProvidesMeterProvider(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	var provider *sdkmetric.MeterProvider
	app := fx.New(
		fx.Supply(&config.Config{Env: "development"}),
		fx.Supply(zap.NewNop()),
		Module,
		fx.Populate(&provider),
	)
	if err := app.Err(); err != nil {
		t.Fatalf("fx graph error: %v", err)
	}
	if provider == nil {
		t.Fatal("CRM infrastructure did not provide a meter provider")
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown meter provider: %v", err)
	}
}
