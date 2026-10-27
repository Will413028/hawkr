package infrastructure

import (
	"context"
	"testing"

	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
)

// TestModule_ProvidesFinalGRPCOnlyServerConfig pins pricebook-service's final
// server boundary onto the shared pkg/server: only the gRPC port is
// propagated, and gRPC streaming interceptors remain disabled because every
// Pricebook RPC is unary.
func TestModule_ProvidesFinalGRPCOnlyServerConfig(t *testing.T) {
	var cfg pkgserver.ServerConfig

	app := fx.New(
		fx.Supply(&config.Config{
			Env: "development",
			Server: config.ServerConfig{
				GRPCPort: "10039",
			},
		}),
		fx.Supply(zap.NewNop()),
		Module,
		fx.Populate(&cfg),
	)
	if err := app.Err(); err != nil {
		t.Fatalf("fx graph error: %v", err)
	}

	if cfg.GRPCPort != "10039" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "10039")
	}
	if cfg.HTTPPort != "" {
		t.Errorf("HTTPPort = %q, want empty in final gRPC-only runtime", cfg.HTTPPort)
	}
	if cfg.EnableStreamInterceptors {
		t.Error("EnableStreamInterceptors = true, want false for pricebook-service (GetObservedPrice is unary-only)")
	}
}

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
