package infrastructure

import (
	"context"
	"testing"

	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func TestModule_ProvidesFinalGRPCOnlyServerConfig(t *testing.T) {
	loaded := loadSellerConfigForTest(t, "10020", "10021")

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

	if cfg.GRPCPort != "10020" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "10020")
	}
	if cfg.HTTPPort != "" {
		t.Errorf("HTTPPort = %q, want empty in final gRPC-only runtime", cfg.HTTPPort)
	}
	if cfg.Env != "development" {
		t.Errorf("Env = %q, want %q", cfg.Env, "development")
	}
	if !cfg.EnableStreamInterceptors {
		t.Error("EnableStreamInterceptors = false, want true for seller-service")
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
		t.Fatal("seller-service infrastructure did not provide a meter provider")
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown meter provider: %v", err)
	}
}

func loadSellerConfigForTest(t *testing.T, grpcPort, httpPort string) *config.Config {
	t.Helper()
	t.Setenv("ENV", "development")
	t.Setenv("GRPC_PORT", grpcPort)
	t.Setenv("HTTP_PORT", httpPort)
	t.Setenv("PAYUNI_AGENT_HASH_KEY", "12345678901234567890123456789012")
	t.Setenv("PAYUNI_AGENT_HASH_IV", "1234567890123456")
	t.Setenv("ENCRYPTION_MASTER_KEY", "seller-test-master-key")
	t.Setenv("TGOS_BASE_URL", "https://tgos.example.test")
	t.Setenv("TGOS_QUERYADDR_APPID", "test-appid")
	t.Setenv("TGOS_QUERYADDR_APIKEY", "test-apikey")
	t.Setenv("TGOS_PROXY_CALLER_KEY_FILE", "/tmp/tgos-caller-key.json")
	t.Setenv("MEDIA_BASE_URL", "https://media.example.test")

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load seller config: %v", err)
	}
	return loaded
}
