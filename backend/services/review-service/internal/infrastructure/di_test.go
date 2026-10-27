package infrastructure

import (
	"testing"

	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/review-service/internal/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type serverBootstrapInputs struct {
	fx.In

	Config pkgserver.ServerConfig
}

// TestModule_ProvidesFinalGRPCServerBootstrap pins review-service's wiring to
// one shared gRPC listener. HTTP is intentionally absent from the service-local
// config and from the shared server config passed to pkg/server.
func TestModule_ProvidesFinalGRPCServerBootstrap(t *testing.T) {
	var got serverBootstrapInputs

	app := fx.New(
		fx.Supply(&config.Config{
			Env:    "development",
			Server: config.ServerConfig{GRPCPort: "10033"},
		}),
		fx.Supply(zap.NewNop()),
		Module,
		fx.Invoke(func(in serverBootstrapInputs) {
			got = in
		}),
	)
	if err := app.Err(); err != nil {
		t.Fatalf("shared server bootstrap graph invalid: %v", err)
	}
	if got.Config.GRPCPort != "10033" {
		t.Fatalf("gRPC port = %q, want %q", got.Config.GRPCPort, "10033")
	}
	if got.Config.HTTPPort != "" {
		t.Fatalf("HTTP port = %q, want empty in final gRPC-only runtime", got.Config.HTTPPort)
	}
	if len(got.Config.HealthServiceNames) != 0 {
		t.Fatalf("legacy health service names = %v, want empty; review owns final health controller", got.Config.HealthServiceNames)
	}
	if !got.Config.EnableStreamInterceptors {
		t.Fatal("EnableStreamInterceptors = false, want true for review-service")
	}
}
