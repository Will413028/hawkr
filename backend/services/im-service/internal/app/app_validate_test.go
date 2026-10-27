package app

import (
	"testing"

	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/im-service/internal/client"
	"github.com/Will413028/hawkr/im-service/internal/config"
	imgrpc "github.com/Will413028/hawkr/im-service/internal/grpc"
	"github.com/Will413028/hawkr/im-service/internal/infrastructure"
	immq "github.com/Will413028/hawkr/im-service/internal/mq"
	"github.com/Will413028/hawkr/im-service/internal/repository"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
	"github.com/Will413028/hawkr/im-service/internal/worker"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// TestFxGraph_Valid makes the actual IM gRPC registration/bootstrap graph
// observable to fx.ValidateApp. Without an explicit *grpc.Server consumer, a
// broken server provider can remain lazy and compile-only checks miss it.
func TestFxGraph_Valid(t *testing.T) {
	err := fx.ValidateApp(
		config.Module,
		infrastructure.Module,
		client.Module,
		repository.Module,
		usecase.Module,
		immq.Module,
		worker.Module,
		imgrpc.Module,
		fx.Invoke(func(*grpc.Server) {}),
		fx.Invoke(func(*pkgserver.HealthController) {}),
		fx.Invoke(func(*sdkmetric.MeterProvider) {}),
	)
	if err != nil {
		t.Fatalf("fx dependency graph invalid: %v", err)
	}
}
