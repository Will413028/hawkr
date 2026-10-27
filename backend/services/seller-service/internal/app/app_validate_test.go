package app

import (
	"testing"

	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/seller-service/internal/app/cron"
	"github.com/Will413028/hawkr/seller-service/internal/client"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	sellergrpc "github.com/Will413028/hawkr/seller-service/internal/grpc"
	"github.com/Will413028/hawkr/seller-service/internal/infrastructure"
	"github.com/Will413028/hawkr/seller-service/internal/mq"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"github.com/Will413028/hawkr/seller-service/internal/worker"
	"go.uber.org/fx"
)

// TestFxGraph_Valid makes the complete seller-service graph observable to
// fx.ValidateApp, including the final health controller, telemetry, and
// shared gRPC bootstrap consumers. A missing final-mode dependency must fail
// here instead of surfacing only after deployment.
func TestFxGraph_Valid(t *testing.T) {
	err := fx.ValidateApp(
		config.Module,
		infrastructure.Module,
		postgres.Module,
		client.Module,
		usecase.Module,
		worker.Module,
		mq.Module,
		sellergrpc.Module,
		cron.Module,
		fx.Invoke(func(*pkgserver.HealthController) {}),
		fx.Invoke(func(pkgserver.GRPCServerParams) {}),
	)
	if err != nil {
		t.Fatalf("fx dependency graph invalid: %v", err)
	}
}
