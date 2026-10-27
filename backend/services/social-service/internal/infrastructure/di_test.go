package infrastructure

import (
	"context"
	"testing"

	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/social-service/internal/config"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func TestModule_ProvidesFinalGRPCOnlyServerConfig(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("GRPC_PORT", "9918")
	t.Setenv("HTTP_PORT", "9917")
	t.Setenv("MEDIA_BASE_URL", "https://media.example.test")
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load social config: %v", err)
	}

	var cfg pkgserver.ServerConfig
	app := fx.New(
		fx.Supply(loaded),
		fx.Supply(zap.NewNop()),
		Module,
		fx.Populate(&cfg),
	)
	if err := app.Err(); err != nil {
		t.Fatalf("fx graph error: %v", err)
	}
	if cfg.GRPCPort != "9918" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "9918")
	}
	if cfg.HTTPPort != "" {
		t.Errorf("HTTPPort = %q, want empty in final gRPC-only runtime", cfg.HTTPPort)
	}
	if cfg.Env != "development" {
		t.Errorf("Env = %q, want %q", cfg.Env, "development")
	}
	if !cfg.EnableStreamInterceptors {
		t.Error("EnableStreamInterceptors = false, want true for social-service")
	}
}

func TestModule_ProvidesMeterProvider(t *testing.T) {
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
		t.Fatal("social-service infrastructure did not provide a meter provider")
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown meter provider: %v", err)
	}
}
